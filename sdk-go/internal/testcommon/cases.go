package testcommon

import (
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/ptr"
)

// GetWeatherTool returns the standard weather tool for testing
func GetWeatherTool() llmsdk.Tool {
	return llmsdk.Tool{
		Name:        "get_weather",
		Description: "Get the weather",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]any{"type": "string"},
				"unit":     map[string]any{"type": []string{"string", "null"}, "enum": []string{"c", "f"}},
			},
			"required":             []string{"location", "unit"},
			"additionalProperties": false,
		},
	}
}

// GetStockPriceTool returns the standard stock price tool for testing
func GetStockPriceTool() llmsdk.Tool {
	return llmsdk.Tool{
		Name:        "get_stock_price",
		Description: "Get the stock price",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"symbol": map[string]any{"type": "string"},
			},
			"required":             []string{"symbol"},
			"additionalProperties": false,
		},
	}
}

// CommonTestCases returns the standard test cases that all models should support
var CommonTestCases = []TestCase{
	{
		Name: "generate text",
		Input: llmsdk.LanguageModelInput{
			Messages: []llmsdk.Message{
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart(`Respond by saying "Hello"`, nil),
				),
			},
		},
		Method: Generate,
		Output: OutputAssertion{
			Content: []PartAssertion{
				NewTextAssertion("Hello"),
			},
		},
	},
	{
		Name: "stream text",
		Input: llmsdk.LanguageModelInput{
			Messages: []llmsdk.Message{
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart(`Respond by saying "Hello"`, nil),
				),
			},
		},
		Method: Stream,
		Output: OutputAssertion{
			Content: []PartAssertion{
				NewTextAssertion("Hello"),
			},
		},
	},
	{
		Name: "generate with system prompt",
		Input: llmsdk.LanguageModelInput{
			SystemPrompt: ptr.To(`You must always start your message with "🤖"`),
			Messages: []llmsdk.Message{
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart("Hello", nil),
				),
			},
		},
		Method: Generate,
		Output: OutputAssertion{
			Content: []PartAssertion{
				NewTextAssertion("^🤖"),
			},
		},
	},
	{
		Name: "generate tool call",
		Input: llmsdk.LanguageModelInput{
			Messages: []llmsdk.Message{
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart("What's the weather like in Boston today?", nil),
				),
			},
			Tools: []llmsdk.Tool{GetWeatherTool()},
		},
		Method: Generate,
		Output: OutputAssertion{
			Content: []PartAssertion{
				NewToolCallAssertion("get_weather", map[string]string{
					"location": "Boston",
				}),
			},
		},
	},
	{
		Name: "stream tool call",
		Input: llmsdk.LanguageModelInput{
			Messages: []llmsdk.Message{
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart("What's the weather like in Boston today?", nil),
				),
			},
			Tools: []llmsdk.Tool{GetWeatherTool()},
		},
		Method: Stream,
		Output: OutputAssertion{
			Content: []PartAssertion{
				NewToolCallAssertion("get_weather", map[string]string{
					"location": "Boston",
				}),
			},
		},
	},
	{
		Name: "generate text from tool result",
		Input: llmsdk.LanguageModelInput{
			Messages: []llmsdk.Message{
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart("What's the weather like in Boston today?", nil),
				),
				llmsdk.NewAssistantMessage(
					llmsdk.NewToolCallPart("0mbnj08nt", "get_weather", map[string]any{
						"location": "Boston",
					}, nil),
				),
				llmsdk.NewToolMessage(
					llmsdk.NewToolResultPart("0mbnj08nt", "get_weather", []llmsdk.Part{
						llmsdk.NewTextPart(`{"temperature": 70, "unit": "f", "description": "Sunny"}`, nil),
					}, nil),
				),
			},
			Tools: []llmsdk.Tool{GetWeatherTool()},
		},
		Method: Generate,
		Output: OutputAssertion{
			Content: []PartAssertion{
				NewTextAssertion("(?i)70.*sunny|sunny.*70"),
			},
		},
	},
	{
		Name: "stream text from tool result",
		Input: llmsdk.LanguageModelInput{
			Messages: []llmsdk.Message{
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart("What's the weather like in Boston today?", nil),
				),
				llmsdk.NewAssistantMessage(
					llmsdk.NewToolCallPart("0mbnj08nt", "get_weather", map[string]any{
						"location": "Boston",
					}, nil),
				),
				llmsdk.NewToolMessage(
					llmsdk.NewToolResultPart("0mbnj08nt", "get_weather", []llmsdk.Part{
						llmsdk.NewTextPart(`{"temperature": 70, "unit": "f", "description": "Sunny"}`, nil),
					}, nil),
				),
			},
			Tools: []llmsdk.Tool{GetWeatherTool()},
		},
		Method: Stream,
		Output: OutputAssertion{
			Content: []PartAssertion{
				NewTextAssertion("(?i)70.*sunny|sunny.*70"),
			},
		},
	},
	{
		Name: "generate parallel tool calls",
		Input: llmsdk.LanguageModelInput{
			Messages: []llmsdk.Message{
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart("Get me the weather in Boston and the stock price of AAPL.", nil),
				),
			},
			Tools: []llmsdk.Tool{GetWeatherTool(), GetStockPriceTool()},
		},
		Method: Generate,
		Output: OutputAssertion{
			Content: []PartAssertion{
				NewToolCallAssertion("get_weather", map[string]string{
					"location": "Boston",
				}),
				NewToolCallAssertion("get_stock_price", map[string]string{
					"symbol": "AAPL",
				}),
			},
		},
	},
	{
		Name: "stream parallel tool calls",
		Input: llmsdk.LanguageModelInput{
			Messages: []llmsdk.Message{
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart("Get me the weather in Boston and the stock price of AAPL. You must do both of them in one go.", nil),
				),
			},
			Tools: []llmsdk.Tool{GetWeatherTool(), GetStockPriceTool()},
		},
		Method: Stream,
		Output: OutputAssertion{
			Content: []PartAssertion{
				NewToolCallAssertion("get_weather", map[string]string{
					"location": "Boston",
				}),
				NewToolCallAssertion("get_stock_price", map[string]string{
					"symbol": "AAPL",
				}),
			},
		},
	},
	{
		Name: "stream parallel tool calls of same name",
		Input: llmsdk.LanguageModelInput{
			Messages: []llmsdk.Message{
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart("Get me the weather in Boston and the weather in New York.", nil),
				),
			},
			Tools: []llmsdk.Tool{GetWeatherTool()},
		},
		Method: Stream,
		Output: OutputAssertion{
			Content: []PartAssertion{
				NewToolCallAssertion("get_weather", map[string]string{
					"location": "Boston",
				}),
				NewToolCallAssertion("get_weather", map[string]string{
					"location": "New York",
				}),
			},
		},
	},
	{
		Name:                 "structured response format",
		RequiredCapabilities: []llmsdk.LanguageModelCapability{llmsdk.CapabilityStructuredOutput},
		Input: llmsdk.LanguageModelInput{
			Messages: []llmsdk.Message{
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart(`Create a user with the id "a1b2c3", name "John Doe", email "john.doe@example.com", birthDate "1990-05-15", age 34, isActive true, role "user", accountBalance 500.75, phoneNumber "+1234567890123", tags ["developer", "gamer"], and lastLogin "2024-11-09T10:30:00Z".`, nil),
				),
			},
			ResponseFormat: &llmsdk.ResponseFormatOption{
				JSON: &llmsdk.ResponseFormatJSON{
					Name: "user",
					Schema: &llmsdk.JSONSchema{
						"type": "object",
						"properties": map[string]any{
							"id":             map[string]any{"type": "string"},
							"name":           map[string]any{"type": "string"},
							"email":          map[string]any{"type": "string"},
							"birthDate":      map[string]any{"type": "string"},
							"age":            map[string]any{"type": "integer"},
							"isActive":       map[string]any{"type": "boolean"},
							"role":           map[string]any{"type": "string"},
							"accountBalance": map[string]any{"type": "number"},
							"phoneNumber":    map[string]any{"type": "string"},
							"tags":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							"lastLogin":      map[string]any{"type": "string"},
						},
						"required": []string{
							"id", "name", "email", "birthDate", "age", "isActive",
							"role", "accountBalance", "phoneNumber", "tags", "lastLogin",
						},
						"additionalProperties": false,
					},
				},
			},
		},
		Method: Generate,
		Output: OutputAssertion{
			Content: []PartAssertion{
				NewTextAssertion(`"id"\s*:\s*"a1b2c3"`),
				NewTextAssertion(`"name"\s*:\s*"John Doe"`),
				NewTextAssertion(`"email"\s*:\s*"john\.doe@example\.com"`),
			},
		},
	},
}
