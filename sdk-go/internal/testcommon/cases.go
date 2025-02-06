package testcommon

import (
	"context"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/internal/ptr"
)

// TestCase represents a complete test case
type TestCase struct {
	Name             string
	Input            llmsdk.LanguageModelInput
	Method           TestMethod
	Output           OutputAssertion
	CompatibleSchema bool
}

// RunTestCase executes a single test case
func RunTestCase(t *testing.T, model llmsdk.LanguageModel, testCase TestCase) {
	t.Helper()

	ctx := context.Background()

	switch testCase.Method {
	case Generate:
		result, err := model.Generate(ctx, &testCase.Input)
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		assertContentPart(t, result.Content, testCase.Output.Content)
	case Stream:
		stream, err := model.Stream(ctx, &testCase.Input)
		if err != nil {
			t.Fatalf("Stream failed: %v", err)
		}

		accumulator := llmsdk.NewStreamAccumulator()
		for stream.Next() {
			partial := stream.Current()
			if err := accumulator.AddPartial(*partial); err != nil {
				t.Fatalf("Failed to add partial: %v", err)
			}
		}

		if err := stream.Err(); err != nil {
			t.Fatalf("Stream error: %v", err)
		}

		result, err := accumulator.ComputeResponse()
		if err != nil {
			t.Fatalf("Failed to compute response: %v", err)
		}
		assertContentPart(t, result.Content, testCase.Output.Content)
	}
}

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

var TestCaseGenerateText = TestCase{
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
}

var TestCaseStreamText = TestCase{
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
}

var TestCaseGenerateWithSystemPrompt = TestCase{
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
}

var TestCaseGenerateToolCall = TestCase{
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
}

var TestCaseStreamToolCall = TestCase{
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
}

var TestCaseGenerateTextWithToolResult = TestCase{
	Name: "generate text with tool result",
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
				}, false),
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
}

var TestCaseStreamTextWithToolResult = TestCase{
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
				}, false),
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
}

var TestCaseGenerateParallelToolCalls = TestCase{
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
}

var TestCaseStreamParallelToolCalls = TestCase{
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
}

var TestCaseStreamParallelToolCallsOfSameName = TestCase{
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
}

var TestCaseStructuredResponseFormat = TestCase{
	Name: "structured response format",
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
}

var TestCaseSourcePartInput = TestCase{
	Name: "source part in content",
	Input: llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("What is my first secret number?", nil),
			),
			llmsdk.NewAssistantMessage(
				llmsdk.NewToolCallPart("0mbnj08nt", "get_first_secret_number", map[string]any{}, nil),
			),
			llmsdk.NewToolMessage(
				llmsdk.NewToolResultPart("0mbnj08nt", "get_first_secret_number", []llmsdk.Part{
					llmsdk.NewTextPart(`{"number": 24}`, nil),
				}, false),
			),
			llmsdk.NewAssistantMessage(
				llmsdk.NewTextPart("Got it!", nil),
			),
			llmsdk.NewUserMessage(
				llmsdk.NewSourcePart("my secret number", []llmsdk.Part{
					llmsdk.NewTextPart("Remember that my second secret number is \"42\".", nil),
				}, nil),
				llmsdk.NewTextPart(" What are my two secret numbers?", nil),
			),
		},
	},
	Method: Generate,
	Output: OutputAssertion{
		Content: []PartAssertion{
			NewTextAssertion("24"),
			NewTextAssertion("42"),
		},
	},
}
