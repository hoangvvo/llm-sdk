package examples

import (
	"fmt"
	"os"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/anthropic"
	"github.com/hoangvvo/llm-sdk/sdk-go/google"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load("../.env")
}

// GetModel creates and returns a language model based on provider and model ID
func GetModel(provider, modelID string) llmsdk.LanguageModel {
	switch provider {
	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			panic("OPENAI_API_KEY environment variable is required")
		}
		return openai.NewOpenAIModel(modelID, openai.OpenAIModelOptions{
			APIKey: apiKey,
		})
	case "openai-chat-completion":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			panic("OPENAI_API_KEY environment variable is required")
		}
		return openai.NewOpenAIChatModel(modelID, openai.OpenAIChatModelOptions{
			APIKey: apiKey,
		})
	case "anthropic":
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			panic("ANTHROPIC_API_KEY environment variable is required")
		}
		return anthropic.NewAnthropicModel(modelID, anthropic.AnthropicModelOptions{
			APIKey: apiKey,
		})
	case "google":
		apiKey := os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			panic("GOOGLE_API_KEY environment variable is required")
		}
		return google.NewGoogleModel(modelID, google.GoogleModelOptions{
			APIKey: apiKey,
		})
	default:
		panic(fmt.Sprintf("Unsupported provider: %s", provider))
	}
}
