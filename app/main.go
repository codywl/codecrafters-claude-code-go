package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

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

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(prompt),
	}

	done := 0
	for done < 1 {
		client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseUrl))
		resp, err := client.Chat.Completions.New(context.Background(),
			openai.ChatCompletionNewParams{
				Model:    "anthropic/claude-haiku-4.5",
				Messages: messages,
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
		messages = append(messages, toolMessage.ToParam())

		if len(toolMessage.ToolCalls) < 1 {
			if toolMessage.Content != "" {
				fmt.Print(toolMessage.Content)
			}
			return
		}

		for _, toolCall := range toolMessage.ToolCalls {

			type ReadArgs struct {
				FilePath string `json:"file_path"`
			}

			var args ReadArgs
			err = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error parsing json args")
				os.Exit(1)
			}

			content, err := os.ReadFile(args.FilePath)
			if err != nil {
				fmt.Errorf("Error reading file. %v\n", err)
				os.Exit(1)
			}

			fmt.Print(string(content))

			if toolCall.Function.Name == "Read" {
				fmt.Fprintln(os.Stderr, "Found read")
			}

			// You can use print statements as follows for debugging, they'll be visible when running tests.
			fmt.Fprintln(os.Stderr, "Logs from your program will appear here!")

			fmt.Print(resp.Choices[0].Message.Content)
		}
	}
}
