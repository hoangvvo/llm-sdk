package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
	"github.com/sanity-io/litter"
)

type EnemyState struct {
	HP int `json:"hp"`
}

type EncounterState struct {
	Scene        string
	Enemies      map[string]EnemyState
	DownedAllies map[string]struct{}
}

type DungeonRunContext struct {
	DungeonMaster string
	PartyName     string
	Encounter     *EncounterState
	ActionBudget  map[string]int
}

func createDungeonContext() *DungeonRunContext {
	return &DungeonRunContext{
		DungeonMaster: "Rowan",
		PartyName:     "Lanternbearers",
		Encounter: &EncounterState{
			Scene: "The Echo Bridge over the Sunken Keep",
			Enemies: map[string]EnemyState{
				"ghoul":    {HP: 12},
				"marauder": {HP: 9},
			},
			DownedAllies: map[string]struct{}{
				"finley": {},
			},
		},
		ActionBudget: map[string]int{
			"thorne": 1,
			"mira":   2,
		},
	}
}

type AttackEnemyParams struct {
	Attacker string `json:"attacker"`
	Target   string `json:"target"`
	Weapon   string `json:"weapon"`
}

type AttackEnemyTool struct{}

func (t *AttackEnemyTool) Name() string {
	return "attack_enemy"
}

func (t *AttackEnemyTool) Description() string {
	return "Resolve a martial attack from a party member against an active enemy and update its hit points."
}

func (t *AttackEnemyTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"attacker": map[string]any{
				"type":        "string",
				"description": "Name of the party member making the attack.",
			},
			"target": map[string]any{
				"type":        "string",
				"description": "Enemy to strike.",
			},
			"weapon": map[string]any{
				"type":        "string",
				"description": "Weapon or maneuver used for flavour and damage bias.",
			},
		},
		"required":             []string{"attacker", "target", "weapon"},
		"additionalProperties": false,
	}
}

func (t *AttackEnemyTool) Execute(_ context.Context, paramsJSON json.RawMessage, contextVal *DungeonRunContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params AttackEnemyParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	attackerKey := strings.ToLower(strings.TrimSpace(params.Attacker))
	targetKey := strings.ToLower(strings.TrimSpace(params.Target))

	remainingActions := contextVal.ActionBudget[attackerKey]
	if remainingActions <= 0 {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("%s is out of actions this round. Ask another hero to step in or advance the scene.", params.Attacker)),
			},
			IsError: true,
		}, nil
	}

	enemy, ok := contextVal.Encounter.Enemies[targetKey]
	if !ok {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("No enemy named %s remains at %s. Double-check the initiative order.", params.Target, contextVal.Encounter.Scene)),
			},
			IsError: true,
		}, nil
	}

	weaponLower := strings.ToLower(params.Weapon)
	baseDamage := 5
	if strings.Contains(weaponLower, "axe") {
		baseDamage = 7
	}
	finesseBonus := 0
	if strings.Contains(weaponLower, "dagger") {
		finesseBonus = 1
	}
	computedDamage := baseDamage + (len(params.Attacker) % 3) + finesseBonus

	enemy.HP = enemy.HP - computedDamage
	if enemy.HP < 0 {
		enemy.HP = 0
	}
	contextVal.Encounter.Enemies[targetKey] = enemy
	contextVal.ActionBudget[attackerKey] = remainingActions - 1

	defeated := ""
	if enemy.HP == 0 {
		defeated = fmt.Sprintf(" %s is defeated!", params.Target)
	}

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(fmt.Sprintf("%s hits %s for %d damage with the %s. %s now has %d HP.%s",
				params.Attacker, params.Target, computedDamage, params.Weapon, params.Target, enemy.HP, defeated)),
		},
		IsError: false,
	}, nil
}

type StabilizeAllyParams struct {
	Hero string `json:"hero"`
}

type StabilizeAllyTool struct{}

func (t *StabilizeAllyTool) Name() string {
	return "stabilize_ally"
}

func (t *StabilizeAllyTool) Description() string {
	return "Spend the round stabilising a downed ally. Removes them from the downed list if available."
}

func (t *StabilizeAllyTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"hero": map[string]any{
				"type":        "string",
				"description": "Name of the ally to stabilise.",
			},
		},
		"required":             []string{"hero"},
		"additionalProperties": false,
	}
}

func (t *StabilizeAllyTool) Execute(_ context.Context, paramsJSON json.RawMessage, contextVal *DungeonRunContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params StabilizeAllyParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	heroKey := strings.ToLower(strings.TrimSpace(params.Hero))
	if _, ok := contextVal.Encounter.DownedAllies[heroKey]; !ok {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("%s is already on their feet. Consider taking another tactical action instead of stabilising.", params.Hero)),
			},
			IsError: true,
		}, nil
	}

	delete(contextVal.Encounter.DownedAllies, heroKey)

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(fmt.Sprintf("%s is stabilised and ready to rejoin when the next round begins.", params.Hero)),
		},
		IsError: false,
	}, nil
}

func listDownedAllies(m map[string]struct{}) []string {
	allies := make([]string, 0, len(m))
	for k := range m {
		allies = append(allies, k)
	}
	return allies
}

func main() {
	godotenv.Load("../.env")

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable must be set")
	}

	model := openai.NewOpenAIModel("gpt-4o", openai.OpenAIModelOptions{APIKey: apiKey})

	dungeonCoach := llmagent.NewAgent[*DungeonRunContext]("Torch", model,
		llmagent.WithInstructions(
			llmagent.InstructionParam[*DungeonRunContext]{String: ptr(
				"You are Torch, a steady co-Dungeon Master. Keep answers short and, when combat actions come up, lean on the provided tools to resolve them.",
			)},
			llmagent.InstructionParam[*DungeonRunContext]{String: ptr(
				"If a requested action involves striking an enemy, call attack_enemy. If the party wants to help someone back up, call stabilize_ally before answering.",
			)},
		),
		llmagent.WithTools(&AttackEnemyTool{}, &StabilizeAllyTool{}),
	)

	successContext := createDungeonContext()
	successResponse, err := dungeonCoach.Run(context.Background(), llmagent.AgentRequest[*DungeonRunContext]{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart("Thorne will strike the ghoul with a battleaxe while Mira uses her turn to stabilise Finley. Help me resolve it."),
				),
			),
		},
		Context: successContext,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Success response:")
	litter.Dump(successResponse)
	fmt.Printf("Remaining enemy HP: %#v\n", successContext.Encounter.Enemies)
	fmt.Printf("Downed allies after success run: %#v\n", listDownedAllies(successContext.Encounter.DownedAllies))

	failureContext := createDungeonContext()
	failureContext.ActionBudget["thorne"] = 0
	failureContext.Encounter.DownedAllies = map[string]struct{}{}

	failureResponse, err := dungeonCoach.Run(context.Background(), llmagent.AgentRequest[*DungeonRunContext]{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart("Thorne wants to swing again at the marauder, and Mira tries to stabilise Finley anyway."),
				),
			),
		},
		Context: failureContext,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Failure response:")
	litter.Dump(failureResponse)
}

func ptr[T any](v T) *T {
	return &v
}
