package llmagent

import (
	"context"
	"fmt"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Initialize the tracer lazily to allow user to have a chance to configure the global tracer provider
var tracer = otel.Tracer("github.com/hoangvvo/llm-agent")

// AgentSpan manages the span for an agent run
type AgentSpan struct {
	agentName string
	method    string
	usage     *llmsdk.ModelUsage
	cost      float64
	span      trace.Span
}

// NewAgentSpan creates a new agent span
func NewAgentSpan(ctx context.Context, agentName string, method string) (*AgentSpan, context.Context) {
	spanName := fmt.Sprintf("llm_agent.%s", method)
	newCtx, span := tracer.Start(ctx, spanName)

	return &AgentSpan{
		agentName: agentName,
		method:    method,
		usage:     nil,
		cost:      0,
		span:      span,
	}, newCtx
}

// OnResponse updates the span with response information
func (s *AgentSpan) OnResponse(response *AgentResponse) {
	for _, item := range response.Output {
		if item.Model != nil {
			if item.Model.Usage != nil {
				if s.usage == nil {
					s.usage = &llmsdk.ModelUsage{}
				}
				s.usage.Add(item.Model.Usage)
			}
			if item.Model.Cost != nil {
				s.cost += *item.Model.Cost
			}
		}
	}
}

// OnEnd ends the span and sets the final attributes
func (s *AgentSpan) OnEnd() {
	attrs := []attribute.KeyValue{
		attribute.String("gen_ai.operation.name", "invoke_agent"),
		attribute.String("gen_ai.agent.name", s.agentName),
	}

	if s.usage != nil {
		attrs = append(attrs, attribute.Int64("gen_ai.model.input_tokens", int64(s.usage.InputTokens)))
		attrs = append(attrs, attribute.Int64("gen_ai.model.output_tokens", int64(s.usage.OutputTokens)))
	}

	if s.cost > 0 {
		attrs = append(attrs, attribute.Float64("llm_agent.cost", s.cost))
	}

	s.span.SetAttributes(attrs...)
	s.span.End()
}

// OnError records an error and ends the span
func (s *AgentSpan) OnError(err error) {
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
	s.span.End()
}

// startActiveToolSpan creates a span for tool execution
func startActiveToolSpan(
	ctx context.Context,
	toolCallID string,
	toolName string,
	toolDescription string,
	fn func(context.Context) (AgentToolResult, error),
) (AgentToolResult, error) {
	spanCtx, span := tracer.Start(ctx, "llm_agent.tool")
	defer func() {
		// Set attributes following OpenTelemetry semantic conventions
		span.SetAttributes(
			attribute.String("gen_ai.operation.name", "execute_tool"),
			attribute.String("gen_ai.tool.call.id", toolCallID),
			attribute.String("gen_ai.tool.description", toolDescription),
			attribute.String("gen_ai.tool.name", toolName),
			attribute.String("gen_ai.tool.type", "function"),
		)
		span.End()
	}()

	res, err := fn(spanCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return AgentToolResult{}, err
	}

	return res, nil
}
