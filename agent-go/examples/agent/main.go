package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
)

// Define the context interface that can be accessed in the instructions and
// tools
type MyContext struct {
	UserName string
}

type GetWeatherParams struct {
	City string `json:"city"`
}

// Define the agent tools
type GetWeatherTool struct{}

func (t *GetWeatherTool) Name() string {
	return "get_weather"
}

func (t *GetWeatherTool) Description() string {
	return "Get weather for a given city"
}

func (t *GetWeatherTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{
				"type":        "string",
				"description": "The city to get the weather for",
			},
		},
		"required":             []string{"city"},
		"additionalProperties": false,
	}
}

func (t *GetWeatherTool) Execute(ctx context.Context, paramsJSON json.RawMessage, contextVal MyContext, runState *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params GetWeatherParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	fmt.Printf("Getting weather for %s\n", params.City)

	result := map[string]any{
		"city":         params.City,
		"forecast":     "Sunny",
		"temperatureC": 25,
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return llmagent.AgentToolResult{}, err
	}

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(string(resultJSON)),
		},
		IsError: false,
	}, nil
}

type SendMessageParams struct {
	Message     string `json:"message"`
	PhoneNumber string `json:"phone_number"`
}

type SendMessageTool struct{}

func (t *SendMessageTool) Name() string {
	return "send_message"
}

func (t *SendMessageTool) Description() string {
	return "Send a text message to a phone number"
}

func (t *SendMessageTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "The message to send",
			},
			"phone_number": map[string]any{
				"type":        "string",
				"description": "The phone number to send the message to",
			},
		},
		"required":             []string{"message", "phone_number"},
		"additionalProperties": false,
	}
}

func (t *SendMessageTool) Execute(ctx context.Context, paramsJSON json.RawMessage, contextVal MyContext, runState *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params SendMessageParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	fmt.Printf("Sending message to %s: %s\n", params.PhoneNumber, params.Message)

	result := map[string]any{
		"message": params.Message,
		"status":  "sent",
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return llmagent.AgentToolResult{}, err
	}

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(string(resultJSON)),
		},
		IsError: false,
	}, nil
}

func main() {
	godotenv.Load("../.env")

	// Define the model to use for the Agent
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable must be set")
	}

	model := openai.NewOpenAIModel(openai.OpenAIModelOptions{
		APIKey:  apiKey,
		ModelID: "gpt-4o",
	})

	// Get user name
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Your name: ")
	userName, _ := reader.ReadString('\n')
	userName = strings.TrimSpace(userName)

	myContext := MyContext{UserName: userName}

	// Create instruction params
	staticInstruction := "You are Mai, a helpful assistant. Answer questions to the best of your ability."
	dynamicInstruction := func(ctx context.Context, ctxVal MyContext) (string, error) {
		return fmt.Sprintf("You are talking to %s", ctxVal.UserName), nil
	}

	// Create the Agent
	myAssistant := llmagent.NewAgent("Mai", model,
		llmagent.WithInstructions(
			llmagent.InstructionParam[MyContext]{String: &staticInstruction},
			llmagent.InstructionParam[MyContext]{Func: dynamicInstruction},
		),
		llmagent.WithTools(
			&GetWeatherTool{},
			&SendMessageTool{},
		),
	)

	// Implement the CLI to interact with the Agent
	var items []llmagent.AgentItem

	fmt.Println("Type 'exit' to quit")

	ctx := context.Background()

	for {
		fmt.Print("> ")
		userInput, _ := reader.ReadString('\n')
		userInput = strings.TrimSpace(userInput)

		if userInput == "" {
			continue
		}

		if strings.ToLower(userInput) == "exit" {
			break
		}

		// Add user message as the input
		items = append(items, llmagent.NewMessageAgentItem(llmsdk.NewUserMessage(
			llmsdk.NewTextPart(userInput),
		)))

		// Call assistant
		response, err := myAssistant.Run(ctx, llmagent.AgentRequest[MyContext]{
			Context: myContext,
			Input:   items,
		})
		if err != nil {
			log.Printf("Error: %v\n", err)
			continue
		}

		// Append items with the output items
		items = append(items, response.Output...)

		prettyJSON, err := json.MarshalIndent(response.Content, "", "  ")
		if err != nil {
			log.Fatalf("Failed to format JSON: %v", err)
		}
		fmt.Printf("%s\n", string(prettyJSON))
	}
}
