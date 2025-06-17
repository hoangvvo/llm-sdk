package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmmcp "github.com/hoangvvo/llm-sdk/agent-go/mcp"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/openai"
	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// This example demonstrates:
// 1. Launching a minimal streamable HTTP MCP server using the official Go SDK.
// 2. Registering that server through the MCP toolkit primitive.
// 3. Having the agent call the remote tool during a conversation.

const (
	listenerAddr = "127.0.0.1:39812"
	serverURL    = "http://" + listenerAddr
	authToken    = "transit-hub-secret"
)

func main() {
	godotenv.Load("../.env")

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	stopServer := startStubMCPServer()
	defer stopServer()

	model := openai.NewOpenAIModel("gpt-4o-mini", openai.OpenAIModelOptions{APIKey: apiKey})

	// Build the agent and register the MCP toolkit so every run hydrates tools from the remote server.
	agent := llmagent.NewAgent[*sessionContext](
		"Sage",
		model,
		llmagent.WithInstructions(
			llmagent.InstructionParam[*sessionContext]{String: stringPtr("You are Sage, the shuttle concierge for the Transit Hub.")},
			llmagent.InstructionParam[*sessionContext]{String: stringPtr("Lean on connected transit systems before guessing, and tailor advice to the rider's shift.")},
			llmagent.InstructionParam[*sessionContext]{Func: func(ctx context.Context, sc *sessionContext) (string, error) {
				if sc == nil {
					return "", fmt.Errorf("session context missing")
				}
				return fmt.Sprintf("You are assisting %s with tonight's shuttle planning.", sc.RiderName), nil
			}},
		),
		llmagent.WithToolkits(
			// The MCP toolkit primitive resolves transport params per session. Here we pull the rider-specific
			// authorization token from context so each agent session connects with the correct credentials.
			llmmcp.NewMCPToolkit[*sessionContext](func(_ context.Context, sc *sessionContext) (llmmcp.MCPParams, error) {
				if sc == nil {
					return llmmcp.MCPParams{}, fmt.Errorf("session context missing")
				}
				return llmmcp.NewMCPStreamableHTTPParams(serverURL, sc.Authorization), nil
			}),
		),
	)

	ctx := context.Background()

	session, err := agent.CreateSession(ctx, &sessionContext{
		RiderName:     "Avery",
		Authorization: authToken,
	})
	if err != nil {
		log.Fatalf("create session: %v", err)
	}
	defer func() {
		if cerr := session.Close(ctx); cerr != nil {
			log.Printf("session close: %v", cerr)
		}
	}()

	transcript := []llmagent.AgentItem{
		llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("What shuttles are running tonight?"))),
	}

	response, err := session.Run(ctx, llmagent.RunSessionRequest{Input: transcript})
	if err != nil {
		log.Fatalf("run: %v", err)
	}

	fmt.Println("=== Agent Response ===")
	fmt.Println(response.Text())
}

// sessionContext illustrates how MCP params can depend on the agent context.
type sessionContext struct {
	RiderName     string
	Authorization string
}

func stringPtr(s string) *string { return &s }

// startStubMCPServer launches a minimal streamable HTTP MCP server using the official SDK.
func startStubMCPServer() func() {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		server := mcp.NewServer(&mcp.Implementation{
			Name:    "shuttle-scheduler",
			Version: "1.0.0",
		}, nil)

		type listArgs struct {
			Shift string `json:"shift" jsonschema_description:"Operating window to query" jsonschema_enum:"evening,overnight"`
		}

		mcp.AddTool(server, &mcp.Tool{
			Name:        "list_shuttles",
			Description: "List active shuttle routes for the selected shift",
		}, func(ctx context.Context, _ *mcp.CallToolRequest, args listArgs) (*mcp.CallToolResult, any, error) {
			_ = ctx
			summary := "Midnight Loop and Harbor Express are on duty tonight."
			if args.Shift == "overnight" {
				summary = "Harbor Express and Dawn Flyer are staged for the overnight shift."
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: summary},
				},
			}, nil, nil
		})

		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true})

	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hasValidToken(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "unauthorized",
				"message": "Provide the shuttle access token.",
			})
			return
		}
		handler.ServeHTTP(w, r)
	}))

	srv := &http.Server{Addr: listenerAddr, Handler: mux}
	ln, err := net.Listen("tcp", listenerAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("mcp server: %v", err)
		}
	}()

	time.Sleep(200 * time.Millisecond)

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}

func hasValidToken(r *http.Request) bool {
	return r.Header.Get("Authorization") == "Bearer "+authToken
}
