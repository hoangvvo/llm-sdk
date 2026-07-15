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

	response, err := model.Generate(context.Background(), &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart(`A car starts from rest and accelerates at a constant rate of 4 m/s^2 for 10 seconds.
1. What is the final velocity of the car after 10 seconds?
2. How far does the car travel in those 10 seconds?`),
			),
		},
		Reasoning: &llmsdk.ReasoningOptions{
			Enabled: true,
		},
	})

	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	var reasoningParts, otherParts []llmsdk.Part
	for _, part := range response.Content {
		if part.Type() == llmsdk.PartTypeReasoning {
			reasoningParts = append(reasoningParts, part)
		} else {
			otherParts = append(otherParts, part)
		}
	}

	log.Println("Reasoning")
	litter.Dump(reasoningParts)

	log.Println("\nAnswer")
	litter.Dump(otherParts)
}
