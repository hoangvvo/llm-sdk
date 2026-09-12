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
		modelID = "gpt-5.6-terra"
	}
	model := examples.GetModel(provider, modelID)

	response, err := model.Generate(context.Background(), &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("Describe this image"),
				llmsdk.NewImagePartFromURL(
					"https://images.unsplash.com/photo-1464809142576-df63ca4ed7f0",
					"image/jpeg",
				),
			),
		},
	})
	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	litter.Dump(response)
}
