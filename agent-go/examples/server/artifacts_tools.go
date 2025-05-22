package main

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    llmagent "github.com/hoangvvo/llm-sdk/agent-go"
    llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

// Helpers
func randID(n int) string {
    b := make([]byte, n)
    if _, err := rand.Read(b); err != nil {
        // Fallback to timestamp-based ID if crypto/rand fails
        return fmt.Sprintf("%x", time.Now().UnixNano())
    }
    // Trim to n bytes and hex-encode; take ~n*2 chars
    return strings.ToLower(hex.EncodeToString(b))[:n*2]
}

func findArtifact(ctx *MyContext, id string) *Artifact {
    for i := range ctx.Artifacts {
        if ctx.Artifacts[i].ID == id {
            return &ctx.Artifacts[i]
        }
    }
    return nil
}

// artifact_create
type ArtifactCreateParams struct {
    Title   string       `json:"title"`
    Kind    ArtifactKind `json:"kind"`
    Content string       `json:"content"`
}

type ArtifactCreateTool struct{}

func (t *ArtifactCreateTool) Name() string        { return "artifact_create" }
func (t *ArtifactCreateTool) Description() string { return "Create a new document and return an instruction for the client to persist it" }
func (t *ArtifactCreateTool) Parameters() llmsdk.JSONSchema {
    return llmsdk.JSONSchema{
        "type": "object",
        "properties": map[string]any{
            "title": map[string]any{"type": "string"},
            "kind":  map[string]any{"type": "string", "enum": []string{"markdown", "text", "code"}},
            "content": map[string]any{"type": "string"},
        },
        "required":             []string{"title", "kind", "content"},
        "additionalProperties": false,
    }
}
func (t *ArtifactCreateTool) Execute(_ context.Context, paramsJSON json.RawMessage, _ *MyContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
    var p ArtifactCreateParams
    if err := json.Unmarshal(paramsJSON, &p); err != nil {
        return llmagent.AgentToolResult{}, err
    }
    now := time.Now().UTC().Format(time.RFC3339)
    id := randID(5)
    artifact := Artifact{ID: id, Title: p.Title, Kind: p.Kind, Content: p.Content, Version: ptr(1), UpdatedAt: &now}
    payload, _ := json.Marshal(map[string]any{"op": "artifact_create", "artifact": artifact})
    return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart(string(payload))}}, nil
}

// artifact_update
type ArtifactUpdateParams struct {
    ID      string `json:"id"`
    Content string `json:"content"`
}

type ArtifactUpdateTool struct{}

func (t *ArtifactUpdateTool) Name() string        { return "artifact_update" }
func (t *ArtifactUpdateTool) Description() string { return "Replace document content and return an instruction for the client to persist changes" }
func (t *ArtifactUpdateTool) Parameters() llmsdk.JSONSchema {
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
func (t *ArtifactUpdateTool) Execute(_ context.Context, paramsJSON json.RawMessage, ctx *MyContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
    var p ArtifactUpdateParams
    if err := json.Unmarshal(paramsJSON, &p); err != nil {
        return llmagent.AgentToolResult{}, err
    }
    prev := findArtifact(ctx, p.ID)
    now := time.Now().UTC().Format(time.RFC3339)
    var nextVersion int
    if prev != nil && prev.Version != nil {
        nextVersion = *prev.Version + 1
    } else {
        nextVersion = 1
    }
    title := "Untitled"
    kind := ArtifactKindMarkdown
    prevContent := ""
    if prev != nil {
        if prev.Title != "" {
            title = prev.Title
        }
        kind = prev.Kind
        prevContent = prev.Content
    }
    artifact := Artifact{ID: p.ID, Title: title, Kind: kind, Content: p.Content, Version: &nextVersion, UpdatedAt: &now}
    payload, _ := json.Marshal(map[string]any{"op": "artifact_update", "id": p.ID, "prev_content": prevContent, "artifact": artifact})
    return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart(string(payload))}}, nil
}

// artifact_get
type ArtifactGetParams struct{ ID string `json:"id"` }
type ArtifactGetTool struct{}

func (t *ArtifactGetTool) Name() string        { return "artifact_get" }
func (t *ArtifactGetTool) Description() string { return "Fetch a document from the current client context" }
func (t *ArtifactGetTool) Parameters() llmsdk.JSONSchema {
    return llmsdk.JSONSchema{
        "type": "object",
        "properties": map[string]any{"id": map[string]any{"type": "string"}},
        "required":             []string{"id"},
        "additionalProperties": false,
    }
}
func (t *ArtifactGetTool) Execute(_ context.Context, paramsJSON json.RawMessage, ctx *MyContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
    var p ArtifactGetParams
    if err := json.Unmarshal(paramsJSON, &p); err != nil {
        return llmagent.AgentToolResult{}, err
    }
    artifact := findArtifact(ctx, p.ID)
    payload, _ := json.Marshal(map[string]any{"op": "artifact_get", "id": p.ID, "artifact": artifact})
    return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart(string(payload))}}, nil
}

// artifact_list
type ArtifactListTool struct{}

func (t *ArtifactListTool) Name() string        { return "artifact_list" }
func (t *ArtifactListTool) Description() string { return "List documents from the current client context" }
func (t *ArtifactListTool) Parameters() llmsdk.JSONSchema {
    return llmsdk.JSONSchema{"type": "object", "properties": map[string]any{}, "additionalProperties": false}
}
func (t *ArtifactListTool) Execute(_ context.Context, _ json.RawMessage, ctx *MyContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
    payload, _ := json.Marshal(map[string]any{"op": "artifact_list", "artifacts": ctx.Artifacts})
    return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart(string(payload))}}, nil
}

// artifact_delete
type ArtifactDeleteParams struct{ ID string `json:"id"` }
type ArtifactDeleteTool struct{}

func (t *ArtifactDeleteTool) Name() string        { return "artifact_delete" }
func (t *ArtifactDeleteTool) Description() string { return "Delete a document by id (client will persist)" }
func (t *ArtifactDeleteTool) Parameters() llmsdk.JSONSchema {
    return llmsdk.JSONSchema{
        "type": "object",
        "properties": map[string]any{"id": map[string]any{"type": "string"}},
        "required":             []string{"id"},
        "additionalProperties": false,
    }
}
func (t *ArtifactDeleteTool) Execute(_ context.Context, paramsJSON json.RawMessage, _ *MyContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
    var p ArtifactDeleteParams
    if err := json.Unmarshal(paramsJSON, &p); err != nil {
        return llmagent.AgentToolResult{}, err
    }
    payload, _ := json.Marshal(map[string]any{"op": "artifact_delete", "id": p.ID})
    return llmagent.AgentToolResult{Content: []llmsdk.Part{llmsdk.NewTextPart(string(payload))}}, nil
}

