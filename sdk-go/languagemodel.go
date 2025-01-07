package llmsdk

import "context"

type ProviderName string

type LanguageModel interface {
	Provider() ProviderName
	ModelID() string
	Generate(ctx context.Context, input *LanguageModelInput) (*ModelResponse, error)
	Stream(ctx context.Context, input *LanguageModelInput) (*StreamResponse, error)
}
