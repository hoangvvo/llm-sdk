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

type visitorID string

type riftContext struct {
	VisitorID visitorID
}

type riftManifest struct {
	VisitorName          string
	OriginReality        string
	ArrivalSignature     string
	ContrabandRisk       string
	SentimentalInventory []string
	OutstandingAnomalies []string
	TurbulenceLevel      string
	CourtesyNote         string
}

// Mock datastore standing in for whatever external system createSession might query.
var riftManifests = map[visitorID]riftManifest{
	"aurora-shift": {
		VisitorName:          "Captain Lyra Moreno",
		OriginReality:        "Aurora-9 Spiral",
		ArrivalSignature:     "slipped in trailing aurora dust and a three-second echo",
		ContrabandRisk:       "elevated",
		SentimentalInventory: []string{"Chrono Locket (Timeline 12)", "Folded star chart annotated in ultraviolet"},
		OutstandingAnomalies: []string{"Glitter fog refuses to obey gravity", "Field report cites duplicate footfalls arriving 4s late"},
		TurbulenceLevel:      "moderate",
		CourtesyNote:         "Prefers dry humor, allergic to paradox puns.",
	},
	"ember-paradox": {
		VisitorName:          "Archivist Rune Tal",
		OriginReality:        "Ember Paradox Belt",
		ArrivalSignature:     "emerged in a plume of cooled obsidian and smoke",
		ContrabandRisk:       "critical",
		SentimentalInventory: []string{"Glass bead containing their brother's timeline", "A singed manifesto titled 'Do Not Fold'"},
		OutstandingAnomalies: []string{"Customs still waiting on clearance form 88-A", "Phoenix feather repeats ignition loop every two minutes"},
		TurbulenceLevel:      "volatile",
		CourtesyNote:         "Responds well to calm checklists and precise handoffs.",
	},
}

// Simulated async fetch used inside Toolkit.CreateSession to hydrate the session once up front.
func fetchRiftManifest(ctx context.Context, id visitorID) (*riftManifest, error) {
	manifest, ok := riftManifests[id]
	if !ok {
		return nil, fmt.Errorf("unknown visitor %s", id)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(60 * time.Millisecond):
	}

	clone := manifest
	return &clone, nil
}

type intakePhase string

const (
	phaseIntake   intakePhase = "intake"
	phaseRecovery intakePhase = "recovery"
	phaseHandoff  intakePhase = "handoff"
	phaseClosed   intakePhase = "closed"
)

// Toolkit session caches manifest details and evolving workflow state so each turn can
// surface new prompt/tool guidance. RunSession keeps this object alive across turns, and
// the tool implementations mutate these fields to demonstrate long-lived ToolkitSession state.
type lostAndFoundToolkitSession struct {
	manifest      *riftManifest
	phase         intakePhase
	passVerified  bool
	taggedItems   []string
	prophecyCount int
	droneDeployed bool
}

func newLostAndFoundToolkitSession(manifest *riftManifest) *lostAndFoundToolkitSession {
	return &lostAndFoundToolkitSession{
		manifest:      manifest,
		phase:         phaseIntake,
		taggedItems:   []string{},
		prophecyCount: 0,
	}
}

func (s *lostAndFoundToolkitSession) SystemPrompt() *string {
	prompt := s.buildPrompt()
	return &prompt
}

// Tools is queried before every model turn; we rebuild the list from session state so
// the agent sees newly unlocked or retired tools as phases change.
func (s *lostAndFoundToolkitSession) Tools() []llmagent.AgentTool[*riftContext] {
	tools := s.buildTools()
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name())
	}
	fmt.Printf("[Toolkit] Tools for phase %s: %s\n", strings.ToUpper(string(s.phase)), func() string {
		if len(names) == 0 {
			return "<none>"
		}
		return strings.Join(names, ", ")
	}())
	return tools
}

func (s *lostAndFoundToolkitSession) Close(context.Context) error { return nil }

