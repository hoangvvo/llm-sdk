package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	"github.com/hoangvvo/llm-sdk/agent-go/examples"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/joho/godotenv"
)

type DungeonRunContext struct {
	DungeonMaster        string
	PartyName            string
	CurrentQuest         string
	HighlightPlayerClass string
	OracleHint           string
}

func (c *DungeonRunContext) GetOracleWhisper(ctx context.Context) (string, error) {
	select {
	case <-time.After(25 * time.Millisecond):
		return c.OracleHint, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func main() {
	godotenv.Load("../.env")

	provider := os.Getenv("PROVIDER")
	if provider == "" {
		provider = "openai"
	}
	modelID := os.Getenv("MODEL")
	if modelID == "" {
		modelID = "gpt-5.6-terra"
	}
	model, err := examples.GetModel(provider, modelID, llmsdk.LanguageModelMetadata{}, "")
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	staticInstruction := "You are Torch, a supportive guide who keeps tabletop role-playing sessions moving. Offer concrete options instead of long monologues."

	dynamicInstruction := func(ctx context.Context, ctxVal *DungeonRunContext) (string, error) {
		return fmt.Sprintf(
			"You are helping %s, the Dungeon Master for the %s. They are running the quest \"%s\" and need a quick nudge that favors the party's %s.",
			ctxVal.DungeonMaster,
			ctxVal.PartyName,
			ctxVal.CurrentQuest,
			ctxVal.HighlightPlayerClass,
		), nil
	}

	asyncInstruction := func(ctx context.Context, ctxVal *DungeonRunContext) (string, error) {
		whisper, err := ctxVal.GetOracleWhisper(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Weave in the oracle whisper: \"%s\" so it feels like an in-world hint.", whisper), nil
	}

	dungeonCoach := llmagent.NewAgent("Torch", model,
		llmagent.WithInstructions(
			llmagent.InstructionParam[*DungeonRunContext]{String: &staticInstruction},
			llmagent.InstructionParam[*DungeonRunContext]{Func: dynamicInstruction},
			llmagent.InstructionParam[*DungeonRunContext]{Func: asyncInstruction},
		),
	)

	ctx := &DungeonRunContext{
		DungeonMaster:        "Rowan",
		PartyName:            "Lanternbearers",
		CurrentQuest:         "Echoes of the Sunken Keep",
		HighlightPlayerClass: "ranger",
		OracleHint:           "the moss remembers every secret step",
	}

	prompt := "The party is stuck at a collapsed bridge. What should happen next?"

	response, err := dungeonCoach.Run(context.Background(), llmagent.AgentRequest[*DungeonRunContext]{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart(prompt),
				),
			),
		},
		Context: ctx,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(response.Text())
}
