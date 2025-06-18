package mcp_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmmcp "github.com/hoangvvo/llm-sdk/agent-go/mcp"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	llmsdktest "github.com/hoangvvo/llm-sdk/sdk-go/llmsdktest"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type listArgs struct {
	Shift string `json:"shift" jsonschema_description:"Which operating window to query." jsonschema_enum:"evening,overnight"`
}

type toolResponder func(args listArgs) *gomcp.CallToolResult

type stubServer struct {
	url        string
	stop       func()
	updateTool func(name, description string, responder toolResponder)
}

var (
	imageBytes  = []byte{0x00, 0x01, 0x02}
	audioBytes  = []byte{0x03, 0x04}
	imageBase64 = base64.StdEncoding.EncodeToString(imageBytes)
	audioBase64 = base64.StdEncoding.EncodeToString(audioBytes)
)

func startStubMCPServer() (*stubServer, error) {
	server := gomcp.NewServer(&gomcp.Implementation{Name: "stub-mcp", Version: "1.0.0"}, nil)

	toolName := "list_shuttles"
	toolDescription := "List active shuttle routes for a shift"
	responder := func(args listArgs) *gomcp.CallToolResult {
		summary := "Shuttle summary for " + args.Shift + " shift."
		return &gomcp.CallToolResult{
			Content: []gomcp.Content{
				&gomcp.TextContent{Text: summary},
				&gomcp.ImageContent{MIMEType: "image/png", Data: imageBytes},
				&gomcp.AudioContent{MIMEType: "audio/mpeg", Data: audioBytes},
				&gomcp.ResourceLink{URI: "https://example.com/docs", Name: "ignored"},
			},
		}
	}

	registerTool := func() {
		gomcp.AddTool(server, &gomcp.Tool{
			Name:        toolName,
			Description: toolDescription,
		}, func(ctx context.Context, _ *gomcp.CallToolRequest, args listArgs) (*gomcp.CallToolResult, any, error) {
			_ = ctx
			return responder(args), nil, nil
		})
	}

	registerTool()

	handler := gomcp.NewStreamableHTTPHandler(
		func(*http.Request) *gomcp.Server { return server },
		&gomcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	srv := &http.Server{Handler: handler}
	go func() {
		if serveErr := srv.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			// best effort logging via testing output when available
		}
	}()

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}

	update := func(name, description string, nextResponder toolResponder) {
		server.RemoveTools(toolName)
		if name != "" {
			toolName = name
		}
		if description != "" {
			toolDescription = description
		}
		if nextResponder != nil {
			responder = nextResponder
		}
		registerTool()
	}

	return &stubServer{
		url:        "http://" + listener.Addr().String(),
		stop:       cleanup,
		updateTool: update,
	}, nil
}

func TestMCPToolkitSessionHydratesToolsAndExecutes(t *testing.T) {
	stub, err := startStubMCPServer()
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			var sysErr *os.SyscallError
			if errors.As(opErr.Err, &sysErr) {
				switch sysErr.Err {
				case syscall.EACCES, syscall.EPERM:
					t.Skipf("skipping: listening on loopback blocked (%v)", sysErr.Err)
				}
			}
		}
		t.Fatalf("start stub server: %v", err)
	}
	t.Cleanup(stub.stop)

	ctx := context.Background()

	model := llmsdktest.NewMockLanguageModel()
	toolCallArgs, err := json.Marshal(map[string]string{"shift": "evening"})
	if err != nil {
		t.Fatalf("marshal tool args: %v", err)
	}
	model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
		Content: []llmsdk.Part{
			{
				ToolCallPart: &llmsdk.ToolCallPart{
					ToolCallID: "call_1",
					ToolName:   "list_shuttles",
					Args:       toolCallArgs,
				},
			},
		},
	}))
	model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
		Content: []llmsdk.Part{{TextPart: &llmsdk.TextPart{Text: "Ready to roll."}}},
	}))

	agent := llmagent.NewAgent[struct{}](
		"mcp-test",
		model,
		llmagent.WithToolkits(
			llmmcp.NewMCPToolkit(llmmcp.StaticMCPInit[struct{}](llmmcp.NewMCPStreamableHTTPParams(stub.url, ""))),
		),
	)

	session, err := agent.CreateSession(ctx, struct{}{})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	t.Cleanup(func() {
		if cerr := session.Close(ctx); cerr != nil {
			var exitErr *exec.ExitError
			if errors.As(cerr, &exitErr) && exitErr.ExitCode() == 13 {
				return
			}
			t.Errorf("close session: %v", cerr)
		}
	})

	resp, err := session.Run(ctx, llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("What shuttles are running tonight?"))),
		},
	})
	if err != nil {
		t.Fatalf("run session: %v", err)
	}

	expected := &llmagent.AgentResponse{
		Content: []llmsdk.Part{{TextPart: &llmsdk.TextPart{Text: "Ready to roll."}}},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					{
						ToolCallPart: &llmsdk.ToolCallPart{
							ToolCallID: "call_1",
							ToolName:   "list_shuttles",
							Args:       toolCallArgs,
						},
					},
				},
			}),
			llmagent.NewAgentItemTool(
				"call_1",
				"list_shuttles",
				toolCallArgs,
				[]llmsdk.Part{
					{TextPart: &llmsdk.TextPart{Text: "Shuttle summary for evening shift."}},
					{ImagePart: &llmsdk.ImagePart{MimeType: "image/png", ImageData: imageBase64}},
					{AudioPart: &llmsdk.AudioPart{AudioData: audioBase64, Format: llmsdk.AudioFormatMP3}},
				},
				false,
			),
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{{TextPart: &llmsdk.TextPart{Text: "Ready to roll."}}},
			}),
		},
	}

	if diff := cmp.Diff(expected, resp); diff != "" {
		t.Fatalf("agent response mismatch (-want +got): %s", diff)
	}
}

