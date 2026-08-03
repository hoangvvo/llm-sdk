package llmsdk_test

import (
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
)

func TestCalculateCostCacheAccounting(t *testing.T) {
	pricing := &llmsdk.LanguageModelPricing{
		InputCostPerTextToken:       ptr.To(2.0),
		InputCostPerCachedToken:     ptr.To(0.5),
		InputCostPerCacheWriteToken: ptr.To(2.5),
		OutputCostPerTextToken:      ptr.To(3.0),
	}
	tests := []struct {
		name       string
		usage      llmsdk.ModelUsage
		additional bool
	}{
		{
			name: "included",
			usage: llmsdk.ModelUsage{
				InputTokens: 100, OutputTokens: 10,
				InputTokensDetails: &llmsdk.ModelTokensDetails{
					CachedTokens: ptr.To(40), CacheWriteTokens: ptr.To(20),
				},
			},
		},
		{
			name: "additional",
			usage: llmsdk.ModelUsage{
				InputTokens: 40, OutputTokens: 10,
				InputTokensDetails: &llmsdk.ModelTokensDetails{
					CachedTokens: ptr.To(40), CacheWriteTokens: ptr.To(20),
				},
			},
			additional: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cost := test.usage.CalculateCost(pricing, llmsdk.ModelUsageCostOptions{
				InputCacheTokensAreAdditional: test.additional,
			})
			if cost != 180 {
				t.Fatalf("cost = %v, want 180", cost)
			}
		})
	}
}

func TestCalculateCostUsesModalityCacheBreakdownOnce(t *testing.T) {
	usage := llmsdk.ModelUsage{
		InputTokens: 100,
		InputTokensDetails: &llmsdk.ModelTokensDetails{
			TextTokens: ptr.To(100), CachedTextTokens: ptr.To(80), CachedTokens: ptr.To(80),
		},
	}
	pricing := &llmsdk.LanguageModelPricing{
		InputCostPerTextToken:       ptr.To(2.0),
		InputCostPerCachedToken:     ptr.To(0.1),
		InputCostPerCachedTextToken: ptr.To(0.5),
	}
	if cost := usage.CalculateCost(pricing, llmsdk.ModelUsageCostOptions{}); cost != 80 {
		t.Fatalf("cost = %v, want 80", cost)
	}
}

func TestCalculateCostReasoningAndMissingCacheRate(t *testing.T) {
	t.Run("additional reasoning", func(t *testing.T) {
		usage := llmsdk.ModelUsage{
			OutputTokens:        10,
			OutputTokensDetails: &llmsdk.ModelTokensDetails{ReasoningTokens: ptr.To(5)},
		}
		pricing := &llmsdk.LanguageModelPricing{OutputCostPerTextToken: ptr.To(3.0)}
		if cost := usage.CalculateCost(pricing, llmsdk.ModelUsageCostOptions{}); cost != 30 {
			t.Fatalf("included cost = %v, want 30", cost)
		}
		cost := usage.CalculateCost(pricing, llmsdk.ModelUsageCostOptions{
			OutputReasoningTokensAreAdditional: true,
		})
		if cost != 45 {
			t.Fatalf("cost = %v, want 45", cost)
		}
	})

	t.Run("missing cache-write rate", func(t *testing.T) {
		usage := llmsdk.ModelUsage{
			InputTokens:        40,
			InputTokensDetails: &llmsdk.ModelTokensDetails{CacheWriteTokens: ptr.To(20)},
		}
		pricing := &llmsdk.LanguageModelPricing{InputCostPerTextToken: ptr.To(2.0)}
		cost := usage.CalculateCost(pricing, llmsdk.ModelUsageCostOptions{
			InputCacheTokensAreAdditional: true,
		})
		if cost != 80 {
			t.Fatalf("cost = %v, want 80", cost)
		}
	})
}

func TestCalculateCostAdjustsReportedModalities(t *testing.T) {
	usage := llmsdk.ModelUsage{
		InputTokens:        100,
		InputTokensDetails: &llmsdk.ModelTokensDetails{AudioTokens: ptr.To(20)},
	}
	pricing := &llmsdk.LanguageModelPricing{
		InputCostPerTextToken:  ptr.To(2.0),
		InputCostPerAudioToken: ptr.To(3.0),
	}
	if cost := usage.CalculateCost(pricing, llmsdk.ModelUsageCostOptions{InputCacheTokensAreAdditional: true}); cost != 220 {
		t.Fatalf("cost = %v, want 220", cost)
	}
}

func TestCalculateCostIgnoresZeroOnlyModalityDetails(t *testing.T) {
	usage := llmsdk.ModelUsage{
		InputTokens:         100,
		OutputTokens:        10,
		InputTokensDetails:  &llmsdk.ModelTokensDetails{AudioTokens: ptr.To(0)},
		OutputTokensDetails: &llmsdk.ModelTokensDetails{AudioTokens: ptr.To(0)},
	}
	pricing := &llmsdk.LanguageModelPricing{
		InputCostPerTextToken:   ptr.To(2.0),
		InputCostPerAudioToken:  ptr.To(3.0),
		OutputCostPerTextToken:  ptr.To(4.0),
		OutputCostPerAudioToken: ptr.To(5.0),
	}
	if cost := usage.CalculateCost(pricing, llmsdk.ModelUsageCostOptions{}); cost != 240 {
		t.Fatalf("cost = %v, want 240", cost)
	}
}
