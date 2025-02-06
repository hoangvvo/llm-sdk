package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
	examplescommon "github.com/hoangvvo/llm-sdk/sdk-go/examples"
)

func main() {
	imageURL := "https://images.unsplash.com/photo-1464809142576-df63ca4ed7f0"

	resp, err := http.Get(imageURL)
	if err != nil {
		log.Fatalf("Failed to fetch image: %v", err)
	}
	defer resp.Body.Close()

	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read image: %v", err)
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	imageData := base64.StdEncoding.EncodeToString(imageBytes)

	model := examples.GetModel("openai", "gpt-4o")

	response, err := model.Generate(context.Background(), &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("Describe this image"),
				llmsdk.NewImagePart(mimeType, imageData, nil, nil),
			),
		},
	})

	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	fmt.Println(examplescommon.ToJSONString(response))
}