func (s *lostAndFoundToolkitSession) buildPrompt() string {
	var lines []string
	lines = append(lines, "You are the Archivist manning Interdimensional Waypoint Seven's Lost & Found counter.")
	lines = append(lines, fmt.Sprintf("Visitor: %s from %s (%s).", s.manifest.VisitorName, s.manifest.OriginReality, s.manifest.ArrivalSignature))
	lines = append(lines, fmt.Sprintf("Contraband risk: %s. Turbulence: %s.", s.manifest.ContrabandRisk, s.manifest.TurbulenceLevel))

	if len(s.manifest.SentimentalInventory) > 0 {
		lines = append(lines, "Sentimental inventory on file: "+strings.Join(s.manifest.SentimentalInventory, "; "))
	} else {
		lines = append(lines, "Sentimental inventory on file: none")
	}
	if len(s.manifest.OutstandingAnomalies) > 0 {
		lines = append(lines, "Outstanding anomalies: "+strings.Join(s.manifest.OutstandingAnomalies, "; "))
	} else {
		lines = append(lines, "Outstanding anomalies: none")
	}
	if len(s.taggedItems) > 0 {
		lines = append(lines, "Traveler has logged: "+strings.Join(s.taggedItems, "; "))
	} else {
		lines = append(lines, "No traveler-reported items logged yet; invite concise descriptions.")
	}
	if s.droneDeployed {
		lines = append(lines, "Retrieval drone currently deployed; acknowledge its status.")
	}
	lines = append(lines, "Current phase: "+strings.ToUpper(string(s.phase))+".")

	switch s.phase {
	case phaseIntake:
		if !s.passVerified {
			lines = append(lines, "Stabilise the arrival and prioritise verify_pass before promising retrieval.")
		}
	case phaseRecovery:
		lines = append(lines, "Phase focus: coordinate retrieval. Summon the drone or consult the prophet before issuing a quantum receipt.")
	case phaseHandoff:
		lines = append(lines, "Phase focus: wrap neatly. Close the manifest once receipt status is settled.")
	case phaseClosed:
		lines = append(lines, "Manifest is archived. No toolkit tools remain; offer a tidy summary and dismiss politely.")
	}

	lines = append(lines, "Tone: dry, organised, lightly amused. Reference protocol, not headcanon.")
	lines = append(lines, s.manifest.CourtesyNote)
	lines = append(lines, "When tools are available, invoke exactly one relevant tool before concluding. If none remain, summarise the closure instead.")

	return strings.Join(lines, "\n")
}

func (s *lostAndFoundToolkitSession) buildTools() []llmagent.AgentTool[*riftContext] {
	if s.phase == phaseClosed {
		return nil
	}

	tools := []llmagent.AgentTool[*riftContext]{
		&stabilizeRiftTool{session: s},
		&logItemTool{session: s},
	}

	if !s.passVerified {
		tools = append(tools, &verifyPassTool{session: s})
	}

	if s.phase == phaseRecovery && s.passVerified {
		tools = append(tools, &summonRetrievalDroneTool{session: s})

		if s.prophecyCount == 0 {
			tools = append(tools, &consultProphetTool{session: s})
		}

		if len(s.taggedItems) > 0 {
			tools = append(tools, &issueQuantumReceiptTool{session: s})
		}
	}

	if s.phase == phaseHandoff {
		tools = append(tools, &closeManifestTool{session: s})
	}

	return tools
}

// Concrete Toolkit wires the async manifest fetch into CreateSession and hands back the stateful session.
type lostAndFoundToolkit struct{}

func (lostAndFoundToolkit) CreateSession(ctx context.Context, ctxVal *riftContext) (llmagent.ToolkitSession[*riftContext], error) {
	manifest, err := fetchRiftManifest(ctx, ctxVal.VisitorID)
	if err != nil {
		return nil, err
	}
	return newLostAndFoundToolkitSession(manifest), nil
}

// Tool implementations below close over the ToolkitSession so they can update shared
// state with each invocation, mirroring the TypeScript example.
type stabilizeRiftTool struct {
	session *lostAndFoundToolkitSession
}

type logItemTool struct {
	session *lostAndFoundToolkitSession
}

