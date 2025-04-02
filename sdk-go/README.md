# llm-sdk for Go

A Go library that provides a unified API to access the LLM APIs of various providers.

## Usage

All models implement the `LanguageModel` interface:

```go
package examples

import (
	"fmt"
	"os"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
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
```

Below is an example to generate text:

```go
package main

import (
	"context"
	"log"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
	"github.com/sanity-io/litter"
)

func main() {
	model := examples.GetModel("openai", "gpt-4o")

	response, err := model.Generate(context.Background(), &llmsdk.LanguageModelInput{
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("Tell me a story."),
			),
			llmsdk.NewAssistantMessage(
				llmsdk.NewTextPart("What kind of story would you like to hear?"),
			),
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("A fairy tale."),
			),
		},
	})

	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	litter.Dump(response)
}
```

Find examples in the [examples](./examples/) folder to learn how to:

- [`generate-text`: Generate text](./examples/generate-text/main.go)
- [`stream-text`: Stream text](./examples/stream-text/main.go)
- [`generate-audio`: Generate audio](./examples/generate-audio/main.go)
- [`stream-audio`: Stream audio](./examples/stream-audio/main.go)
- [`describe-image`: Describe image](./examples/describe-image/main.go)
- [`summarize-audio`: Summarize audio](./examples/summarize-audio/main.go)
- [`tool-use`: Function calling](./examples/tool-use/main.go)
- [`structured-output`: Structured output](./examples/structured-output/main.go)
- [`generate-reasoning`: Reasoning](./examples/generate-reasoning/main.go)

```bash
go run ./examples/generate-text
```
