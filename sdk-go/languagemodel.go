package llmsdk

import (
	"context"

	"github.com/hoangvvo/llm-sdk/sdk-go/utils/stream"
)

type ProviderName string

type LanguageModelMetadata struct {
	Pricing      *LanguageModelPricing     `json:"pricing"`
	Capabilities []LanguageModelCapability `json:"capabilities,omitempty"`
}

type LanguageModel interface {
	Provider() ProviderName
	ModelID() string
	Metadata() *LanguageModelMetadata
	Generate(ctx context.Context, input *LanguageModelInput) (*ModelResponse, error)
	Stream(ctx context.Context, input *LanguageModelInput) (*LanguageModelStream, error)
}

type LanguageModelStream = stream.Stream[*PartialModelResponse]