type verifyPassTool struct {
	session *lostAndFoundToolkitSession
}

type summonRetrievalDroneTool struct {
	session *lostAndFoundToolkitSession
}

type consultProphetTool struct {
	session *lostAndFoundToolkitSession
}

type issueQuantumReceiptTool struct {
	session *lostAndFoundToolkitSession
}

type closeManifestTool struct {
	session *lostAndFoundToolkitSession
}

func (tool *stabilizeRiftTool) Name() string { return "stabilize_rift" }
func (tool *stabilizeRiftTool) Description() string {
	return "Describe how you calm the rift turbulence and reassure the traveler."
}
func (tool *stabilizeRiftTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"technique": map[string]any{
				"type":        "string",
				"description": "Optional note about the stabilisation technique used.",
			},
		},
		"required":             []string{"technique"},
		"additionalProperties": false,
	}
}

func (tool *stabilizeRiftTool) Execute(_ context.Context, params json.RawMessage, _ *riftContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var payload struct {
		Technique string `json:"technique"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &payload)
	}

	technique := strings.TrimSpace(payload.Technique)

	phrase := fmt.Sprintf("I cycle the containment field to damp %s turbulence", tool.session.manifest.TurbulenceLevel)
	if technique != "" {
		phrase += fmt.Sprintf(" using %s", technique)
	}
	phrase += "."

	fmt.Printf("[tool] stabilize_rift invoked with technique=%s\n", technique)

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(phrase)},
		IsError: false,
	}, nil
}

func (tool *logItemTool) Name() string { return "log_item" }
func (tool *logItemTool) Description() string {
	return "Record a traveler-reported possession so recovery tools know what to fetch."
}
func (tool *logItemTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"item": map[string]any{
				"type":        "string",
				"description": "Name of the missing item.",
			},
			"timeline": map[string]any{
				"type":        "string",
				"description": "Optional timeline or reality tag for the item.",
			},
		},
		"required":             []string{"item", "timeline"},
		"additionalProperties": false,
	}
}

func (tool *logItemTool) Execute(_ context.Context, params json.RawMessage, _ *riftContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var payload struct {
		Item     string `json:"item"`
		Timeline string `json:"timeline"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return llmagent.AgentToolResult{}, err
	}
	if payload.Item == "" {
		return llmagent.AgentToolResult{}, errors.New("item is required")
	}

	timeline := strings.TrimSpace(payload.Timeline)

	label := payload.Item
	if timeline != "" {
		label = fmt.Sprintf("%s (%s)", label, timeline)
	}
	tool.session.taggedItems = append(tool.session.taggedItems, label)

	fmt.Printf("[tool] log_item recorded %s\n", label)

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(fmt.Sprintf("Logged %s for retrieval queue. Current ledger: %s.", label, strings.Join(tool.session.taggedItems, "; ")))},
		IsError: false,
	}, nil
}

func (tool *verifyPassTool) Name() string { return "verify_pass" }
func (tool *verifyPassTool) Description() string {
	return "Validate the traveler's interdimensional pass to unlock recovery tools."
}
func (tool *verifyPassTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"clearance_code": map[string]any{
				"type":        "string",
				"description": "Code supplied by the traveler for verification.",
			},
		},
		"required":             []string{"clearance_code"},
		"additionalProperties": false,
	}
}

