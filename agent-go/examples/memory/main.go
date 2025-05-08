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

// Memory pattern example with core + archival memory tools and instructions.

type MemoryBlock struct {
	ID      string         `json:"id"`
	Content string         `json:"content"`
	Meta    map[string]any `json:"metadata,omitempty"`
}

type Store struct {
	Core     map[string]string
	Archival map[string]string
}

func NewStore() *Store {
	return &Store{Core: map[string]string{}, Archival: map[string]string{}}
}

func (s *Store) FetchCore() []MemoryBlock {
	res := make([]MemoryBlock, 0, len(s.Core))
	for id, content := range s.Core {
		res = append(res, MemoryBlock{ID: id, Content: content})
	}
	return res
}
func (s *Store) UpdateCore(b MemoryBlock) []MemoryBlock {
	if strings.TrimSpace(b.Content) == "" {
		delete(s.Core, b.ID)
	} else {
		s.Core[b.ID] = b.Content
	}
	return s.FetchCore()
}
func (s *Store) SearchArchival(query string) []MemoryBlock {
	q := strings.ToLower(query)
	res := []MemoryBlock{}
	for id, content := range s.Archival {
		if strings.Contains(strings.ToLower(content), q) {
			res = append(res, MemoryBlock{ID: id, Content: content})
		}
	}
	return res
}
func (s *Store) UpdateArchival(b MemoryBlock) {
	if strings.TrimSpace(b.Content) == "" {
		delete(s.Archival, b.ID)
	} else {
		s.Archival[b.ID] = b.Content
	}
}

// No context required for this example
type Ctx = struct{}

// Tools
type CoreMemoryUpdateTool struct{ S *Store }

func (t *CoreMemoryUpdateTool) Name() string { return "core_memory_update" }
func (t *CoreMemoryUpdateTool) Description() string {
	return "Update or add a core memory block. Returns all core memories after the update."
}
func (t *CoreMemoryUpdateTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"id":      map[string]any{"type": "string"},
			"content": map[string]any{"type": "string"},
		},
		"required":             []string{"id", "content"},
		"additionalProperties": false,
	}
}
func (t *CoreMemoryUpdateTool) Execute(ctx context.Context, params json.RawMessage, _ Ctx, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var in struct{ ID, Content string }
	if err := json.Unmarshal(params, &in); err != nil {
		return llmagent.AgentToolResult{}, err
	}
	fmt.Printf("[memory.core_memory_update] id=%s len=%d\n", in.ID, len(in.Content))
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = randID()
	}
	updated := t.S.UpdateCore(MemoryBlock{ID: id, Content: in.Content})
	b, _ := json.Marshal(map[string]any{"core_memories": updated})
	return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart(string(b))}, IsError: false}, nil
}

type ArchivalSearchTool struct{ S *Store }

func (t *ArchivalSearchTool) Name() string { return "archival_memory_search" }
func (t *ArchivalSearchTool) Description() string {
	return "Search for memories in the archival memory"
}
func (t *ArchivalSearchTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type":                 "object",
		"properties":           map[string]any{"query": map[string]any{"type": "string"}},
		"required":             []string{"query"},
		"additionalProperties": false,
	}
}
func (t *ArchivalSearchTool) Execute(ctx context.Context, params json.RawMessage, _ Ctx, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var in struct{ Query string }
	if err := json.Unmarshal(params, &in); err != nil {
		return llmagent.AgentToolResult{}, err
	}
	fmt.Printf("[memory.archival_memory_search] query=\"%s\"\n", in.Query)
	// TODO: Replace substring search with semantic vector search using embeddings
	results := t.S.SearchArchival(in.Query)
	b, _ := json.Marshal(map[string]any{"results": results})
	return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart(string(b))}, IsError: false}, nil
}

type ArchivalUpdateTool struct{ S *Store }

