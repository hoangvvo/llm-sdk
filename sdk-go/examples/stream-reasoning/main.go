package main

import (
	"context"
	"log"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
	"github.com/sanity-io/litter"
)

func main() {
	model := examples.GetModel("openai", "o1")

	stream, err := model.Stream(context.Background(), &llmsdk.LanguageModelInput{
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

	for stream.Next() {
		partial := stream.Current()
		if partial.Delta.Part.ReasoningPartDelta != nil {
			log.Println("Reasoning:")
			litter.Dump(partial.Delta.Part)
		} else {
			log.Println("Answer:")
			litter.Dump(partial.Delta.Part)
		}
	}
}
