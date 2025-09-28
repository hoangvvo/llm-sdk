package main

import (
	"fmt"
	"os"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/anthropic"
	"github.com/hoangvvo/llm-sdk/sdk-go/google"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
)

func getModel(provider, modelID string, metadata llmsdk.LanguageModelMetadata, apiKey string) llmsdk.LanguageModel {
	godotenv.Load("../.env")

	switch provider {
	case "openai":
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			panic("OPENAI_API_KEY is not set")
		}
		return openai.NewOpenAIModel(modelID, openai.OpenAIModelOptions{
			APIKey: apiKey,
		}).WithMetadata(&metadata)

	case "openai-chat-completion":
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			panic("OPENAI_API_KEY is not set")
		}
		return openai.NewOpenAIChatModel(modelID, openai.OpenAIChatModelOptions{
			APIKey: apiKey,
		}).WithMetadata(&metadata)

	case "anthropic":
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if apiKey == "" {
			panic("ANTHROPIC_API_KEY is not set")
		}
		return anthropic.NewAnthropicModel(modelID, anthropic.AnthropicModelOptions{
			APIKey: apiKey,
		}).WithMetadata(&metadata)

	case "google":
		if apiKey == "" {
			apiKey = os.Getenv("GOOGLE_API_KEY")
		}
		if apiKey == "" {
			panic("GOOGLE_API_KEY is not set")
		}
		return google.NewGoogleModel(modelID, google.GoogleModelOptions{
			APIKey: apiKey,
		}).WithMetadata(&metadata)

	default:
		panic(fmt.Sprintf("Unsupported provider: %s", provider))
	}
}
