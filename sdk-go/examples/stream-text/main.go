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

	response, err := model.Stream(context.Background(), &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("Tell me a story.", nil),
			),
			llmsdk.NewAssistantMessage(
				llmsdk.NewTextPart("What kind of story would you like to hear?", nil),
			),
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("A fairy tale.", nil),
			),
		},
	})

	if err != nil {
		log.Fatalf("Stream failed: %v", err)
	}

	accumulator := llmsdk.NewStreamAccumulator()

	for response.Next() {
		current := response.Current()
		fmt.Println(examples.ToJSONString(current))

		if err := accumulator.AddPartial(*current); err != nil {
			log.Printf("Failed to add partial: %v", err)
		}
	}

	if err := response.Err(); err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	finalResponse, err := accumulator.ComputeResponse()
	if err != nil {
		log.Fatalf("Failed to compute response: %v", err)
	}

	fmt.Printf("Final response: %s\n", examples.ToJSONString(finalResponse))
}
