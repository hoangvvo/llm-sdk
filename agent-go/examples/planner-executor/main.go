package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
)

// Internal TODO model and store
type Todo struct {
	Status string `json:"status"`
	Step   string `json:"step"`
}

type Store struct {
	m           map[string]*Todo
	explanation string
}

func NewStore() *Store { return &Store{m: map[string]*Todo{}, explanation: ""} }
func (s *Store) List() []*Todo {
	out := make([]*Todo, 0, len(s.m))
	for _, t := range s.m {
		out = append(out, t)
	}
	return out
}
func (s *Store) Explanation() string { return s.explanation }
func (s *Store) ResetWith(plan []Todo, explanation string) {
	s.m = map[string]*Todo{}
	for i := range plan {
		it := plan[i]
		key := fmt.Sprintf("%d", i)
		itCopy := it
		s.m[key] = &itCopy
	}
	s.explanation = explanation
}

func formatTodos(s *Store) string {
	list := s.List()
	var b strings.Builder
	fmt.Fprintf(&b, "\n─ PLAN (internal) · %d items\n", len(list))
	if s.Explanation() != "" {
		fmt.Fprintf(&b, "Explanation: %s\n", s.Explanation())
	}
	if len(list) == 0 {
		b.WriteString("(empty)\n")
		return b.String()
	}
	for _, t := range list {
		sym := "○"
		if strings.EqualFold(strings.TrimSpace(t.Status), "in_progress") {
			sym = "▸"
		}
		if strings.EqualFold(strings.TrimSpace(t.Status), "complete") {
			sym = "✓"
		}
		fmt.Fprintf(&b, "%s %s\n", sym, t.Step)
	}
	return b.String()
}

func clearAndRender(messages []string, s *Store) {
	// Clear console
	// Prefer console clear; fallback to ANSI
	fmt.Print("\033[2J\033[H")
	if len(messages) > 0 {
		fmt.Println(strings.Join(messages, "\n\n"))
		fmt.Println()
	}
	fmt.Print(formatTodos(s))
}

// No context for this example
type Ctx = struct{}

// Tools
// Single tool: update_plan
type UpdatePlanTool struct{ S *Store }

func (t *UpdatePlanTool) Name() string { return "update_plan" }
func (t *UpdatePlanTool) Description() string {
	return "Replace internal plan with explanation and steps"
}
func (t *UpdatePlanTool) Parameters() llmsdk.JSONSchema {
	// Strict schema: all properties required, no additional
	item := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"status": map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "complete"}},
			"step":   map[string]any{"type": "string"},
		},
		"required":             []string{"status", "step"},
		"additionalProperties": false,
	}
	m := llmsdk.JSONSchema{"type": "object"}
	m["properties"] = map[string]any{
		"explanation": map[string]any{"type": "string"},
		"plan":        map[string]any{"type": "array", "items": item},
	}
	m["required"] = []string{"explanation", "plan"}
	m["additionalProperties"] = false
	return m
}
func (t *UpdatePlanTool) Execute(_ context.Context, params json.RawMessage, _ Ctx, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var p struct {
		Explanation string `json:"explanation"`
		Plan        []Todo `json:"plan"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return llmagent.AgentToolResult{}, err
	}
	t.S.ResetWith(p.Plan, p.Explanation)
	body, _ := json.Marshal(map[string]any{"ok": true, "explanation": p.Explanation, "plan": t.S.List()})
	return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart(string(body))}, IsError: false}, nil
}

func main() {
	godotenv.Load("../.env")
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		panic("OPENAI_API_KEY must be set")
	}

	model := openai.NewOpenAIModel("gpt-4o", openai.OpenAIModelOptions{APIKey: apiKey})

	store := NewStore()

	// Build agent
	overview := `You are a planner–executor assistant.
Break the user's goal into clear, actionable steps using the tool update_plan (explanation, plan: [{status, step}]).
Use the plan strictly as your internal plan: NEVER reveal or enumerate plan items to the user. Do not mention the words TODO, task list, or the names of tools.
Keep user-visible replies concise and focused on results and next-step confirmations.
Work iteratively: plan an initial set of high-level steps, then refine/execute one major step per turn, marking completed items along the way via tools.
When the work is complete, respond with the final deliverable and a brief one-paragraph summary of what you did.`

	agent := llmagent.NewAgent("planner-executor", model,
		llmagent.WithInstructions(
			llmagent.InstructionParam[Ctx]{String: &overview},
			// Dynamic instruction: inject internal plan
			llmagent.InstructionParam[Ctx]{Func: func(_ context.Context, _ Ctx) (string, error) {
				var b strings.Builder
				b.WriteString("INTERNAL PLAN:\n")
				list := store.List()
				for i, it := range list {
					fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, it.Status, it.Step)
				}
				if store.Explanation() != "" {
					fmt.Fprintf(&b, "Explanation: %s\n", store.Explanation())
				}
				return b.String(), nil
			}},
		),
		llmagent.WithTools(&UpdatePlanTool{S: store}),
		llmagent.WithMaxTurns[Ctx](20),
	)

	// Conversation
	items := []llmagent.AgentItem{
		llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart(
			"You are hired to produce a concise PRD (Product Requirements Document) for a travel booking app. " +
				"Do high-level planning and execution across turns: outline the PRD structure, then draft sections " +
				"(Overview, Target Users, Core Features, MVP Scope, Non-Goals, Success Metrics, Risks), and finally " +
				"produce the final PRD in markdown. Keep replies brief and focused on progress/results only.",
		))),
	}

	var messages []string
	clearAndRender(messages, store)

	ctx := context.Background()
	for turn := 1; ; turn++ {
		res, err := agent.Run(ctx, llmagent.AgentRequest[Ctx]{Context: Ctx{}, Input: items})
		if err != nil {
			panic(err)
		}
		// Append assistant-visible text
		var visible []string
		for _, p := range res.Content {
			if p.TextPart != nil {
				visible = append(visible, p.TextPart.Text)
			}
		}
		if len(visible) > 0 {
			messages = append(messages, strings.TrimSpace(strings.Join(visible, "\n")))
		}
		clearAndRender(messages, store)

		// Append output
		items = append(items, res.Output...)

		list := store.List()
		allDone := len(list) > 0
		for _, t := range list {
			allDone = allDone && strings.EqualFold(strings.TrimSpace(t.Status), "DONE")
		}
		if allDone {
			break
		}

		items = append(items, llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("NEXT"))))
	}

	clearAndRender(messages, store)
}
