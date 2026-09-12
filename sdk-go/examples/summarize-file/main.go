package main

import (
	"context"
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"os"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
	"github.com/sanity-io/litter"
)

func main() {
	fileURL := "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf"
	resp, err := http.Get(fileURL)
	if err != nil {
		log.Fatalf("Failed to fetch file: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Failed to fetch file: %s", resp.Status)
	}
	fileBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

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
				llmsdk.NewFilePart(
					base64.StdEncoding.EncodeToString(fileBytes),
					"application/pdf",
					llmsdk.WithFileFilename("dummy.pdf"),
				),
			),
		},
	})
	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	litter.Dump(response)
}
