package main

import (
	"context"
	"log"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
	"github.com/sanity-io/litter"
)

func main() {
	model := examples.GetModel("anthropic", "claude-opus-4-20250514")

	stream, err := model.Stream(context.Background(), &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				// Provide sources as part of the user message
				llmsdk.NewSourcePart(
					"https://health-site.example/articles/coffee-benefits",
					"Coffee Health Benefits: What the Research Shows",
					[]llmsdk.Part{llmsdk.NewTextPart(
						"Coffee contains over 1,000 bioactive compounds, with caffeine being the most studied. " +
							"A typical 8-ounce cup contains 80-100mg of caffeine. Research shows moderate coffee consumption (3-4 cups daily) " +
							"is associated with reduced risk of type 2 diabetes, Parkinson's disease, and liver disease. The antioxidants in coffee, " +
							"particularly chlorogenic acid, may contribute to these protective effects beyond just the caffeine content.",
					),
					},
				),
				llmsdk.NewTextPart(
					"Based on what you know about coffee's health benefits and caffeine content, what would be the optimal daily coffee consumption "+
						"for someone who wants the health benefits but is sensitive to caffeine? Consider timing and metabolism.",
				),
			),
			llmsdk.NewAssistantMessage(
				// The model requests a tool call to get more data, which includes sources
				llmsdk.NewToolCallPart(
					"caffeine_lookup_456",
					"lookup",
					map[string]any{
						"query": "caffeine sensitivity optimal timing metabolism coffee health benefits",
					},
				),
			),
			llmsdk.NewToolMessage(
				llmsdk.NewToolResultPart(
					"caffeine_lookup_456",
					"lookup",
					[]llmsdk.Part{
						// Provide other sources as part of the tool result
						llmsdk.NewSourcePart(
							"https://medical-journal.example/2024/caffeine-metabolism-study",
							"Optimizing Coffee Intake for Caffeine-Sensitive Individuals",
							[]llmsdk.Part{llmsdk.NewTextPart(
								"For caffeine-sensitive individuals, the half-life of caffeine extends to 8-12 hours compared to the average 5-6 hours. " +
									"These individuals experience effects at doses as low as 50mg. Research shows consuming 1-2 cups (100-200mg caffeine) before noon " +
									"provides 75% of coffee's antioxidant benefits while minimizing side effects like insomnia and anxiety. Splitting intake into smaller " +
									"doses (half-cups) throughout the morning can further reduce sensitivity reactions while maintaining beneficial compound levels.",
							),
							},
						),
					},
					false,
				),
			),
		},
	})

	if err != nil {
		log.Fatalf("Stream failed: %v", err)
	}

	accumulator := llmsdk.NewStreamAccumulator()

	for stream.Next() {
		current := stream.Current()
		litter.Dump(current)

		if err := accumulator.AddPartial(*current); err != nil {
			log.Printf("Failed to add partial: %v", err)
		}
	}

	if err := stream.Err(); err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	finalResponse, err := accumulator.ComputeResponse()
	if err != nil {
		log.Fatalf("Failed to compute response: %v", err)
	}

	litter.Dump(finalResponse.Content)
}
