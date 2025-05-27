package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
)

type matchID string

type weather string

type moveKind string

const (
	matchSunShowdown matchID = "sun-showdown"
	matchDreamDusk   matchID = "dream-dusk"

	weatherHarshSunlight weather = "harsh_sunlight"
	weatherNone          weather = "none"
)

const (
	moveFlamethrower moveKind = "flamethrower"
	moveSolarBeam    moveKind = "solar_beam"
	moveAirSlash     moveKind = "air_slash"
	moveShadowBall   moveKind = "shadow_ball"
	moveHypnosis     moveKind = "hypnosis"
	moveDreamEater   moveKind = "dream_eater"
	moveNightmare    moveKind = "nightmare"
)

type battleContext struct {
	MatchID matchID
}

type pokemonState struct {
	Name    string
	Ability string
	Item    *string
	Moves   []moveKind
}

type opponentState struct {
	Name   string
	Typing []string
	Status string
	Hint   string
}

type battleState struct {
	Weather   weather
	Arena     string
	CrowdNote string
	Pokemon   pokemonState
	Opponent  opponentState
}

type moveSpec struct {
	kind        moveKind
	description string
	available   func(*battleState) bool
	execute     func(*battleState, string) string
}

type moveTool struct {
	spec  moveSpec
	state *battleState
}

func (m *moveTool) Name() string { return string(m.spec.kind) }

func (m *moveTool) Description() string { return m.spec.description }

func (m *moveTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"target": map[string]any{
				"type":        "string",
				"description": "Optional target override; defaults to the opposing Pokémon.",
			},
		},
		"required":             []string{},
		"additionalProperties": false,
	}
}

func (m *moveTool) Execute(_ context.Context, params json.RawMessage, _ *battleContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var payload struct {
		Target string `json:"target"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &payload)
	}
	target := payload.Target
	if target == "" {
		target = m.state.Opponent.Name
	}

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(m.spec.execute(m.state, target))},
		IsError: false,
	}, nil
}

// useItemTool and attemptEscapeTool stay static to show base capability the agent always has.
type useItemTool struct{}
type attemptEscapeTool struct{}

func (useItemTool) Name() string        { return "use_item" }
func (useItemTool) Description() string { return "Use a held or bag item to swing the battle." }
func (attemptEscapeTool) Name() string  { return "attempt_escape" }
func (attemptEscapeTool) Description() string {
	return "Attempt to flee if the battle is unwinnable."
}

func (useItemTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"item": map[string]any{
				"type":        "string",
				"description": "Name of the item to use.",
			},
		},
		"required":             []string{"item"},
		"additionalProperties": false,
	}
}

func (attemptEscapeTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type":                 "object",
		"properties":           map[string]any{},
		"required":             []string{},
		"additionalProperties": false,
	}
}

func (useItemTool) Execute(_ context.Context, params json.RawMessage, _ *battleContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var payload struct {
		Item string `json:"item"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return llmagent.AgentToolResult{}, err
	}
	if payload.Item == "" {
		return llmagent.AgentToolResult{}, errors.New("item is required")
	}

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(fmt.Sprintf("I use the %s to shift momentum.", payload.Item))},
		IsError: false,
	}, nil
}

func (attemptEscapeTool) Execute(_ context.Context, _ json.RawMessage, _ *battleContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart("I search for an opening to retreat from the field.")},
		IsError: false,
	}, nil
}

