package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
	"github.com/sanity-io/litter"
)

type lookupHolidayArgs struct {
	Country string `json:"country"`
	Year    int    `json:"year"`
}

func lookupHoliday(args lookupHolidayArgs) string {
	log.Printf("[TOOLS lookup_holiday()] %s %d", args.Country, args.Year)

	// Sample data for this example. Replace with a holiday service in an application.
	if strings.ToUpper(args.Country) != "VN" || args.Year != 2026 {
		log.Fatal("Sample data is only available for VN in 2026")
	}
	return `{"holidays":[{"date":"2026-09-02","name":"National Day"}]}`
}

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

	tools := []llmsdk.Tool{
		llmsdk.NewToolSearchTool(),
		llmsdk.NewFunctionTool(
			"lookup_holiday",
			"Look up the public holidays of a country in a given year",
			llmsdk.JSONSchema{
				"type": "object",
				"properties": map[string]any{
					"country": map[string]any{
						"type":        "string",
						"description": "ISO 3166-1 alpha-2 country code",
					},
					"year": map[string]any{"type": "integer"},
				},
				"required":             []string{"country", "year"},
				"additionalProperties": false,
			},
			// The provider loads this definition when the model finds it through search.
			llmsdk.WithFunctionToolDeferLoading(),
		),
	}

	messages := []llmsdk.Message{
		llmsdk.NewUserMessage(llmsdk.NewTextPart(
			"Use tool search to find a tool that lists public holidays, then call it for Vietnam (country code VN) in 2026. Do not answer without calling the holiday tool.",
		)),
	}

	for turn := 0; turn < 10; turn++ {
		// Keep all tool definitions available on every request, including deferred ones.
		stream, err := model.Stream(context.Background(), &llmsdk.LanguageModelInput{
			Messages: messages,
			Tools:    tools,
		})
		if err != nil {
			log.Fatalf("Stream failed: %v", err)
		}
		accumulator := llmsdk.NewStreamAccumulator()

		for stream.Next() {
			partial := stream.Current()
			litter.Dump(partial)
			if err := accumulator.AddPartial(*partial); err != nil {
				log.Fatalf("Failed to add partial: %v", err)
			}
		}
		if err := stream.Err(); err != nil {
			log.Fatalf("Stream error: %v", err)
		}

		// Execute function calls only after their streamed arguments are complete.
		response, err := accumulator.ComputeResponse()
		if err != nil {
			log.Fatalf("Failed to compute response: %v", err)
		}

		// Preserve the complete response, including hosted search calls and results.
		messages = append(messages, llmsdk.NewAssistantMessage(response.Content...))

		var toolResults []llmsdk.Part
		for _, part := range response.Content {
			switch {
			case part.ToolCallPart != nil && part.ToolCallPart.Call.ToolSearch != nil:
				log.Printf("tool search: %#v", part.ToolCallPart.Call.ToolSearch)
			case part.ToolResultPart != nil && part.ToolResultPart.Result.ToolSearch != nil:
				log.Printf("discovered tools: %s %v", part.ToolResultPart.Status, part.ToolResultPart.Result.ToolSearch.ToolNames)
			case part.ToolCallPart != nil && part.ToolCallPart.Call.Function != nil:
				// The provider executes tool search; the application executes function calls.
				call := part.ToolCallPart.Call.Function
				if call.Name != "lookup_holiday" {
					log.Fatalf("Tool %s not found", call.Name)
				}
				var args lookupHolidayArgs
				if err := json.Unmarshal(call.Args, &args); err != nil {
					log.Fatalf("Failed to parse lookup_holiday args: %v", err)
				}
				result := lookupHoliday(args)
				toolResults = append(toolResults, llmsdk.NewToolResultPart(
					part.ToolCallPart.ToolCallID,
					call.Name,
					[]llmsdk.Part{llmsdk.NewTextPart(result)},
				))
			case part.TextPart != nil:
				log.Print(part.TextPart.Text)
			}
		}

		if len(toolResults) == 0 {
			break
		}
		messages = append(messages, llmsdk.NewToolMessage(toolResults...))
	}
}
