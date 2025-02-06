package main

import (
	"context"
	"fmt"
	"log"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
)

func main() {
	model := examples.GetModel("openai", "gpt-4o")

	response, err := model.Generate(context.Background(), &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("Tell me a story."),
			),
			llmsdk.NewAssistantMessage(
				llmsdk.NewTextPart("What kind of story would you like to hear?"),
			),
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("A fairy tale."),
			),
		},
	})

	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	fmt.Println(examples.ToJSONString(response))
}
