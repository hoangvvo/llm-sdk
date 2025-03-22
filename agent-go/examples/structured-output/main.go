package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
)

type SearchFlightsParams struct {
	From string `json:"from"`
	To   string `json:"to"`
	Date string `json:"date"`
}

type SearchFlightsTool struct{}

func (t *SearchFlightsTool) Name() string {
	return "search_flights"
}

func (t *SearchFlightsTool) Description() string {
	return "Search for flights between two cities"
}

func (t *SearchFlightsTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"from": map[string]any{
				"type":        "string",
				"description": "Origin city/airport",
			},
			"to": map[string]any{
				"type":        "string",
				"description": "Destination city/airport",
			},
			"date": map[string]any{
				"type":        "string",
				"description": "Departure date in YYYY-MM-DD",
			},
		},
		"required":             []string{"from", "to", "date"},
		"additionalProperties": false,
	}
}

func (t *SearchFlightsTool) Execute(ctx context.Context, paramsJSON json.RawMessage, contextVal struct{}, runState *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params SearchFlightsParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	fmt.Printf("Searching flights from %s to %s on %s\n", params.From, params.To, params.Date)

	result := []map[string]any{
		{
			"airline":   "Vietnam Airlines",
			"departure": fmt.Sprintf("%sT10:00:00", params.Date),
			"arrival":   fmt.Sprintf("%sT12:00:00", params.Date),
			"price":     150,
		},
		{
			"airline":   "Southwest Airlines",
			"departure": fmt.Sprintf("%sT11:00:00", params.Date),
			"arrival":   fmt.Sprintf("%sT13:00:00", params.Date),
			"price":     120,
		},
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return llmagent.AgentToolResult{}, err
	}

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(string(resultJSON)),
		},
		IsError: false,
	}, nil
}

type SearchHotelsParams struct {
	City    string `json:"city"`
	CheckIn string `json:"check_in"`
	Nights  int    `json:"nights"`
}

type SearchHotelsTool struct{}

func (t *SearchHotelsTool) Name() string {
	return "search_hotels"
}

func (t *SearchHotelsTool) Description() string {
	return "Search for hotels in a specific location"
}

func (t *SearchHotelsTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{
				"type":        "string",
				"description": "City to search hotels in",
			},
			"check_in": map[string]any{
				"type":        "string",
				"description": "Check-in date in YYYY-MM-DD",
			},
			"nights": map[string]any{
				"type":        "integer",
				"description": "Number of nights to stay",
			},
		},
		"required":             []string{"city", "check_in", "nights"},
		"additionalProperties": false,
	}
}

func (t *SearchHotelsTool) Execute(ctx context.Context, paramsJSON json.RawMessage, contextVal struct{}, runState *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params SearchHotelsParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	fmt.Printf("Searching hotels in %s from %s for %d nights\n", params.City, params.CheckIn, params.Nights)

	result := []map[string]any{
		{
			"name":          "The Plaza",
			"location":      params.City,
			"pricePerNight": 150,
			"rating":        4.8,
		},
		{
			"name":          "Hotel Ritz",
			"location":      params.City,
			"pricePerNight": 200,
			"rating":        4.7,
		},
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return llmagent.AgentToolResult{}, err
	}

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(string(resultJSON)),
		},
		IsError: false,
	}, nil
}

func main() {
	godotenv.Load("../.env")

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable must be set")
	}

	model := openai.NewOpenAIModel("gpt-4o", openai.OpenAIModelOptions{
		APIKey: apiKey,
	})

	// Define the response format
	responseSchema := llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"destination": map[string]any{
				"type": "string",
			},
			"flights": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"airline": map[string]any{
							"type": "string",
						},
						"departure": map[string]any{
							"type": "string",
						},
						"arrival": map[string]any{
							"type": "string",
						},
						"price": map[string]any{
							"type": "number",
						},
					},
					"required":             []string{"airline", "departure", "arrival", "price"},
					"additionalProperties": false,
				},
			},
			"hotels": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
						"location": map[string]any{
							"type": "string",
						},
						"pricePerNight": map[string]any{
							"type": "number",
						},
						"rating": map[string]any{
							"type": "number",
						},
					},
					"required":             []string{"name", "location", "pricePerNight", "rating"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"destination", "flights", "hotels"},
		"additionalProperties": false,
	}

	description := "A structured travel plan including flights, hotels, and weather forecast."
	responseFormat := llmsdk.NewResponseFormatJSON("travel_plan", &description, &responseSchema)

	staticInstruction := "You are Bob, a travel agent that helps users plan their trips."
	dynamicInstruction := func(ctx context.Context, ctxVal struct{}) (string, error) {
		return fmt.Sprintf("The current time is %s", time.Now().Format(time.RFC3339)), nil
	}

	travelAgent := llmagent.NewAgent("Bob", model,
		llmagent.WithInstructions(
			llmagent.InstructionParam[struct{}]{String: &staticInstruction},
			llmagent.InstructionParam[struct{}]{Func: dynamicInstruction},
		),
		llmagent.WithResponseFormat[struct{}](*responseFormat),
		llmagent.WithTools(
			&SearchFlightsTool{},
			&SearchHotelsTool{},
		),
	)

	prompt := "Plan a trip from Paris to Tokyo next week"

	response, err := travelAgent.Run(context.Background(), llmagent.AgentRequest[struct{}]{
		Input: []llmagent.AgentItem{
			llmagent.NewMessageAgentItem(
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart(prompt),
				),
			),
		},
		Context: struct{}{},
	})

	if err != nil {
		log.Fatal(err)
	}

	// Parse and pretty print the JSON response
	var val map[string]any
	if err := json.Unmarshal([]byte(response.Text()), &val); err != nil {
		log.Fatalf("Invalid JSON response: %v", err)
	}

	prettyJSON, err := json.MarshalIndent(val, "", "  ")
	if err != nil {
		log.Fatalf("Failed to format JSON: %v", err)
	}

	fmt.Println(string(prettyJSON))
}