func (t *ArchivalUpdateTool) Name() string { return "archival_memory_update" }
func (t *ArchivalUpdateTool) Description() string {
	return "Update or add a memory block in the archival memory"
}
func (t *ArchivalUpdateTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"id":      map[string]any{"type": "string"},
			"content": map[string]any{"type": "string"},
		},
		"required":             []string{"id", "content"},
		"additionalProperties": false,
	}
}
func (t *ArchivalUpdateTool) Execute(ctx context.Context, params json.RawMessage, _ Ctx, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var in struct{ ID, Content string }
	if err := json.Unmarshal(params, &in); err != nil {
		return llmagent.AgentToolResult{}, err
	}
	fmt.Printf("[memory.archival_memory_update] id=%s len=%d\n", in.ID, len(in.Content))
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = randID()
	}
	t.S.UpdateArchival(MemoryBlock{ID: id, Content: in.Content})
	var resp map[string]any
	if strings.TrimSpace(in.Content) == "" {
		resp = map[string]any{"success": true, "action": "deleted"}
	} else {
		resp = map[string]any{"success": true, "action": "updated", "memory": map[string]any{"id": id, "content": in.Content}}
	}
	b, _ := json.Marshal(resp)
	return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart(string(b))}, IsError: false}, nil
}

func main() {
	godotenv.Load("../.env")

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY must be set")
	}
	model := openai.NewOpenAIModel("gpt-4o", openai.OpenAIModelOptions{APIKey: apiKey})

	store := NewStore()

	// Instructions: static memory guide + dynamic core memories snapshot
    memPrompt := `You can remember information learned from interactions with the user in two types of memory called core memory and archival memory.
Core memory is always available in your conversation context, providing essential, foundational context for keeping track of key details about the user.
As core memory is limited in size, it is important to only store the most important information. For other less important details, use archival memory.
Archival memory is infinite size, but is held outside of your immediate context, so you must explicitly run a search operation to see data inside it.
Archival memory is used to remember less significant details about the user or information found during the conversation. When the user mentions a name, topic, or details you don't know, search your archival memory to see if you have any information about it.`

    rulesPrompt := `You cannot see prior conversation turns beyond what is provided in the current input.
When a user shares a durable preference or profile detail, call core_memory_update to store it.
When asked to recall such facts and it's not present in the current input, rely on the core memories in this prompt.
For less important or long-tail info, use archival_memory_search before answering.`
	coreInstr := func(ctx context.Context, _ Ctx) (string, error) {
		blocks := store.FetchCore()
		b, _ := json.Marshal(blocks)
		return "Core memories (JSON list):\n" + string(b), nil
	}

	agent := llmagent.NewAgent("memory", model,
        llmagent.WithInstructions(
            llmagent.InstructionParam[Ctx]{String: &memPrompt},
            llmagent.InstructionParam[Ctx]{String: &rulesPrompt},
            llmagent.InstructionParam[Ctx]{Func: coreInstr},
        ),
		llmagent.WithTools(
			&CoreMemoryUpdateTool{S: store},
			&ArchivalSearchTool{S: store},
			&ArchivalUpdateTool{S: store},
		),
	)

	// Demo: two separate sessions (second cannot see first turn)
	ctx := context.Background()
	items1 := []llmagent.AgentItem{
		llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("Remember that my favorite color is blue."))),
	}
	res1, err := agent.Run(ctx, llmagent.AgentRequest[Ctx]{Context: Ctx{}, Input: items1})
	if err != nil {
		log.Fatal(err)
	}
	litter.Dump(res1.Content)
	items2 := []llmagent.AgentItem{
		llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("What's my favorite color?"))),
	}
	res2, err := agent.Run(ctx, llmagent.AgentRequest[Ctx]{Context: Ctx{}, Input: items2})
	if err != nil {
		log.Fatal(err)
	}
	litter.Dump(res2.Content)
}

func randID() string {
	// simple pseudo id
	return fmt.Sprintf("%x", os.Getpid())
}
