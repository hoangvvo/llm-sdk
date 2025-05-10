package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	dmp "github.com/sergi/go-diff/diffmatchpatch"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
	"github.com/sanity-io/litter"
)

// Artifacts/Canvas feature in Go: maintain named documents separate from chat.

type ArtifactKind string

const (
	KindMarkdown ArtifactKind = "markdown"
	KindText     ArtifactKind = "text"
	KindCode     ArtifactKind = "code"
)

type Artifact struct {
	ID        string       `json:"id"`
	Title     string       `json:"title"`
	Kind      ArtifactKind `json:"kind"`
	Content   string       `json:"content"`
	Version   int          `json:"version"`
	UpdatedAt string       `json:"updated_at"`
}

type Store struct{ m map[string]*Artifact }

func NewStore() *Store { return &Store{m: map[string]*Artifact{}} }

func (s *Store) Create(title string, kind ArtifactKind, content string) *Artifact {
	id := randID()
	a := &Artifact{ID: id, Title: title, Kind: kind, Content: content, Version: 1, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	s.m[id] = a
	return a
}

func (s *Store) Update(id, content string) (*Artifact, string) {
	a, ok := s.m[id]
	if !ok {
		panic("artifact not found: " + id)
	}
	before := a.Content
	a.Content = content
	a.Version++
	a.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return a, before
}

func (s *Store) Get(id string) *Artifact {
	a, ok := s.m[id]
	if !ok {
		panic("artifact not found: " + id)
	}
	return a
}

func (s *Store) List() []*Artifact {
	out := make([]*Artifact, 0, len(s.m))
	for _, a := range s.m {
		out = append(out, a)
	}
	return out
}

func (s *Store) Delete(id string) bool { _, ok := s.m[id]; delete(s.m, id); return ok }

// Minimal colored line diff using go-diff
func renderDiff(oldText, newText string) string {
	d := dmp.New()
	diffs := d.DiffMain(oldText, newText, false)
	// Coalesce tiny changes to lines
	d.DiffCleanupSemantic(diffs)
	var b strings.Builder
	lines := func(s string) []string { return strings.Split(strings.TrimSuffix(s, "\n"), "\n") }
	for _, df := range diffs {
		switch df.Type {
		case dmp.DiffInsert:
			for _, ln := range lines(df.Text) {
				b.WriteString("\x1b[32m+ " + ln + "\x1b[0m\n")
			}
		case dmp.DiffDelete:
			for _, ln := range lines(df.Text) {
				b.WriteString("\x1b[31m- " + ln + "\x1b[0m\n")
			}
		default:
			for _, ln := range lines(df.Text) {
				b.WriteString("\x1b[2m  " + ln + "\x1b[0m\n")
			}
		}
	}
	return b.String()
}

// No context
type Ctx = struct{}

// Tools
type CreateParams struct {
	Title   string `json:"title"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
}
type UpdateParams struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}
type GetParams struct {
	ID string `json:"id"`
}
type DeleteParams struct {
	ID string `json:"id"`
}

type ArtifactCreateTool struct{ S *Store }

func (t *ArtifactCreateTool) Name() string        { return "artifact_create" }
func (t *ArtifactCreateTool) Description() string { return "Create a new document and return it" }
func (t *ArtifactCreateTool) Parameters() llmsdk.JSONSchema {
	m := llmsdk.JSONSchema{}
	m["type"] = "object"
	props := map[string]any{
		"title":   map[string]any{"type": "string"},
		"kind":    map[string]any{"type": "string", "enum": []string{"markdown", "text", "code"}},
		"content": map[string]any{"type": "string"},
	}
	m["properties"] = props
	m["required"] = []string{"title", "kind", "content"}
	m["additionalProperties"] = false
	return m
}
func (t *ArtifactCreateTool) Execute(_ context.Context, params json.RawMessage, _ Ctx, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var p CreateParams
	if err := json.Unmarshal(params, &p); err != nil {
		return llmagent.AgentToolResult{}, err
	}
	fmt.Printf("[artifacts.create] title=%s kind=%s\n", p.Title, p.Kind)
	a := t.S.Create(p.Title, ArtifactKind(p.Kind), p.Content)
	body, _ := json.Marshal(map[string]interface{}{"artifact": a})
	return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart(string(body))}, IsError: false}, nil
}

type ArtifactUpdateTool struct{ S *Store }

func (t *ArtifactUpdateTool) Name() string { return "artifact_update" }
func (t *ArtifactUpdateTool) Description() string {
	return "Replace the content of a document and return it"
}
func (t *ArtifactUpdateTool) Parameters() llmsdk.JSONSchema {
	m := llmsdk.JSONSchema{}
	m["type"] = "object"
	m["properties"] = map[string]any{"id": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}}
	m["required"] = []string{"id", "content"}
	m["additionalProperties"] = false
	return m
}
func (t *ArtifactUpdateTool) Execute(_ context.Context, params json.RawMessage, _ Ctx, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var p UpdateParams
	if err := json.Unmarshal(params, &p); err != nil {
		return llmagent.AgentToolResult{}, err
	}
	before := t.S.Get(p.ID).Content
	fmt.Printf("[artifacts.update] id=%s len=%d\n", p.ID, len(p.Content))
	a, _ := t.S.Update(p.ID, p.Content)
	fmt.Printf("\n=== Diff (old → new) ===\n%s========================\n\n", renderDiff(before, a.Content))
	body, _ := json.Marshal(map[string]interface{}{"artifact": a})
	return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart(string(body))}, IsError: false}, nil
}

type ArtifactGetTool struct{ S *Store }

func (t *ArtifactGetTool) Name() string        { return "artifact_get" }
func (t *ArtifactGetTool) Description() string { return "Fetch a document by id" }
func (t *ArtifactGetTool) Parameters() llmsdk.JSONSchema {
	m := llmsdk.JSONSchema{}
	m["type"] = "object"
	m["properties"] = map[string]any{"id": map[string]any{"type": "string"}}
	m["required"] = []string{"id"}
	m["additionalProperties"] = false
	return m
}
func (t *ArtifactGetTool) Execute(_ context.Context, params json.RawMessage, _ Ctx, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var p GetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return llmagent.AgentToolResult{}, err
	}
	fmt.Printf("[artifacts.get] id=%s\n", p.ID)
	a := t.S.Get(p.ID)
	body, _ := json.Marshal(map[string]interface{}{"artifact": a})
	return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart(string(body))}, IsError: false}, nil
}

type ArtifactListTool struct{ S *Store }

func (t *ArtifactListTool) Name() string        { return "artifact_list" }
func (t *ArtifactListTool) Description() string { return "List all documents" }
func (t *ArtifactListTool) Parameters() llmsdk.JSONSchema {
	m := llmsdk.JSONSchema{}
	m["type"] = "object"
	m["properties"] = map[string]any{}
	m["additionalProperties"] = false
	return m
}
func (t *ArtifactListTool) Execute(_ context.Context, _ json.RawMessage, _ Ctx, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	fmt.Println("[artifacts.list]")
	body, _ := json.Marshal(map[string]interface{}{"artifacts": t.S.List()})
	return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart(string(body))}, IsError: false}, nil
}

type ArtifactDeleteTool struct{ S *Store }

func (t *ArtifactDeleteTool) Name() string        { return "artifact_delete" }
func (t *ArtifactDeleteTool) Description() string { return "Delete a document by id" }
func (t *ArtifactDeleteTool) Parameters() llmsdk.JSONSchema {
	m := llmsdk.JSONSchema{}
	m["type"] = "object"
	m["properties"] = map[string]any{"id": map[string]any{"type": "string"}}
	m["required"] = []string{"id"}
	m["additionalProperties"] = false
	return m
}
func (t *ArtifactDeleteTool) Execute(_ context.Context, params json.RawMessage, _ Ctx, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var p DeleteParams
	if err := json.Unmarshal(params, &p); err != nil {
		return llmagent.AgentToolResult{}, err
	}
	fmt.Printf("[artifacts.delete] id=%s\n", p.ID)
	ok := t.S.Delete(p.ID)
	body, _ := json.Marshal(map[string]interface{}{"success": ok})
	return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart(string(body))}, IsError: false}, nil
}

func main() {
	godotenv.Load("../.env")
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY must be set")
	}
	model := openai.NewOpenAIModel("gpt-4o", openai.OpenAIModelOptions{APIKey: apiKey})

	store := NewStore()

	overview := "Use documents (artifacts/canvases) for substantive deliverables like documents, plans, specs, or code. Keep chat replies brief and status-oriented; put the full content into a document via the tools. Always reference documents by id."
	rules := "- Prefer creating/updating documents instead of pasting large content into chat\n- When asked to revise or extend prior work, read/update the relevant document\n- Keep the chat response short: what changed, where it lives (document id), and next steps\n"

	agent := llmagent.NewAgent("artifacts", model,
		llmagent.WithInstructions(
			llmagent.InstructionParam[Ctx]{String: &overview},
			llmagent.InstructionParam[Ctx]{String: &rules},
		),
		llmagent.WithTools(
			&ArtifactCreateTool{S: store},
			&ArtifactUpdateTool{S: store},
			&ArtifactGetTool{S: store},
			&ArtifactListTool{S: store},
			&ArtifactDeleteTool{S: store},
		),
	)

	// Demo: create then revise a product requirements document
	ctx := context.Background()
	items1 := []llmagent.AgentItem{
		llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart(
			"We need a product requirements document for a new Todo app. Please draft it in markdown with sections: Overview, Goals, Non-Goals, Requirements. Keep your chat reply short and save the full document to a separate document we can keep iterating on.",
		))),
	}
	res1, err := agent.Run(ctx, llmagent.AgentRequest[Ctx]{Context: Ctx{}, Input: items1})
	if err != nil {
		log.Fatal(err)
	}
	litter.Dump(res1.Content)
	fmt.Println("Documents after creation:")
	litter.Dump(store.List())

	items2 := []llmagent.AgentItem{
		llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart(
			"Please revise the document: expand the Goals section with 3 concrete goals and add a Milestones section. Keep your chat reply brief.",
		))),
	}
	res2, err := agent.Run(ctx, llmagent.AgentRequest[Ctx]{Context: Ctx{}, Input: items2})
	if err != nil {
		log.Fatal(err)
	}
	litter.Dump(res2.Content)
	fmt.Println("Documents after update:")
	litter.Dump(store.List())
}

func randID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
