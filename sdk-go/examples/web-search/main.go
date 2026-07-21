package main

import (
	"context"
	"log"
	"os"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
	"github.com/sanity-io/litter"
)

func main() {
	provider := os.Getenv("PROVIDER")
	if provider == "" {
		provider = "openai"
	}
	modelID := os.Getenv("MODEL")
	if modelID == "" {
		modelID = "gpt-5.6-sol"
	}
	model := examples.GetModel(provider, modelID)

	response, err := model.Generate(
		context.Background(),
		llmsdk.NewLanguageModelInput(
			[]llmsdk.Message{
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart("Use web search to find the official IANA page about reserved domains. Reply with one sentence containing the word IANA and cite the source."),
				),
			},
			llmsdk.WithInputTools(llmsdk.NewWebSearchTool()),
		),
	)
	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	litter.Dump(response.Content)
	for _, part := range response.Content {
		if part.ToolCallPart != nil && part.ToolCallPart.Call.WebSearch != nil {
			log.Printf("web search: %#v", part.ToolCallPart.Call.WebSearch)
		}
		if part.ToolResultPart != nil && part.ToolResultPart.Result.WebSearch != nil {
			log.Printf("sources: %#v", part.ToolResultPart.Result.WebSearch.Sources)
		}
	}
}
