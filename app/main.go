package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
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

	for {
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
					openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
						Name:        "Write",
						Description: openai.String("Write content to a file"),
						Parameters: openai.FunctionParameters{
							"type": "object",
							"properties": map[string]any{
								"file_path": map[string]any{
									"type":        "string",
									"description": "The path to the file to write",
								},
								"content": map[string]any{
									"type":        "string",
									"description": "The content to write to the file",
								},
							},
							"required": []string{"file_path", "content"},
						},
					}),
					openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
						Name:        "Bash",
						Description: openai.String("Execute a shell command"),
						Parameters: openai.FunctionParameters{
							"type":     "object",
							"required": []string{"command"},
							"properties": map[string]any{
								"command": map[string]any{
									"type":        "string",
									"description": "The command to execute",
								},
							},
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
				Content  string `json:"content"`
				Command  string `json:"command"`
			}

			var args ReadArgs
			err = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error parsing json args")
				os.Exit(1)
			}

			var content []byte

			if toolCall.Function.Name == "Bash" {
				cmd := exec.Command("bash", "-c", args.Command)
				output, err := cmd.CombinedOutput()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Command failed: %v\n", err)
					os.Exit(1)
				}
				content = output
			}

			if toolCall.Function.Name == "Read" {
				content, err = os.ReadFile(args.FilePath)
				if err != nil {
					fmt.Printf("Error reading file. %v\n", err)
					os.Exit(1)
				}
			}

			if toolCall.Function.Name == "Write" {
				os.WriteFile(args.FilePath, []byte(args.Content), 0644)
				content, err = os.ReadFile(args.FilePath)
				if err != nil {
					fmt.Printf("Error reading file. %v\n", err)
					os.Exit(1)
				}
			}

			messages = append(messages, openai.ToolMessage(string(content), toolCall.ID))

		}

	}
}
