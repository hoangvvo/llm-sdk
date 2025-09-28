package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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

type GetCryptoPriceParams struct {
	Symbol            string `json:"symbol"`
	Currency          string `json:"currency"`
	IncludeMarketData bool   `json:"include_market_data"`
}

type GetCryptoPriceTool struct{}

func (t *GetCryptoPriceTool) Name() string {
	return "get_crypto_price"
}

func (t *GetCryptoPriceTool) Description() string {
	return "Get cryptocurrency price and market information"
}

func (t *GetCryptoPriceTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"symbol": map[string]any{
				"type":        "string",
				"description": "Cryptocurrency symbol (e.g., bitcoin, ethereum)",
			},
			"currency": map[string]any{
				"type":        "string",
				"description": "Target currency for price",
				"default":     "usd",
			},
			"include_market_data": map[string]any{
				"type":        "boolean",
				"description": "Include market cap, volume, and price changes",
				"default":     true,
			},
		},
		"required":             []string{"symbol", "currency", "include_market_data"},
		"additionalProperties": false,
	}
}

func (t *GetCryptoPriceTool) Execute(ctx context.Context, paramsJSON json.RawMessage, _ *MyContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params GetCryptoPriceParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	if params.Currency == "" {
		params.Currency = "usd"
	}
	if !params.IncludeMarketData {
		params.IncludeMarketData = true
	}

	u, _ := url.Parse("https://api.coingecko.com/api/v3/simple/price")
	q := u.Query()
	q.Set("ids", strings.ToLower(params.Symbol))
	q.Set("vs_currencies", strings.ToLower(params.Currency))

	if params.IncludeMarketData {
		q.Set("include_market_cap", "true")
		q.Set("include_24hr_vol", "true")
		q.Set("include_24hr_change", "true")
	}
	q.Set("include_last_updated_at", "true")
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
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
				llmsdk.NewTextPart("Failed to get crypto price"),
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

	var data map[string]map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}

	cryptoData, ok := data[strings.ToLower(params.Symbol)]
	if !ok {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Cryptocurrency %s not found", params.Symbol)),
			},
			IsError: true,
		}, nil
	}

	currencyLower := strings.ToLower(params.Currency)
	result := map[string]any{
		"symbol":   params.Symbol,
		"price":    cryptoData[currencyLower],
		"currency": strings.ToUpper(params.Currency),
	}

	if lastUpdated, ok := cryptoData["last_updated_at"].(float64); ok {
		result["last_updated"] = time.Unix(int64(lastUpdated), 0).Format(time.RFC3339)
	}

	if params.IncludeMarketData {
		result["market_cap"] = cryptoData[fmt.Sprintf("%s_market_cap", currencyLower)]
		result["24h_volume"] = cryptoData[fmt.Sprintf("%s_24h_vol", currencyLower)]
		result["24h_change_percent"] = cryptoData[fmt.Sprintf("%s_24h_change", currencyLower)]
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(string(resultJSON)),
		},
		IsError: false,
	}, nil
}
