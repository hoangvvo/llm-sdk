package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
)

// LostAndFoundContext mirrors the TypeScript example: agent tools mutate the manifest
// directly so the RunSession sees the latest state without requiring a toolkit.
type LostAndFoundContext struct {
	ManifestID        string
	Archivist         string
	IntakeLedger      map[string]ItemRecord
	FlaggedContraband map[string]struct{}
	ReceiptNotes      []string
}

type ItemRecord struct {
	Description string
	Priority    string
}

func newContext() *LostAndFoundContext {
	return &LostAndFoundContext{
		ManifestID:        "aurora-shift",
		Archivist:         "Quill",
		IntakeLedger:      map[string]ItemRecord{},
		FlaggedContraband: map[string]struct{}{},
		ReceiptNotes:      []string{},
	}
}

// intakeItemTool shows a standard AgentTool implementation that validates input,
// mutates context, and returns structured output.
type intakeItemTool struct{}

func (intakeItemTool) Name() string        { return "intake_item" }
func (intakeItemTool) Description() string { return "Register an item reported by the traveller." }

func (intakeItemTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"item_id": map[string]any{
				"type":        "string",
				"description": "Identifier used on the manifest ledger.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "What the traveller says it looks like.",
			},
			"priority": map[string]any{
				"type": "string",
				"enum": []string{"standard", "rush"},
			},
		},
		"required":             []string{"item_id", "description"},
		"additionalProperties": false,
	}
}

func (intakeItemTool) Execute(_ context.Context, raw json.RawMessage, ctx *LostAndFoundContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params struct {
		ItemID      string `json:"item_id"`
		Description string `json:"description"`
		Priority    string `json:"priority"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	normalized := strings.ToLower(strings.TrimSpace(params.ItemID))
	if normalized == "" {
		return llmagent.AgentToolResult{}, errors.New("item_id cannot be empty")
	}
	if _, exists := ctx.IntakeLedger[normalized]; exists {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{llmsdk.NewTextPart(fmt.Sprintf("Item %s is already on the ledger—confirm the manifest number before adding duplicates.", params.ItemID))},
			IsError: true,
		}, nil
	}

	priority := params.Priority
	if priority == "" {
		priority = "standard"
	}

	ctx.IntakeLedger[normalized] = ItemRecord{Description: params.Description, Priority: priority}
	ctx.ReceiptNotes = append(ctx.ReceiptNotes, fmt.Sprintf("%s: %s%s", params.ItemID, params.Description, ternary(priority == "rush", " (rush intake)", "")))

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(fmt.Sprintf("Logged %s as %s. Intake queue now holds %d item(s).", params.Description, params.ItemID, len(ctx.IntakeLedger)))},
		IsError: false,
	}, nil
}

// flagContrabandTool showcases additional validation and context mutation.
type flagContrabandTool struct{}

func (flagContrabandTool) Name() string { return "flag_contraband" }
func (flagContrabandTool) Description() string {
	return "Escalate a manifest item for contraband review."
}

func (flagContrabandTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"item_id": map[string]any{
				"type":        "string",
				"description": "Identifier within the manifest.",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "Why the item needs review.",
			},
		},
		"required":             []string{"item_id", "reason"},
		"additionalProperties": false,
	}
}

func (flagContrabandTool) Execute(_ context.Context, raw json.RawMessage, ctx *LostAndFoundContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params struct {
		ItemID string `json:"item_id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	normalized := strings.ToLower(strings.TrimSpace(params.ItemID))
	if _, exists := ctx.IntakeLedger[normalized]; !exists {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{llmsdk.NewTextPart(fmt.Sprintf("Cannot flag %s; it has not been logged yet. Intake the item first.", params.ItemID))},
			IsError: true,
		}, nil
	}

	ctx.FlaggedContraband[normalized] = struct{}{}
	ctx.ReceiptNotes = append(ctx.ReceiptNotes, fmt.Sprintf("⚠️ %s held for review: %s", params.ItemID, params.Reason))

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(fmt.Sprintf("%s marked for contraband inspection. Inform security before release.", params.ItemID))},
		IsError: false,
	}, nil
}

