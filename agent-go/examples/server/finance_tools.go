package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

type GetStockPriceParams struct {
	Symbol string `json:"symbol"`
}

type GetStockPriceTool struct{}

func (t *GetStockPriceTool) Name() string {
	return "get_stock_price"
}

func (t *GetStockPriceTool) Description() string {
	return "Get current or historical stock price information"
}

func (t *GetStockPriceTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"symbol": map[string]any{
				"type":        "string",
				"description": "Stock ticker symbol",
			},
		},
		"required":             []string{"symbol"},
		"additionalProperties": false,
	}
}

func (t *GetStockPriceTool) Execute(ctx context.Context, paramsJSON json.RawMessage, _ *MyContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params GetStockPriceParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s", params.Symbol)
	resp, err := http.Get(url)
	if err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Failed to get stock price for %s", params.Symbol)),
			},
			IsError: true,
		}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}

	chart := data["chart"].(map[string]any)
	results := chart["result"].([]any)
	if len(results) == 0 {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart("No data found"),
			},
			IsError: true,
		}, nil
	}

	quote := results[0].(map[string]any)
	meta := quote["meta"].(map[string]any)

	result := map[string]any{
		"symbol":         params.Symbol,
		"price":          meta["regularMarketPrice"],
		"open":           meta["regularMarketOpen"],
		"high":           meta["regularMarketDayHigh"],
		"low":            meta["regularMarketDayLow"],
		"previous_close": meta["previousClose"],
		"timestamp":      time.Unix(int64(meta["regularMarketTime"].(float64)), 0).Format(time.RFC3339),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(string(resultJSON)),
		},
		IsError: false,
	}, nil
}
