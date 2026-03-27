package main

import (
	"context"
	"fmt"
	"log"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
	"github.com/sanity-io/litter"
)

const redPixelPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg=="

type colorSample struct {
	MimeType string
	Data     string
}

func getColorSample() colorSample {
	fmt.Println("[TOOLS getColorSample()] Returning a red sample image")
	return colorSample{
		MimeType: "image/png",
		Data:     redPixelPNGBase64,
	}
}

func main() {
	model := examples.GetModel("openai", "gpt-4o")

	maxTurnLeft := 10

	tools := []llmsdk.Tool{
		{
			Name:        "get_color_sample",
			Description: "Get a color sample image",
			Parameters: llmsdk.JSONSchema{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
	}

	messages := []llmsdk.Message{
		llmsdk.NewUserMessage(
			llmsdk.NewTextPart("What color is the image returned by the tool? Answer with one word."),
		),
	}

	var response *llmsdk.ModelResponse
	var err error

	for maxTurnLeft > 0 {
		response, err = model.Generate(context.Background(), &llmsdk.LanguageModelInput{
			Messages: messages,
			Tools:    tools,
		})

		if err != nil {
			log.Fatalf("Generation failed: %v", err)
		}

		messages = append(messages, llmsdk.NewAssistantMessage(response.Content...))

		var hasToolCalls bool
		var toolMessage *llmsdk.ToolMessage

		for _, part := range response.Content {
			if part.ToolCallPart == nil {
				continue
			}

			hasToolCalls = true
			toolCallPart := part.ToolCallPart

			var toolResult colorSample
			switch toolCallPart.ToolName {
			case "get_color_sample":
				toolResult = getColorSample()
			default:
				log.Fatalf("Tool %s not found", toolCallPart.ToolName)
			}

			if toolMessage == nil {
				toolMessage = &llmsdk.ToolMessage{Content: []llmsdk.Part{}}
			}

			toolMessage.Content = append(toolMessage.Content,
				llmsdk.NewToolResultPart(
					toolCallPart.ToolCallID,
					toolCallPart.ToolName,
					[]llmsdk.Part{
						llmsdk.NewImagePart(toolResult.Data, toolResult.MimeType),
					},
				),
			)
		}

		if !hasToolCalls {
			break
		}

		if toolMessage != nil {
			messages = append(messages, llmsdk.NewToolMessage(toolMessage.Content...))
		}

		maxTurnLeft--
	}

	litter.Dump(response)
}