// issueReceiptTool highlights summarising context state and clearing it afterwards.
type issueReceiptTool struct{}

func (issueReceiptTool) Name() string { return "issue_receipt" }
func (issueReceiptTool) Description() string {
	return "Publish a receipt and clear the manifest ledger."
}

func (issueReceiptTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"traveller": map[string]any{
				"type":        "string",
				"description": "Recipient of the receipt.",
			},
		},
		"required":             []string{"traveller"},
		"additionalProperties": false,
	}
}

func (issueReceiptTool) Execute(_ context.Context, raw json.RawMessage, ctx *LostAndFoundContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params struct {
		Traveller string `json:"traveller"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	if len(ctx.IntakeLedger) == 0 {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{llmsdk.NewTextPart(fmt.Sprintf("No items pending on manifest %s. Intake something before issuing a receipt.", ctx.ManifestID))},
			IsError: true,
		}, nil
	}

	cleared := []string{}
	for id, record := range ctx.IntakeLedger {
		if _, flagged := ctx.FlaggedContraband[id]; !flagged {
			cleared = append(cleared, fmt.Sprintf("%s (%s)", id, record.Description))
		}
	}

	summary := []string{
		fmt.Sprintf("Receipt for %s on manifest %s:", params.Traveller, ctx.ManifestID),
	}
	if len(cleared) > 0 {
		summary = append(summary, fmt.Sprintf("Cleared items: %s", strings.Join(cleared, ", ")))
	} else {
		summary = append(summary, "No items cleared—everything is held for review.")
	}
	if len(ctx.ReceiptNotes) > 0 {
		summary = append(summary, "Notes:")
		summary = append(summary, ctx.ReceiptNotes...)
	}
	summary = append(summary, fmt.Sprintf("%d item(s) require contraband follow-up.", len(ctx.FlaggedContraband)))

	// Clear state so subsequent turns start fresh.
	ctx.IntakeLedger = map[string]ItemRecord{}
	ctx.FlaggedContraband = map[string]struct{}{}
	ctx.ReceiptNotes = ctx.ReceiptNotes[:0]

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(strings.Join(summary, "\n"))},
		IsError: false,
	}, nil
}

func main() {
	if err := godotenv.Load("../.env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("load env: %v", err)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable must be set")
	}

	model := openai.NewOpenAIModel("gpt-4o", openai.OpenAIModelOptions{APIKey: apiKey})

	agent := llmagent.NewAgent[*LostAndFoundContext](
		"WaypointClerk",
		model,
		llmagent.WithInstructions(
			llmagent.InstructionParam[*LostAndFoundContext]{String: ptr("You are the archivist completing intake for Waypoint Seven's Interdimensional Lost & Found desk.")},
			llmagent.InstructionParam[*LostAndFoundContext]{String: ptr("When travellers report belongings, call the available tools to mutate the manifest and then summarise your actions.")},
			llmagent.InstructionParam[*LostAndFoundContext]{String: ptr("If a tool reports an error, acknowledge the issue and guide the traveller appropriately.")},
		),
		llmagent.WithTools(intakeItemTool{}, flagContrabandTool{}, issueReceiptTool{}),
	)

	// Successful run exercises multiple tools in a single turn.
	successCtx := newContext()
	successResp, err := agent.Run(context.Background(), llmagent.AgentRequest[*LostAndFoundContext]{
		Context: successCtx,
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart("Log the Chrono Locket as rush, flag the Folded star chart for contraband, then issue a receipt for Captain Lyra Moreno."),
				),
			),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n=== SUCCESS RUN ===")
	fmt.Printf("%#v\n", successResp)
	fmt.Println(successResp.Text())

	// Failure run demonstrates a tool error path.
	failureCtx := newContext()
	failureResp, err := agent.Run(context.Background(), llmagent.AgentRequest[*LostAndFoundContext]{
		Context: failureCtx,
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(
				llmsdk.NewUserMessage(
					llmsdk.NewTextPart("Issue a receipt immediately without logging anything."),
				),
			),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n=== FAILURE RUN ===")
	fmt.Printf("%#v\n", failureResp)
	fmt.Println(failureResp.Text())
}

func ptr[T any](v T) *T { return &v }

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
