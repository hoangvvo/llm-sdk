package examples

import (
	"fmt"
	"os"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
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
		return openai.NewOpenAIModel(openai.OpenAIModelOptions{
			APIKey:  apiKey,
			ModelID: modelID,
		})
	case "openai-chat-completion":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			panic("OPENAI_API_KEY environment variable is required")
		}
		return openai.NewOpenAIChatModel(openai.OpenAIModelOptions{
			APIKey:  apiKey,
			ModelID: modelID,
		})
	default:
		panic(fmt.Sprintf("Unsupported provider: %s", provider))
	}
}
