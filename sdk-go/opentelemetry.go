package llmsdk

import (
	"context"
	"time"

	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("github.com/hoangvvo/llm-sdk/sdk-go")

type LMSpan struct {
	Provider  string      `json:"provider"`
	ModelID   string      `json:"model_id"`
	Usage     *ModelUsage `json:"usage,omitempty"`
	Cost      *float64    `json:"cost,omitempty"`
	StartTime time.Time   `json:"start_time"`
	// Time to first token, in seconds
	TimeToFirstToken *float64 `json:"time_to_first_token,omitempty"`
	MaxTokens        *uint32  `json:"max_tokens,omitempty"`
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	TopK             *int32   `json:"top_k,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	Seed             *int64   `json:"seed,omitempty"`

	span trace.Span
}

func NewLMSpan(ctx context.Context, provider string, modelID string, method string, input *LanguageModelInput) (context.Context, *LMSpan) {
	spanCtx, span := tracer.Start(ctx, "llm_sdk."+method)

	return spanCtx, &LMSpan{
		Provider:         provider,
		ModelID:          modelID,
		StartTime:        time.Now(),
		MaxTokens:        input.MaxTokens,
		Temperature:      input.Temperature,
		TopP:             input.TopP,
		TopK:             input.TopK,
		PresencePenalty:  input.PresencePenalty,
		FrequencyPenalty: input.FrequencyPenalty,
		Seed:             input.Seed,
		span:             span,
	}
}

func (s *LMSpan) OnStreamPartial(partial *PartialModelResponse) {
	if partial.Usage != nil {
		if s.Usage == nil {
			s.Usage = &ModelUsage{}
		}
		s.Usage.Add(partial.Usage)
	}
	if partial.Delta != nil && s.TimeToFirstToken == nil {
		s.TimeToFirstToken = ptr.To(time.Since(s.StartTime).Seconds())
	}
}

func (s *LMSpan) OnResponse(response *ModelResponse) {
	if response.Usage != nil {
		s.Usage = response.Usage
	}
}

func (s *LMSpan) OnError(err error) {
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
}

func (s *LMSpan) OnEnd() {
	// https://opentelemetry.io/docs/specs/semconv/gen-ai/
	s.span.SetAttributes(
		attribute.String("gen_ai.operation.name", "generate_content"),
		attribute.String("gen_ai.provider.name", s.Provider),
		attribute.String("gen_ai.request.model", s.ModelID),
	)
	if s.Usage != nil {
		s.span.SetAttributes(
			attribute.Int("gen_ai.usage.input_tokens", s.Usage.InputTokens),
			attribute.Int("gen_ai.usage.output_tokens", s.Usage.OutputTokens),
		)
	}
	if s.Cost != nil {
		s.span.SetAttributes(attribute.Float64("llm_sdk.cost", *s.Cost))
	}
	if s.TimeToFirstToken != nil {
		s.span.SetAttributes(attribute.Float64("gen_ai.server.time_to_first_token", *s.TimeToFirstToken))
	}
	if s.MaxTokens != nil {
		s.span.SetAttributes(attribute.Int64("gen_ai.request.max_tokens", int64(*s.MaxTokens)))
	}
	if s.Temperature != nil {
		s.span.SetAttributes(attribute.Float64("gen_ai.request.temperature", *s.Temperature))
	}
	if s.TopP != nil {
		s.span.SetAttributes(attribute.Float64("gen_ai.request.top_p", *s.TopP))
	}
	if s.TopK != nil {
		s.span.SetAttributes(attribute.Int64("gen_ai.request.top_k", int64(*s.TopK)))
	}
	if s.PresencePenalty != nil {
		s.span.SetAttributes(attribute.Float64("gen_ai.request.presence_penalty", *s.PresencePenalty))
	}
	if s.FrequencyPenalty != nil {
		s.span.SetAttributes(attribute.Float64("gen_ai.request.frequency_penalty", *s.FrequencyPenalty))
	}
	if s.Seed != nil {
		s.span.SetAttributes(attribute.Int64("gen_ai.request.seed", *s.Seed))
	}
	s.span.End()
}