var battles = map[matchID]battleState{
	matchSunShowdown: {
		Weather:   weatherHarshSunlight,
		Arena:     "Pyrite Crater",
		CrowdNote: "Crowd roars when attacks lean into the blazing sun.",
		Pokemon: pokemonState{
			Name:    "Charizard",
			Ability: "Solar Power",
			Item:    ptr("Choice Specs"),
			Moves:   []moveKind{moveFlamethrower, moveSolarBeam, moveAirSlash},
		},
		Opponent: opponentState{
			Name:   "Ferrothorn",
			Typing: []string{"Grass", "Steel"},
			Status: "healthy",
			Hint:   "likes to turtle behind Leech Seed and Iron Defense.",
		},
	},
	matchDreamDusk: {
		Weather:   weatherNone,
		Arena:     "Midnight Colosseum",
		CrowdNote: "Spectators fall silent for sinister dream tactics.",
		Pokemon: pokemonState{
			Name:    "Haunter",
			Ability: "Levitate",
			Item:    ptr("Wide Lens"),
			Moves:   []moveKind{moveShadowBall, moveHypnosis, moveDreamEater, moveNightmare},
		},
		Opponent: opponentState{
			Name:   "Gardevoir",
			Typing: []string{"Psychic", "Fairy"},
			Status: "asleep",
			Hint:   "was stacking Calm Mind boosts before dozing off.",
		},
	},
}

func ptr[T any](v T) *T { return &v }

func loadBattle(match matchID) (*battleState, error) {
	state, ok := battles[match]
	if !ok {
		return nil, fmt.Errorf("unknown match %s", match)
	}
	// Toolkits run async operations inside CreateSession, so sleep to mimic fetch latency.
	time.Sleep(25 * time.Millisecond)
	clone := state
	return &clone, nil
}

var moveLibrary = map[moveKind]moveSpec{
	moveFlamethrower: {
		kind:        moveFlamethrower,
		description: "Flamethrower is a dependable Fire-type strike that thrives in Harsh Sunlight.",
		execute: func(state *battleState, target string) string {
			bonus := ""
			if state.Weather == weatherHarshSunlight {
				bonus = ", the sunlight turning the flames white-hot"
			}
			return fmt.Sprintf("I scorch %s with Flamethrower%s.", target, bonus)
		},
	},
	moveSolarBeam: {
		kind:        moveSolarBeam,
		description: "Solar Beam normally charges, but fires instantly in Harsh Sunlight.",
		available:   func(state *battleState) bool { return state.Weather == weatherHarshSunlight },
		execute: func(_ *battleState, target string) string {
			return fmt.Sprintf("I gather sunlight and unleash Solar Beam on %s without needing to charge.", target)
		},
	},
	moveAirSlash: {
		kind:        moveAirSlash,
		description: "Air Slash provides Flying coverage with a flinch chance against slower foes.",
		execute: func(state *battleState, target string) string {
			return fmt.Sprintf("I ride the thermals around %s and carve %s with Air Slash.", state.Arena, target)
		},
	},
	moveShadowBall: {
		kind:        moveShadowBall,
		description: "Shadow Ball is Haunter's safest Ghost attack versus Psychic targets.",
		execute: func(state *battleState, target string) string {
			return fmt.Sprintf("I hurl Shadow Ball at %s, disrupting their %s defenses.", target, strings.Join(state.Opponent.Typing, "/"))
		},
	},
	moveHypnosis: {
		kind:        moveHypnosis,
		description: "Hypnosis can return the opponent to sleep if they stir.",
		execute: func(_ *battleState, target string) string {
			return fmt.Sprintf("I sway and cast Hypnosis toward %s, setting up dream tactics.", target)
		},
	},
	moveDreamEater: {
		kind:        moveDreamEater,
		description: "Dream Eater only works while the opponent sleeps, draining them and healing me.",
		available:   func(state *battleState) bool { return state.Opponent.Status == "asleep" },
		execute: func(state *battleState, target string) string {
			return fmt.Sprintf("I feast on %s's dreams, restoring power to %s.", target, state.Pokemon.Name)
		},
	},
	moveNightmare: {
		kind:        moveNightmare,
		description: "Nightmare curses a sleeping foe to lose HP at the end of each turn.",
		available:   func(state *battleState) bool { return state.Opponent.Status == "asleep" },
		execute: func(_ *battleState, target string) string {
			return fmt.Sprintf("I lace %s's dreams with Nightmare so they suffer each turn while asleep.", target)
		},
	},
}

func buildMoveTools(state *battleState) []llmagent.AgentTool[*battleContext] {
	result := make([]llmagent.AgentTool[*battleContext], 0, len(state.Pokemon.Moves))
	for _, kind := range state.Pokemon.Moves {
		spec, ok := moveLibrary[kind]
		if !ok {
			continue
		}
		if spec.available != nil && !spec.available(state) {
			continue
		}
		result = append(result, &moveTool{spec: spec, state: state})
	}
	return result
}

