package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmmcp "github.com/hoangvvo/llm-sdk/agent-go/mcp"
)

type RunStreamBody struct {
	Provider             string                            `json:"provider"`
	ModelID              string                            `json:"model_id"`
	Input                llmagent.AgentRequest[*MyContext] `json:"input"`
	EnabledTools         []string                          `json:"enabled_tools,omitempty"`
	MCPServers           []llmmcp.MCPParams                `json:"mcp_servers,omitempty"`
	DisabledInstructions bool                              `json:"disabled_instructions,omitempty"`
	Temperature          *float64                          `json:"temperature,omitempty"`
	TopP                 *float64                          `json:"top_p,omitempty"`
	TopK                 *int                              `json:"top_k,omitempty"`
	FrequencyPenalty     *float64                          `json:"frequency_penalty,omitempty"`
	PresencePenalty      *float64                          `json:"presence_penalty,omitempty"`
}

func runStreamHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req RunStreamBody
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	apiKey := r.Header.Get("Authorization")

	modelList, err := getModelList()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var modelInfo *ModelInfo
	for _, m := range modelList {
		if m.Provider == req.Provider && m.ModelID == req.ModelID {
			modelInfo = &m
			break
		}
	}

	if modelInfo == nil {
		http.Error(w, fmt.Sprintf("Model not found: %s - %s", req.Provider, req.ModelID), http.StatusBadRequest)
		return
	}

	model := getModel(req.Provider, req.ModelID, modelInfo.Metadata, apiKey)

	var enabledTools []string
	if req.EnabledTools != nil {
		enabledTools = slices.Compact(req.EnabledTools)
	}

	for _, mcpServer := range req.MCPServers {
		if _, ok := mcpServer.StdioParams(); ok && os.Getenv("ALLOW_STDIO_MCP") != "true" {
			http.Error(w, "Stdio MCP server is not allowed. Set ALLOW_STDIO_MCP=true to allow it.", http.StatusBadRequest)
			return
		}
	}

	options := &AgentOptions{
		EnabledTools:         enabledTools,
		MCPServers:           req.MCPServers,
		DisabledInstructions: req.DisabledInstructions,
		Temperature:          req.Temperature,
		TopP:                 req.TopP,
		TopK:                 req.TopK,
		FrequencyPenalty:     req.FrequencyPenalty,
		PresencePenalty:      req.PresencePenalty,
		Audio:                modelInfo.Audio,
		Reasoning:            modelInfo.Reasoning,
		Modalities:           modelInfo.Modalities,
	}

	agent := createAgent(model, modelInfo, options)

	stream, err := agent.RunStream(context.Background(), req.Input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	for stream.Next() {
		event := stream.Current()
		data, err := json.Marshal(event)
		if err != nil {
			errorData, _ := json.Marshal(map[string]string{"event": "error", "error": err.Error()})
			fmt.Fprintf(w, "data: %s\n\n", errorData)
			flusher.Flush()
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	if err := stream.Err(); err != nil {
		errorData, _ := json.Marshal(map[string]string{"event": "error", "error": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", errorData)
		flusher.Flush()
	}
}

func listModelsHandler(w http.ResponseWriter) {
	modelList, err := getModelList()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(modelList); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func listToolsHandler(w http.ResponseWriter) {
	tools := make([]map[string]string, 0, len(availableTools))
	for _, tool := range availableTools {
		tools = append(tools, map[string]string{
			"name":        tool.Name(),
			"description": tool.Description(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tools); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4321")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

func main() {
	mux := http.NewServeMux()

	// Handle CORS preflight
	mux.HandleFunc("OPTIONS /", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		w.WriteHeader(http.StatusNoContent)
	})

	// Routes
	mux.HandleFunc("POST /run-stream", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		runStreamHandler(w, r)
	})

	mux.HandleFunc("GET /models", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		listModelsHandler(w)
	})

	mux.HandleFunc("GET /tools", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		listToolsHandler(w)
	})

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		fmt.Fprintf(w, `Welcome to llm-agent-go Server!
GitHub: https://github.com/hoangvvo/llm-sdk`)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}

	log.Printf("Server listening on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
