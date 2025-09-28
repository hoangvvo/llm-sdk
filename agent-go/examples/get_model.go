package examples

import (
	"errors"
	"fmt"
	"os"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/anthropic"
	"github.com/hoangvvo/llm-sdk/sdk-go/google"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
)

func init() {
	// Load .env file if exists
	_ = godotenv.Load("../.env")
}

func GetModel(provider, modelID string, metadata llmsdk.LanguageModelMetadata, apiKey string) (llmsdk.LanguageModel, error) {
	switch provider {
	case "openai":
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			return nil, errors.New("OPENAI_API_KEY is not set")
		}
		return openai.NewOpenAIModel(modelID, openai.OpenAIModelOptions{
			APIKey: apiKey,
		}).WithMetadata(&metadata), nil

	case "openai-chat-completion":
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			return nil, errors.New("OPENAI_API_KEY is not set")
		}
		return openai.NewOpenAIChatModel(modelID, openai.OpenAIChatModelOptions{
			APIKey: apiKey,
		}).WithMetadata(&metadata), nil

	case "anthropic":
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if apiKey == "" {
			return nil, errors.New("ANTHROPIC_API_KEY is not set")
		}
		return anthropic.NewAnthropicModel(modelID, anthropic.AnthropicModelOptions{
			APIKey: apiKey,
		}).WithMetadata(&metadata), nil

	case "google":
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
		}
		if apiKey == "" {
			return nil, errors.New("GOOGLE_API_KEY is not set")
		}
		return google.NewGoogleModel(modelID, google.GoogleModelOptions{
			APIKey: apiKey,
		}).WithMetadata(&metadata), nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}
