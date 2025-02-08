package llmsdk

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "github.com/hoangvvo/llm-sdk/sdk-go"
)

var tracer trace.Tracer

func getTracer() trace.Tracer {
	if tracer == nil {
		tracer = otel.Tracer(tracerName)
	}
	return tracer
}

type LMSpan struct {
	startTime          time.Time
	span               trace.Span
	streamPartialUsage *ModelUsage
	timeToFirstToken   *float64
}

func NewLMSpan(ctx context.Context, provider string, modelID string, method string, input *LanguageModelInput) (context.Context, *LMSpan) {
	spanCtx, span := getTracer().Start(ctx, "llm_sdk."+method,
		trace.WithAttributes(
			// https://opentelemetry.io/docs/specs/semconv/gen-ai/
			attribute.String("gen_ai.operation.name", "generate_content"),
			attribute.String("gen_ai.provider.name", provider),
			attribute.String("gen_ai.request.model", modelID),
		))

	// Add optional attributes if they exist
	if input.Seed != nil {
		span.SetAttributes(attribute.Int64("gen_ai.request.seed", int64(*input.Seed)))
	}
	if input.FrequencyPenalty != nil {
		span.SetAttributes(attribute.Float64("gen_ai.request.frequency_penalty", *input.FrequencyPenalty))
	}
	if input.MaxTokens != nil {
		span.SetAttributes(attribute.Int("gen_ai.request.max_tokens", int(*input.MaxTokens)))
	}
	if input.PresencePenalty != nil {
		span.SetAttributes(attribute.Float64("gen_ai.request.presence_penalty", *input.PresencePenalty))
	}
	if input.Temperature != nil {
		span.SetAttributes(attribute.Float64("gen_ai.request.temperature", *input.Temperature))
	}
	if input.TopK != nil {
		span.SetAttributes(attribute.Float64("gen_ai.request.top_k", *input.TopK))
	}
	if input.TopP != nil {
		span.SetAttributes(attribute.Float64("gen_ai.request.top_p", *input.TopP))
	}

	return spanCtx, &LMSpan{
		startTime: time.Now(),
		span:      span,
	}
}

func (s *LMSpan) OnResponse(response *ModelResponse) {
	if response.Usage != nil {
		s.span.SetAttributes(
			attribute.Int("gen_ai.usage.input_tokens", response.Usage.InputTokens),
			attribute.Int("gen_ai.usage.output_tokens", response.Usage.OutputTokens),
		)
	}
}

func (s *LMSpan) OnStreamPartial(partial *PartialModelResponse) {
	if partial.Usage != nil {
		if s.streamPartialUsage == nil {
			s.streamPartialUsage = &ModelUsage{
				InputTokens:  0,
				OutputTokens: 0,
			}
		}
		s.streamPartialUsage.InputTokens += partial.Usage.InputTokens
		s.streamPartialUsage.OutputTokens += partial.Usage.OutputTokens
		s.span.SetAttributes(
			attribute.Int("gen_ai.usage.input_tokens", s.streamPartialUsage.InputTokens),
			attribute.Int("gen_ai.usage.output_tokens", s.streamPartialUsage.OutputTokens),
		)
	}
	if partial.Delta != nil && s.timeToFirstToken == nil {
		elapsed := time.Since(s.startTime).Seconds()
		s.timeToFirstToken = &elapsed
		s.span.SetAttributes(
			attribute.Float64("gen_ai.server.time_to_first_token", elapsed),
		)
	}
}

func (s *LMSpan) OnError(err error) {
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
}

func (s *LMSpan) OnEnd() {
	s.span.End()
}
