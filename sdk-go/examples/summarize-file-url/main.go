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
				llmsdk.NewTextPart("Summarize the attached PDF."),
				// The provider fetches this URL; no download or base64 encoding is needed here.
				llmsdk.NewFilePartFromURL(
					"https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf",
					"application/pdf",
				),
			),
		},
	})
	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	litter.Dump(response)
}
