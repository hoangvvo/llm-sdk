package main

import (
	"context"
	"fmt"
	"time"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

var instructions = []llmagent.InstructionParam[*MyContext]{
    {String: ptr(`Answer in markdown format.
To access certain tools, the user may have to provide corresponding API keys in the context fields on the UI.`)},
	{Func: func(ctx context.Context, context *MyContext) (string, error) {
		name := "<not provided>"
		if context.Name != nil {
			name = *context.Name
		}
		location := "<not provided>"
		if context.Location != nil {
			location = *context.Location
		}
		language := "<not provided>"
		if context.Language != nil {
			language = *context.Language
		}
		return fmt.Sprintf(`The user name is %s.
The user location is %s.
The user speaks %s language.`, name, location, language), nil
	}},
    {Func: func(ctx context.Context, context *MyContext) (string, error) {
        return fmt.Sprintf("The current date is %s.", time.Now().Format("Mon Jan 2 2006")), nil
    }},
    {String: ptr(`For substantive deliverables (documents/specs/code), use the artifact tools (artifact_create, artifact_update, artifact_get, artifact_list, artifact_delete).
Keep chat replies brief and put the full document content into artifacts via these tools, rather than pasting large content into chat. Reference documents by their id.`)},
}

var availableTools = []llmagent.AgentTool[*MyContext]{
    &GetStockPriceTool{},
    &GetCryptoPriceTool{},
    &SearchWikipediaTool{},
    &GetNewsTool{},
    &GetCoordinatesTool{},
    &GetWeatherTool{},
    &ArtifactCreateTool{},
    &ArtifactUpdateTool{},
    &ArtifactGetTool{},
    &ArtifactListTool{},
    &ArtifactDeleteTool{},
}

type AgentOptions struct {
	EnabledTools         []string
	DisabledInstructions bool
	Temperature          *float64
	TopP                 *float64
	TopK                 *int
	FrequencyPenalty     *float64
	PresencePenalty      *float64
	Audio                *llmsdk.AudioOptions
	Reasoning            *llmsdk.ReasoningOptions
	Modalities           []llmsdk.Modality
}

func createAgent(model llmsdk.LanguageModel, modelInfo *ModelInfo, options *AgentOptions) *llmagent.Agent[*MyContext] {
	var tools []llmagent.AgentTool[*MyContext]
	if len(options.EnabledTools) > 0 {
		toolNameSet := make(map[string]bool)
		for _, name := range options.EnabledTools {
			toolNameSet[name] = true
		}
		for _, tool := range availableTools {
			if toolNameSet[tool.Name()] {
				tools = append(tools, tool)
			}
		}
	} else {
		tools = availableTools
	}

	var agentInstructions []llmagent.InstructionParam[*MyContext]
	if !options.DisabledInstructions {
		agentInstructions = instructions
	}

	opts := []llmagent.AgentParamsOption[*MyContext]{
		llmagent.WithInstructions(agentInstructions...),
		llmagent.WithTools(tools...),
		llmagent.WithMaxTurns[*MyContext](5),
	}

	if options.Temperature != nil {
		opts = append(opts, llmagent.WithTemperature[*MyContext](*options.Temperature))
	}
	if options.TopP != nil {
		opts = append(opts, llmagent.WithTopP[*MyContext](*options.TopP))
	}
	if options.TopK != nil {
		opts = append(opts, llmagent.WithTopK[*MyContext](int32(*options.TopK)))
	}
	if options.FrequencyPenalty != nil {
		opts = append(opts, llmagent.WithFrequencyPenalty[*MyContext](*options.FrequencyPenalty))
	}
	if options.PresencePenalty != nil {
		opts = append(opts, llmagent.WithPresencePenalty[*MyContext](*options.PresencePenalty))
	}
	if options.Reasoning != nil {
		opts = append(opts, llmagent.WithReasoning[*MyContext](*options.Reasoning))
	}
	if options.Audio != nil {
		opts = append(opts, llmagent.WithAudio[*MyContext](*options.Audio))
	}
	if modelInfo.Modalities != nil {
		opts = append(opts, llmagent.WithModalities[*MyContext](modelInfo.Modalities...))
	}

	return llmagent.NewAgent("MyAgent", model, opts...)
}

func ptr[T any](v T) *T {
	return &v
}
