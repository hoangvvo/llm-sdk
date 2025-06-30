package main

import (
	"bufio"
	"bytes"
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
	"github.com/sanity-io/litter"
)

// Human-in-the-loop outline with agent primitives:
// 1. Seed the run with a user `AgentItem` and call `Agent#RunStream` so we capture
//    every emitted `AgentStreamEvent` (model messages, tool results, etc.).
// 2. When the tool throws our user-land `requireApprovalError`, collect the human
//    decision and persist it on the shared RunSession context.
// 3. Repeat step (1) with the accumulated items and mutated context until the tool
//    succeeds or returns an error result that reflects the denial.

type approvalStatus string

const (
	statusApproved approvalStatus = "approved"
	statusDenied   approvalStatus = "denied"
)

type vaultContext struct {
	approvals map[string]approvalStatus
}

type requireApprovalError struct {
	message  string
	artifact string
}

func (e *requireApprovalError) Error() string { return e.message }

// Single AgentTool that inspects the context map and interrupts the run without
// touching the Agent implementation. Thrown errors become AgentToolExecutionError.
type unlockArtifactTool struct{}

func (unlockArtifactTool) Name() string { return "unlock_artifact" }
func (unlockArtifactTool) Description() string {
	return "Unlock an artifact for release once a human supervisor has recorded their approval."
}

func (unlockArtifactTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"artifact": map[string]any{
				"type":        "string",
				"description": "Name of the artifact to release.",
				"minLength":   1,
			},
		},
		"required":             []string{"artifact"},
		"additionalProperties": false,
	}
}

func (unlockArtifactTool) Execute(ctx context.Context, raw json.RawMessage, state *vaultContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params struct {
		Artifact string `json:"artifact"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	artifact := strings.TrimSpace(params.Artifact)
	artifactKey := strings.ToLower(artifact)
	status, ok := state.approvals[artifactKey]
	if !ok {
		return llmagent.AgentToolResult{}, &requireApprovalError{
			message:  fmt.Sprintf("Release of %s requires human approval before it can proceed.", artifact),
			artifact: artifact,
		}
	}

	if status == statusDenied {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{llmsdk.NewTextPart(fmt.Sprintf("Release of %s remains blocked until a supervisor approves it.", artifact))},
			IsError: true,
		}, nil
	}

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{llmsdk.NewTextPart(fmt.Sprintf("%s unlocked. Proceed with standard vault handling protocols.", artifact))},
		IsError: false,
	}, nil
}

func newAgent(model llmsdk.LanguageModel) *llmagent.Agent[*vaultContext] {
	instruction := "You supervise the Eon Vault, safeguarding experimental expedition technology."
	return llmagent.NewAgent[*vaultContext](
		"VaultSentinel",
		model,
		llmagent.WithInstructions(llmagent.InstructionParam[*vaultContext]{String: &instruction}),
		llmagent.WithTools(unlockArtifactTool{}),
	)
}

var transcript []llmagent.AgentItem

// Stream one pass of the agent, appending every AgentStreamItemEvent.
func runStream(agent *llmagent.Agent[*vaultContext], ctxVal *vaultContext) (*llmagent.AgentResponse, error) {
	input := append([]llmagent.AgentItem(nil), transcript...)

	stream, err := agent.RunStream(context.Background(), llmagent.AgentRequest[*vaultContext]{
		Context: ctxVal,
		Input:   input,
	})
	if err != nil {
		return nil, err
	}

	for stream.Next() {
		event := stream.Current()
		if event.Item != nil {
			transcript = append(transcript, event.Item.Item)
			logItem(event.Item.Item)
		}
		if event.Response != nil {
			return event.Response, nil
		}
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	return nil, errors.New("agent stream completed without emitting a response")
}

func logItem(item llmagent.AgentItem) {
	switch item.Type() {
	case llmagent.AgentItemTypeMessage:
		msg := item.Message
		if msg == nil {
			return
		}
		text := extractMessageText(*msg)
		if text != "" {
			fmt.Printf("\n[%s] %s\n", strings.ToLower(messageRole(*msg)), text)
		}
	case llmagent.AgentItemTypeModel:
		if item.Model == nil {
			return
		}
		text := renderParts(item.Model.Content)
		if text != "" {
			fmt.Printf("\n[assistant]\n%s\n", text)
		}
	case llmagent.AgentItemTypeTool:
		tool := item.Tool
		if tool == nil {
			return
		}
		fmt.Printf("\n[tool:%s]\n  input=%s\n", tool.ToolName, indentJSON(tool.Input))
		if output := renderParts(tool.Output); output != "" {
			fmt.Printf("  output=%s\n", output)
		}
	}
}

func promptForApproval(reader *bufio.Reader, artifact string) approvalStatus {
	fmt.Printf("Grant approval to unlock %s? (y/N) ", artifact)
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("read input error: %v\n", err)
		return statusDenied
	}
	decision := strings.TrimSpace(strings.ToLower(line))
	switch decision {
	case "y", "yes":
		return statusApproved
	case "", "n", "no":
		return statusDenied
	default:
		fmt.Println("Unrecognized response, treating as denied.")
		return statusDenied
	}
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
	agent := newAgent(model)

	initialText := "We have an emergency launch window in four hours. Please unlock the Starlight Compass for the Horizon survey team."
	transcript = []llmagent.AgentItem{
		llmagent.NewAgentItemMessage(llmsdk.NewUserMessage(llmsdk.NewTextPart(initialText))),
	}
	fmt.Printf("[user] %s\n", initialText)

	ctxVal := &vaultContext{approvals: map[string]approvalStatus{}}
	reader := bufio.NewReader(os.Stdin)

	for {
		response, err := runStream(agent, ctxVal)
		if err == nil {
			fmt.Println("\nCompleted run.")
			litter.Dump(response.Content)
			break
		}

		var approvalErr *requireApprovalError
		if errors.As(err, &approvalErr) {
			fmt.Printf("\n[agent halted] err = %s\n", approvalErr.Error())
			decision := promptForApproval(reader, approvalErr.artifact)
			key := strings.ToLower(strings.TrimSpace(approvalErr.artifact))
			if key != "" {
				ctxVal.approvals[key] = decision
			}
			continue
		}

		log.Fatalf("run failed: %v", err)
	}
}

func renderParts(parts []llmsdk.Part) string {
	if len(parts) == 0 {
		return ""
	}
	var lines []string
	for _, part := range parts {
		if part.TextPart == nil {
			continue
		}
		trimmed := strings.TrimSpace(part.TextPart.Text)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return strings.Join(lines, "\n")
}

func extractMessageText(message llmsdk.Message) string {
	if message.UserMessage != nil {
		return renderParts(message.UserMessage.Content)
	}
	if message.AssistantMessage != nil {
		return renderParts(message.AssistantMessage.Content)
	}
	if message.ToolMessage != nil {
		return renderParts(message.ToolMessage.Content)
	}
	return ""
}

func messageRole(message llmsdk.Message) string {
	switch {
	case message.UserMessage != nil:
		return "user"
	case message.AssistantMessage != nil:
		return "assistant"
	case message.ToolMessage != nil:
		return "tool"
	default:
		return "unknown"
	}
}

func indentJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "null"
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}
