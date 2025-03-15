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
```

Below is an example to generate text:

```go
package main

import (
	"context"
	"fmt"
	"log"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
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

	fmt.Println(examples.ToJSONString(response))
}
```

Find examples in the [examples](./examples/) folder to learn how to:

- [`generate-text`: Generate text](./examples/generate-text/main.go)
- [`stream-text`: Stream text](./examples/stream-text/main.go)
- [`describe-image`: Describe image](./examples/describe-image/main.go)
- [`tool-use`: Function calling](./examples/tool-use/main.go)
- [`generate-audio`: Generate audio](./examples/generate-audio/main.go)
- [`stream-audio`: Stream audio](./examples/stream-audio/main.go)

```bash
go run ./examples/generate-text
```