func TestMCPToolkitSessionRefreshesToolsOnChange(t *testing.T) {
	stub, err := startStubMCPServer()
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			var sysErr *os.SyscallError
			if errors.As(opErr.Err, &sysErr) {
				switch sysErr.Err {
				case syscall.EACCES, syscall.EPERM:
					t.Skipf("skipping: listening on loopback blocked (%v)", sysErr.Err)
				}
			}
		}
		t.Fatalf("start stub server: %v", err)
	}
	t.Cleanup(stub.stop)

	ctx := context.Background()

	model := llmsdktest.NewMockLanguageModel()
	toolCallArgs, err := json.Marshal(map[string]string{"shift": "evening"})
	if err != nil {
		t.Fatalf("marshal tool args: %v", err)
	}

	model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
		Content: []llmsdk.Part{
			{
				ToolCallPart: &llmsdk.ToolCallPart{
					ToolCallID: "call_1",
					ToolName:   "list_shuttles",
					Args:       toolCallArgs,
				},
			},
		},
	}))
	model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
		Content: []llmsdk.Part{{TextPart: &llmsdk.TextPart{Text: "Ready to roll."}}},
	}))
	model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
		Content: []llmsdk.Part{
			{
				ToolCallPart: &llmsdk.ToolCallPart{
					ToolCallID: "call_2",
					ToolName:   "list_shuttles_v2",
					Args:       toolCallArgs,
				},
			},
		},
	}))
	model.EnqueueGenerateResult(llmsdktest.NewMockGenerateResultResponse(llmsdk.ModelResponse{
		Content: []llmsdk.Part{{TextPart: &llmsdk.TextPart{Text: "Routes synced."}}},
	}))

	agent := llmagent.NewAgent[struct{}](
		"mcp-test",
		model,
		llmagent.WithToolkits(
			llmmcp.NewMCPToolkit(llmmcp.StaticMCPInit[struct{}](llmmcp.NewMCPStreamableHTTPParams(stub.url, ""))),
		),
	)

	session, err := agent.CreateSession(ctx, struct{}{})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	t.Cleanup(func() {
		if cerr := session.Close(ctx); cerr != nil {
			var exitErr *exec.ExitError
			if errors.As(cerr, &exitErr) && exitErr.ExitCode() == 13 {
				return
			}
			t.Errorf("close session: %v", cerr)
		}
	})

	firstResp, err := session.Run(ctx, llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("What shuttles are running tonight?"))),
		},
	})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}

	expectedFirst := &llmagent.AgentResponse{
		Content: []llmsdk.Part{{TextPart: &llmsdk.TextPart{Text: "Ready to roll."}}},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					{
						ToolCallPart: &llmsdk.ToolCallPart{
							ToolCallID: "call_1",
							ToolName:   "list_shuttles",
							Args:       toolCallArgs,
						},
					},
				},
			}),
			llmagent.NewAgentItemTool(
				"call_1",
				"list_shuttles",
				toolCallArgs,
				[]llmsdk.Part{
					{TextPart: &llmsdk.TextPart{Text: "Shuttle summary for evening shift."}},
					{ImagePart: &llmsdk.ImagePart{MimeType: "image/png", ImageData: imageBase64}},
					{AudioPart: &llmsdk.AudioPart{AudioData: audioBase64, Format: llmsdk.AudioFormatMP3}},
				},
				false,
			),
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{{TextPart: &llmsdk.TextPart{Text: "Ready to roll."}}},
			}),
		},
	}
	if diff := cmp.Diff(expectedFirst, firstResp); diff != "" {
		t.Fatalf("first response mismatch (-want +got): %s", diff)
	}

	stub.updateTool(
		"list_shuttles_v2",
		"List active shuttle routes with live updates",
		func(args listArgs) *gomcp.CallToolResult {
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{
					&gomcp.TextContent{Text: "Updated shuttle roster for " + args.Shift + " shift."},
				},
			}
		},
	)

	time.Sleep(20 * time.Millisecond)

	secondResp, err := session.Run(ctx, llmagent.RunSessionRequest{
		Input: []llmagent.AgentItem{
			llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart("How about now?"))),
		},
	})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	expectedSecond := &llmagent.AgentResponse{
		Content: []llmsdk.Part{{TextPart: &llmsdk.TextPart{Text: "Routes synced."}}},
		Output: []llmagent.AgentItem{
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{
					{
						ToolCallPart: &llmsdk.ToolCallPart{
							ToolCallID: "call_2",
							ToolName:   "list_shuttles_v2",
							Args:       toolCallArgs,
						},
					},
				},
			}),
			llmagent.NewAgentItemTool(
				"call_2",
				"list_shuttles_v2",
				toolCallArgs,
				[]llmsdk.Part{{TextPart: &llmsdk.TextPart{Text: "Updated shuttle roster for evening shift."}}},
				false,
			),
			llmagent.NewAgentItemModelResponse(llmsdk.ModelResponse{
				Content: []llmsdk.Part{{TextPart: &llmsdk.TextPart{Text: "Routes synced."}}},
			}),
		},
	}
	if diff := cmp.Diff(expectedSecond, secondResp); diff != "" {
		t.Fatalf("second response mismatch (-want +got): %s", diff)
	}
}
