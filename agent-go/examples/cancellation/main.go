package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	"github.com/hoangvvo/llm-sdk/agent-go/examples"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

type waitTool struct{}

func (waitTool) Name() string { return "wait" }

func (waitTool) Description() string {
	return "Wait for a requested number of seconds"
}

func (waitTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"seconds": map[string]any{"type": "integer", "minimum": 1},
		},
		"required":             []string{"seconds"},
		"additionalProperties": false,
	}
}

func (waitTool) Execute(ctx context.Context, raw json.RawMessage, _ struct{}, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var args struct {
		Seconds int `json:"seconds"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	timer := time.NewTimer(time.Duration(args.Seconds) * time.Second)
	defer timer.Stop()

	// This timer is only for demonstration. Database, HTTP, and other clients
	// usually accept ctx directly.
	select {
	case <-ctx.Done():
		return llmagent.AgentToolResult{}, ctx.Err()
	case <-timer.C:
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{llmsdk.NewTextPart("Finished waiting")},
			IsError: false,
		}, nil
	}
}

func main() {
	provider := os.Getenv("PROVIDER")
	if provider == "" {
		provider = "openai"
	}
	modelID := os.Getenv("MODEL")
	if modelID == "" {
		modelID = "gpt-5.6-terra"
	}

	model, err := examples.GetModel(provider, modelID, llmsdk.LanguageModelMetadata{}, "")
	if err != nil {
		log.Fatal(err)
	}

	agent := llmagent.NewAgent(
		"CancellableAssistant",
		model,
		llmagent.WithTools(llmagent.NewAgentFunctionTool(waitTool{})),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// A Stop button or client disconnect would call cancel().
	cancellationTimer := time.AfterFunc(2*time.Second, cancel)
	defer cancellationTimer.Stop()

	response, err := agent.Run(ctx, llmagent.AgentRequest[struct{}]{
		Context: struct{}{},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart("Use the wait tool to wait for 30 seconds."),
				),
			),
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	if response.Status == llmagent.AgentResponseStatusCancelled {
		fmt.Println("Run cancelled safely.")
	} else {
		fmt.Println(response.Text())
	}
}
