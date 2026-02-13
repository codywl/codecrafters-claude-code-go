package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func main() {
	var prompt string
	flag.StringVar(&prompt, "p", "", "Prompt to send to LLM")
	flag.Parse()

	if prompt == "" {
		panic("Prompt must not be empty")
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	baseUrl := os.Getenv("OPENROUTER_BASE_URL")
	if baseUrl == "" {
		baseUrl = "https://openrouter.ai/api/v1"
	}

	if apiKey == "" {
		panic("Env variable OPENROUTER_API_KEY not found")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseUrl))
	resp, err := client.Chat.Completions.New(context.Background(),
		openai.ChatCompletionNewParams{
			Model: "anthropic/claude-haiku-4.5",
			Messages: []openai.ChatCompletionMessageParamUnion{
				{
					OfUser: &openai.ChatCompletionUserMessageParam{
						Content: openai.ChatCompletionUserMessageParamContentUnion{
							OfString: openai.String(prompt),
						},
					},
				},
			},
			Tools: []openai.ChatCompletionToolUnionParam{
				openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
					Name:        "Read",
					Description: openai.String("Read and return the contents of a file"),
					Parameters: openai.FunctionParameters{
						"type": "object",
						"properties": map[string]any{
							"file_path": map[string]any{
								"type":        "string",
								"description": "The path to the file to read",
							},
						},
						"required": []string{"file_path"},
					},
				}),
			},
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(resp.Choices) == 0 {
		panic("No choices in response")
	}

	toolMessage := resp.Choices[0].Message
	toolCall := toolMessage.ToolCalls[0]

	toolJson := toolCall.JSON
	dec := json.NewDecoder(strings.NewReader(toolJson.Function.Raw()))

	// open bracket
	t, err := dec.Token()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	fmt.Printf("%T: %v\n", t, t)

	type Message struct {
		Name, text string
	}

	// inner values
	for dec.More() {
		var m Message
		err := dec.Decode(&m)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		fmt.Printf("%v: %v\n", m.Name, m.text)
	}

	// close bracket
	t, err = dec.Token()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	fmt.Printf("%T: %v\n", t, t)

	if toolCall.Function.Name == "Read" {
		fmt.Fprintln(os.Stderr, "Found read")
	}

	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

	fmt.Print(resp.Choices[0].Message.Content)
}