func buildPrompt(state *battleState) string {
	parts := []string{
		fmt.Sprintf("You are %s battling in %s.", state.Pokemon.Name, state.Arena),
	}
	switch state.Weather {
	case weatherHarshSunlight:
		parts = append(parts, "Harsh Sunlight supercharges Fire attacks, lets Solar Beam skip charging, and weakens Water coverage.")
	default:
		parts = append(parts, "There is no active weather effect.")
	}
	if state.Pokemon.Item != nil {
		parts = append(parts, fmt.Sprintf("Ability: %s, holding %s.", state.Pokemon.Ability, *state.Pokemon.Item))
	} else {
		parts = append(parts, fmt.Sprintf("Ability: %s.", state.Pokemon.Ability))
	}
	parts = append(parts, fmt.Sprintf("Opponent: %s (%s), currently %s. %s", state.Opponent.Name, strings.Join(state.Opponent.Typing, "/"), state.Opponent.Status, state.Opponent.Hint))
	parts = append(parts, fmt.Sprintf("Crowd note: %s", state.CrowdNote))
	parts = append(parts, "Call one available move tool (or use_item / attempt_escape) before finalising and explain why the field makes it sensible.")
	return strings.Join(parts, " ")
}

// battleToolkitSession caches the derived prompt/tools so ToolkitSession methods stay synchronous
// even though the data came from async work in CreateSession.
type battleToolkitSession struct {
	prompt string
	tools  []llmagent.AgentTool[*battleContext]
}

func newBattleToolkitSession(state *battleState) *battleToolkitSession {
	return &battleToolkitSession{
		prompt: buildPrompt(state),
		tools:  buildMoveTools(state),
	}
}

func (s *battleToolkitSession) SystemPrompt() *string {
	if s.prompt == "" {
		return nil
	}
	return &s.prompt
}

func (s *battleToolkitSession) Tools() []llmagent.AgentTool[*battleContext] {
	return s.tools
}

func (s *battleToolkitSession) Close(context.Context) error { return nil }

// battleToolkit runs async lookups inside CreateSession so dynamic tools and instructions are ready
// up front while the session surface remains synchronous for the agent loop.
type battleToolkit struct{}

func (battleToolkit) CreateSession(ctx context.Context, ctxVal *battleContext) (llmagent.ToolkitSession[*battleContext], error) {
	state, err := loadBattle(ctxVal.MatchID)
	if err != nil {
		return nil, err
	}
	return newBattleToolkitSession(state), nil
}

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("load env: %v", err)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	model := openai.NewOpenAIModel("gpt-4o", openai.OpenAIModelOptions{APIKey: apiKey})

	agent := llmagent.NewAgent[*battleContext](
		"Satoshi",
		model,
		llmagent.WithInstructions(
			llmagent.InstructionParam[*battleContext]{String: ptr("Speak in first person as the active Pokémon.")},
			llmagent.InstructionParam[*battleContext]{String: ptr("Always invoke exactly one tool before ending your answer, and mention how weather or status makes it sensible.")},
		),
		llmagent.WithTools(useItemTool{}, attemptEscapeTool{}),
		llmagent.WithToolkits(battleToolkit{}),
	)

	runExample := func(match matchID, prompt string) {
		fmt.Printf("\n=== %s ===\n", match)
		response, err := agent.Run(context.Background(), llmagent.AgentRequest[*battleContext]{
			Context: &battleContext{MatchID: match},
			Input: []llmagent.AgentItem{
				llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart(prompt))),
			},
		})
		if err != nil {
			log.Fatalf("run example: %v", err)
		}
		fmt.Println(response.Text())
	}

	runExample(matchSunShowdown, "Ferrothorn is hiding behind Iron Defense again—what's our play?")
	runExample(matchDreamDusk, "Gardevoir is still asleep—press the advantage before it wakes up!")
}