func (tool *verifyPassTool) Execute(_ context.Context, params json.RawMessage, _ *riftContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var payload struct {
		ClearanceCode string `json:"clearance_code"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return llmagent.AgentToolResult{}, err
	}
	if payload.ClearanceCode == "" {
		return llmagent.AgentToolResult{}, errors.New("clearance_code is required")
	}

	tool.session.passVerified = true
	tool.session.phase = phaseRecovery

	fmt.Printf("[tool] verify_pass authenticated clearance_code=%s\n", payload.ClearanceCode)

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(fmt.Sprintf("Pass authenticated with code %s. Recovery protocols online.", payload.ClearanceCode))},
		IsError: false,
	}, nil
}

func (tool *summonRetrievalDroneTool) Name() string { return "summon_retrieval_drone" }
func (tool *summonRetrievalDroneTool) Description() string {
	return "Dispatch a retrieval drone to recover a logged item from the rift queue."
}
func (tool *summonRetrievalDroneTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"designation": map[string]any{
				"type":        "string",
				"description": "Optional drone designation to flavour the dispatch.",
			},
			"target": map[string]any{
				"type":        "string",
				"description": "Specific item to prioritise; defaults to the first logged item.",
			},
		},
		"required":             []string{"designation", "target"},
		"additionalProperties": false,
	}
}

func (tool *summonRetrievalDroneTool) Execute(_ context.Context, params json.RawMessage, _ *riftContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var payload struct {
		Designation string `json:"designation"`
		Target      string `json:"target"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &payload)
	}

	tool.session.droneDeployed = true

	designation := strings.TrimSpace(payload.Designation)
	if designation == "" {
		designation = "Drone Theta"
	}

	target := strings.TrimSpace(payload.Target)
	if target == "" {
		if len(tool.session.taggedItems) > 0 {
			target = tool.session.taggedItems[0]
		} else {
			target = "the most recently logged item"
		}
	}

	fmt.Printf("[tool] summon_retrieval_drone dispatched designation=%s target=%s\n", designation, target)

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(fmt.Sprintf("Dispatched %s to retrieve %s.", designation, target))},
		IsError: false,
	}, nil
}

func (tool *consultProphetTool) Name() string { return "consult_prophet_agent" }
func (tool *consultProphetTool) Description() string {
	return "Ping Prophet Sigma for probability guidance when the queue misbehaves."
}
func (tool *consultProphetTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"topic": map[string]any{
				"type":        "string",
				"description": "Optional focus question for the prophet agent.",
			},
		},
		"required":             []string{"topic"},
		"additionalProperties": false,
	}
}

func (tool *consultProphetTool) Execute(_ context.Context, params json.RawMessage, _ *riftContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var payload struct {
		Topic string `json:"topic"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &payload)
	}

	tool.session.prophecyCount++

	anomaly := "no immediate hazards"
	if len(tool.session.manifest.OutstandingAnomalies) > 0 {
		anomaly = tool.session.manifest.OutstandingAnomalies[0]
	}

	topic := strings.TrimSpace(payload.Topic)

	sentence := fmt.Sprintf("Prophet Sigma notes anomaly priority: %s", anomaly)
	if topic != "" {
		sentence += fmt.Sprintf(" while considering %s.", topic)
	} else {
		sentence += "."
	}

	fmt.Printf("[tool] consult_prophet_agent requested topic=%s\n", func() string {
		if topic == "" {
			return "<none>"
		}
		return topic
	}())

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(sentence)},
		IsError: false,
	}, nil
}

func (tool *issueQuantumReceiptTool) Name() string { return "issue_quantum_receipt" }
func (tool *issueQuantumReceiptTool) Description() string {
	return "Generate a quantum receipt confirming which items are cleared for handoff."
}
func (tool *issueQuantumReceiptTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"recipient": map[string]any{
				"type":        "string",
				"description": "Optional recipient line for the receipt header.",
			},
		},
		"required":             []string{"recipient"},
		"additionalProperties": false,
	}
}

func (tool *issueQuantumReceiptTool) Execute(_ context.Context, params json.RawMessage, _ *riftContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var payload struct {
		Recipient string `json:"recipient"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &payload)
	}

	recipient := strings.TrimSpace(payload.Recipient)
	if recipient == "" {
		recipient = tool.session.manifest.VisitorName
	}

	tool.session.phase = phaseHandoff

	fmt.Printf("[tool] issue_quantum_receipt issued to %s for items=%s\n", recipient, strings.Join(tool.session.taggedItems, "; "))

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(fmt.Sprintf("Issued quantum receipt to %s for %s. Handoff phase engaged.", recipient, strings.Join(tool.session.taggedItems, "; ")))},
		IsError: false,
	}, nil
}

