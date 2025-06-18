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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var tracer = otel.Tracer("examples/agent-go/tracing")

func initTracing(ctx context.Context) (*sdktrace.TracerProvider, error) {
	// Configure an OTLP/HTTP exporter; defaults to OTEL_* environment variables when unset.
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			"",
			attribute.String("service.name", "agent-go-tracing-example"),
		)),
	)

	otel.SetTracerProvider(tp)
	return tp, nil
}

type tracingContext struct {
	CustomerName string
}

// Weather tool

type weatherArgs struct {
	City string `json:"city"`
}

type weatherTool struct{}

func (t *weatherTool) Name() string        { return "get_weather" }
func (t *weatherTool) Description() string { return "Get the current weather for a city" }
func (t *weatherTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"city": map[string]any{
				"type":        "string",
				"description": "City to describe",
			},
		},
		"required":             []string{"city"},
		"additionalProperties": false,
	}
}

func (t *weatherTool) Execute(ctx context.Context, payload json.RawMessage, _ tracingContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	// Bridge this tool to tracing so the agent span includes internal work.
	_, span := tracer.Start(ctx, "tools.get_weather")
	defer span.End()

	var args weatherArgs
	if err := json.Unmarshal(payload, &args); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	span.SetAttributes(attribute.String("weather.city", args.City)) // annotate the span with the lookup
	time.Sleep(120 * time.Millisecond)

	result := map[string]any{
		"city":          args.City,
		"forecast":      "Sunny",
		"temperature_c": 24,
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return llmagent.AgentToolResult{}, err
	}

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(string(encoded))},
		IsError: false,
	}, nil
}

// Notification tool

type notifyArgs struct {
	PhoneNumber string `json:"phone_number"`
	Message     string `json:"message"`
}

type notifyTool struct{}

func (t *notifyTool) Name() string        { return "send_notification" }
func (t *notifyTool) Description() string { return "Send an SMS notification" }
func (t *notifyTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"phone_number": map[string]any{
				"type":        "string",
				"description": "Recipient phone number",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "Message to send",
			},
		},
		"required":             []string{"phone_number", "message"},
		"additionalProperties": false,
	}
}

func (t *notifyTool) Execute(ctx context.Context, payload json.RawMessage, _ tracingContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	_, span := tracer.Start(ctx, "tools.send_notification")
	defer span.End()

	var args notifyArgs
	if err := json.Unmarshal(payload, &args); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	// Annotate the span with useful metadata about the notification work.
	span.SetAttributes(attribute.String("notification.phone", args.PhoneNumber))
	span.SetAttributes(attribute.Int("notification.message_length", len(args.Message)))
	time.Sleep(80 * time.Millisecond)

	encoded, err := json.Marshal(map[string]any{
		"status":       "sent",
		"phone_number": args.PhoneNumber,
		"message":      args.Message,
	})
	if err != nil {
		return llmagent.AgentToolResult{}, err
	}

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(string(encoded))},
		IsError: false,
	}, nil
}

func main() {
	ctx := context.Background()
	godotenv.Load("../.env")

	tp, err := initTracing(ctx)
	if err != nil {
		log.Fatalf("failed to init tracing: %v", err)
	}
	defer func() {
		_ = tp.Shutdown(ctx)
	}()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable must be set")
	}

	model := openai.NewOpenAIModel("gpt-4o-mini", openai.OpenAIModelOptions{APIKey: apiKey})

	agent := llmagent.NewAgent("Trace Assistant", model,
		llmagent.WithInstructions(
			// Keep these aligned with the other language examples.
			llmagent.InstructionParam[tracingContext]{String: ptr("Coordinate weather updates and notifications for clients.")},
			llmagent.InstructionParam[tracingContext]{String: ptr("When a request needs both a forecast and a notification, call get_weather before send_notification and summarize the tool results in your reply.")},
			llmagent.InstructionParam[tracingContext]{Func: func(_ context.Context, c tracingContext) (string, error) {
				return fmt.Sprintf("When asked to contact someone, include a friendly note from %s.", c.CustomerName), nil
			}},
		),
		llmagent.WithTools(&weatherTool{}, &notifyTool{}),
	)

	// Run a single turn that forces both tools to execute.
	req := llmagent.AgentRequest[tracingContext]{
		Context: tracingContext{CustomerName: "Skyline Tours"},
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(
				llmsdk.NewTextPart("Please check the weather for Seattle today and text Mia at +1-555-0100 with the summary."),
			)),
		},
	}

	// This single call emits agent + tool spans under the configured exporter.
	resp, err := agent.Run(ctx, req)
	if err != nil {
		log.Fatalf("agent run failed: %v", err)
	}

	fmt.Printf("Response: %+v\n", resp.Content)
}

func ptr[T any](v T) *T { return &v }
