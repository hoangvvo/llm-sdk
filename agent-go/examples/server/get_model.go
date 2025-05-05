package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/google"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
)

type ModelInfo struct {
	Provider   string                       `json:"provider"`
	ModelID    string                       `json:"model_id"`
	Metadata   llmsdk.LanguageModelMetadata `json:"metadata"`
	Audio      *llmsdk.AudioOptions         `json:"audio,omitempty"`
	Reasoning  *llmsdk.ReasoningOptions     `json:"reasoning,omitempty"`
	Modalities []llmsdk.Modality            `json:"modalities,omitempty"`
}

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

func getModelList() ([]ModelInfo, error) {
	// Get the current working directory and find the models.json file
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Look for website/models.json relative to the project root
	modelsPath := filepath.Join(cwd, "..", "..", "website", "models.json")

	// Try alternative paths if the first one doesn't exist
	if _, err := os.Stat(modelsPath); os.IsNotExist(err) {
		modelsPath = filepath.Join(cwd, "website", "models.json")
		if _, err := os.Stat(modelsPath); os.IsNotExist(err) {
			modelsPath = filepath.Join(cwd, "..", "website", "models.json")
			if _, err := os.Stat(modelsPath); os.IsNotExist(err) {
				return nil, fmt.Errorf("models.json file not found")
			}
		}
	}

	file, err := os.Open(modelsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var modelList []ModelInfo
	if err := json.Unmarshal(data, &modelList); err != nil {
		return nil, err
	}

	return modelList, nil
}
