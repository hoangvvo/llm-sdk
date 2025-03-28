package main

import (
	"context"
	"encoding/base64"
	"io"
	"log"
	"net/http"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
)

func main() {
	audioURL := "https://archive.org/download/MLKDream/MLKDream.ogg"

	resp, err := http.Get(audioURL)
	if err != nil {
		log.Fatalf("Failed to fetch audio: %v", err)
	}
	defer resp.Body.Close()

	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read audio: %v", err)
	}

	audioData := base64.StdEncoding.EncodeToString(audioBytes)

	model := examples.GetModel("google", "gemini-2.0-flash")

	response, err := model.Generate(context.Background(), &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("What is this speech about?"),
				llmsdk.NewAudioPart(audioData, llmsdk.AudioFormatOpus),
			),
		},
	})

	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	log.Println(examples.ToJSONString(response))
}
