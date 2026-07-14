package main

import (
	"context"
	"log"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
	"github.com/sanity-io/litter"
)

func main() {
	model := examples.GetModel("openai", "gpt-5.6-terra")

	response, err := model.Generate(context.Background(), llmsdk.NewLanguageModelInput(
		[]llmsdk.Message{
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
	))

	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	litter.Dump(response)
}
