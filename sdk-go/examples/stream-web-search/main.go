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

	stream, err := model.Stream(
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
		log.Fatalf("Stream failed: %v", err)
	}

	accumulator := llmsdk.NewStreamAccumulator()

	for stream.Next() {
		current := stream.Current()
		litter.Dump(current)

		if err := accumulator.AddPartial(*current); err != nil {
			log.Printf("Failed to add partial: %v", err)
		}
	}

	if err := stream.Err(); err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	response, err := accumulator.ComputeResponse()
	if err != nil {
		log.Fatalf("Failed to compute response: %v", err)
	}

	litter.Dump(response.Content)
}
