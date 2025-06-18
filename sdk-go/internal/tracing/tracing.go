package tracing

import (
	"context"
	"time"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/stream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("github.com/hoangvvo/llm-sdk/sdk-go")

type lmSpan struct {
	Provider  string             `json:"provider"`
	ModelID   string             `json:"model_id"`
	Usage     *llmsdk.ModelUsage `json:"usage,omitempty"`
	Cost      *float64           `json:"cost,omitempty"`
	StartTime time.Time          `json:"start_time"`
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

func TraceGenerate(
	ctx context.Context,
	provider string,
	modelID string,
	input *llmsdk.LanguageModelInput,
	fn func(context.Context) (*llmsdk.ModelResponse, error),
) (*llmsdk.ModelResponse, error) {
	ctx, span := newLMSpan(ctx, provider, modelID, "generate", input)
	defer span.OnEnd()

	response, err := fn(ctx)
	if err != nil {
		span.OnError(err)
		return nil, err
	}

	if response != nil {
		span.OnResponse(response)
	}

	return response, nil
}

func TraceStream(
	ctx context.Context,
	provider string,
	modelID string,
	input *llmsdk.LanguageModelInput,
	fn func(context.Context) (*llmsdk.LanguageModelStream, error),
) (*llmsdk.LanguageModelStream, error) {
	ctx, span := newLMSpan(ctx, provider, modelID, "stream", input)

	innerStream, err := fn(ctx)
	if err != nil {
		span.OnError(err)
		span.OnEnd()
		return nil, err
	}
	if innerStream == nil {
		span.OnEnd()
		return nil, nil
	}

	responseCh := make(chan *llmsdk.PartialModelResponse)
	errCh := make(chan error, 1)

	go func() {
		defer close(responseCh)
		defer close(errCh)
		defer span.OnEnd()

		for innerStream.Next() {
			partial := innerStream.Current()
			if partial == nil {
				continue
			}

			span.OnStreamPartial(partial)
			responseCh <- partial
		}

		if err := innerStream.Err(); err != nil {
			span.OnError(err)
			errCh <- err
		}
	}()

	return stream.New(responseCh, errCh), nil
}

func newLMSpan(
	ctx context.Context,
	provider string,
	modelID string,
	method string,
	input *llmsdk.LanguageModelInput,
) (context.Context, *lmSpan) {
	spanCtx, otelSpan := tracer.Start(ctx, "llm_sdk."+method)

	var maxTokens *uint32
	var temperature *float64
	var topP *float64
	var topK *int32
	var presencePenalty *float64
	var frequencyPenalty *float64
	var seed *int64

	if input != nil {
		maxTokens = input.MaxTokens
		temperature = input.Temperature
		topP = input.TopP
		topK = input.TopK
		presencePenalty = input.PresencePenalty
		frequencyPenalty = input.FrequencyPenalty
		seed = input.Seed
	}

	return spanCtx, &lmSpan{
		Provider:         provider,
		ModelID:          modelID,
		StartTime:        time.Now(),
		MaxTokens:        maxTokens,
		Temperature:      temperature,
		TopP:             topP,
		TopK:             topK,
		PresencePenalty:  presencePenalty,
		FrequencyPenalty: frequencyPenalty,
		Seed:             seed,
		span:             otelSpan,
	}
}

func (s *lmSpan) OnStreamPartial(partial *llmsdk.PartialModelResponse) {
	if partial == nil {
		return
	}

	if partial.Usage != nil {
		if s.Usage == nil {
			s.Usage = &llmsdk.ModelUsage{}
		}
		s.Usage.Add(partial.Usage)
	}

	if partial.Cost != nil {
		s.Cost = partial.Cost
	}

	if partial.Delta != nil && s.TimeToFirstToken == nil {
		s.TimeToFirstToken = ptr.To(time.Since(s.StartTime).Seconds())
	}
}

func (s *lmSpan) OnResponse(response *llmsdk.ModelResponse) {
	if response == nil {
		return
	}

	if response.Usage != nil {
		s.Usage = response.Usage
	}

	if response.Cost != nil {
		s.Cost = response.Cost
	}
}

func (s *lmSpan) OnError(err error) {
	if err == nil {
		return
	}
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
}

func (s *lmSpan) OnEnd() {
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
