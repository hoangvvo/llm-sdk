package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
	"github.com/sanity-io/litter"
)

var myBalance = 1000

const stockPrice = 100

type tradeArgs struct {
	Action   string `json:"action"`
	Quantity int    `json:"quantity"`
	Symbol   string `json:"symbol"`
}

type tradeResult struct {
	Success       bool `json:"success"`
	Balance       int  `json:"balance"`
	BalanceChange int  `json:"balance_change"`
}

func trade(args tradeArgs) tradeResult {
	fmt.Printf("[TOOLS trade()] Trading %d shares of %s with action: %s\n", args.Quantity, args.Symbol, args.Action)

	var balanceChange int
	if args.Action == "buy" {
		balanceChange = -args.Quantity * stockPrice
	} else {
		balanceChange = args.Quantity * stockPrice
	}

	myBalance += balanceChange

	return tradeResult{
		Success:       true,
		Balance:       myBalance,
		BalanceChange: balanceChange,
	}
}

func main() {
	model := examples.GetModel("openai", "gpt-4o")

	maxTurnLeft := 10

	tools := []llmsdk.Tool{
		{
			Name:        "trade",
			Description: "Trade stocks",
			Parameters: llmsdk.JSONSchema{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{
						"type":        "string",
						"enum":        []string{"buy", "sell"},
						"description": "The action to perform",
					},
					"quantity": map[string]any{
						"type":        "number",
						"description": "The number of stocks to trade",
					},
					"symbol": map[string]any{
						"type":        "string",
						"description": "The stock symbol",
					},
				},
				"required":             []string{"action", "quantity", "symbol"},
				"additionalProperties": false,
			},
		},
	}

	messages := []llmsdk.Message{
		llmsdk.NewUserMessage(
			llmsdk.NewTextPart("I would like to buy 50 NVDA stocks."),
		),
	}

	var response *llmsdk.ModelResponse
	var err error

	for maxTurnLeft > 0 {
		response, err = model.Generate(context.Background(), &llmsdk.LanguageModelInput{
			Messages: messages,
			Tools:    tools,
		})

		if err != nil {
			log.Fatalf("Generation failed: %v", err)
		}

		messages = append(messages, llmsdk.NewAssistantMessage(response.Content...))

		var hasToolCalls bool
		var toolMessage *llmsdk.ToolMessage

		for _, part := range response.Content {
			if part.ToolCallPart != nil {
				hasToolCalls = true

				toolCallPart := part.ToolCallPart
				fmt.Printf("Tool call: %s(%s)\n", toolCallPart.ToolName, toolCallPart.Args)

				var toolResult any
				switch toolCallPart.ToolName {
				case "trade":
					var args tradeArgs
					argsBytes, _ := json.Marshal(toolCallPart.Args)
					if err := json.Unmarshal(argsBytes, &args); err != nil {
						log.Fatalf("Failed to parse trade args: %v", err)
					}
					toolResult = trade(args)
				default:
					log.Fatalf("Tool %s not found", toolCallPart.ToolName)
				}

				if toolMessage == nil {
					toolMessage = &llmsdk.ToolMessage{Content: []llmsdk.Part{}}
				}

				resultBytes, _ := json.Marshal(toolResult)
				toolMessage.Content = append(toolMessage.Content,
					llmsdk.NewToolResultPart(
						toolCallPart.ToolCallID,
						toolCallPart.ToolName,
						[]llmsdk.Part{
							llmsdk.NewTextPart(string(resultBytes)),
						},
					),
				)
			}
		}

		if !hasToolCalls {
			break
		}

		if toolMessage != nil {
			messages = append(messages, llmsdk.NewToolMessage(toolMessage.Content...))
		}

		maxTurnLeft--
	}

	litter.Dump(response)
}