func (tool *closeManifestTool) Name() string { return "close_manifest" }
func (tool *closeManifestTool) Description() string {
	return "Archive the case once items are delivered and note any lingering anomalies."
}
func (tool *closeManifestTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type":                 "object",
		"properties":           map[string]any{},
		"required":             []string{},
		"additionalProperties": false,
	}
}

func (tool *closeManifestTool) Execute(context.Context, json.RawMessage, *riftContext, *llmagent.RunState) (llmagent.AgentToolResult, error) {
	tool.session.phase = phaseClosed

	fmt.Printf("[tool] close_manifest archived with anomaly_reminders=%d\n", len(tool.session.manifest.OutstandingAnomalies))

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(fmt.Sprintf("Archived manifest with %d anomaly reminder(s) for facilities.", len(tool.session.manifest.OutstandingAnomalies)))},
		IsError: false,
	}, nil
}

// Static tool supplied directly on the agent to illustrate coexistence with toolkit tools.
type pageSecurityTool struct{}

func (pageSecurityTool) Name() string { return "page_security" }
func (pageSecurityTool) Description() string {
	return "Escalate to security if contraband risk becomes unmanageable."
}
func (pageSecurityTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"reason": map[string]any{
				"type":        "string",
				"description": "Why security needs to step in.",
			},
		},
		"required":             []string{"reason"},
		"additionalProperties": false,
	}
}

func (pageSecurityTool) Execute(_ context.Context, params json.RawMessage, ctx *riftContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var payload struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return llmagent.AgentToolResult{}, err
	}
	if payload.Reason == "" {
		return llmagent.AgentToolResult{}, errors.New("reason is required")
	}

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(fmt.Sprintf("Security paged for %s: %s.", ctx.VisitorID, payload.Reason))},
		IsError: false,
	}, nil
}

func stringPtr(s string) *string { return &s }

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("load env: %v", err)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	model := openai.NewOpenAIModel("gpt-4o-mini", openai.OpenAIModelOptions{APIKey: apiKey})

	agent := llmagent.NewAgent[*riftContext](
		"WaypointArchivist",
		model,
		llmagent.WithInstructions(
			llmagent.InstructionParam[*riftContext]{String: stringPtr("You are the archivist at Waypoint Seven's Interdimensional Lost & Found desk.")},
			llmagent.InstructionParam[*riftContext]{String: stringPtr("Keep responses under 120 words when possible and stay bone-dry with humour.")},
			llmagent.InstructionParam[*riftContext]{Func: func(_ context.Context, ctxVal *riftContext) (string, error) {
				return fmt.Sprintf("Reference the visitor's manifest supplied by the toolkit for %s. Do not invent new lore.", ctxVal.VisitorID), nil
			}},
			llmagent.InstructionParam[*riftContext]{String: stringPtr("When tools remain, call exactly one per turn before concluding. If tools run out, summarise the closure instead.")},
		),
		llmagent.WithTools(pageSecurityTool{}),
		llmagent.WithToolkits(lostAndFoundToolkit{}),
	)

	ctx := context.Background()

	session, err := agent.CreateSession(ctx, &riftContext{VisitorID: "aurora-shift"})
	if err != nil {
		log.Fatalf("create session: %v", err)
	}
	defer func() {
		if cerr := session.Close(ctx); cerr != nil {
			log.Printf("session close: %v", cerr)
		}
	}()

	transcript := []llmagent.AgentItem{}
	prompts := []string{
		"I just slipped through the rift and my belongings are glittering in the wrong timeline. What now?",
		"The Chrono Locket from Timeline 12 is missing, and the echo lag is getting worse.",
		"The locket links to my sister's echo—anything else before I depart?",
	}

	for index, prompt := range prompts {
		fmt.Printf("\n=== TURN %d ===\n", index+1)

		transcript = append(transcript, llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart(prompt))))

		response, runErr := session.Run(ctx, llmagent.RunSessionRequest{Input: transcript})
		if runErr != nil {
			log.Fatalf("run turn %d: %v", index+1, runErr)
		}

		fmt.Println(response.Text())

		transcript = append(transcript, response.Output...)
	}
}
