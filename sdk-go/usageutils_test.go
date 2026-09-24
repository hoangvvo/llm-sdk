package llmsdk_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

type usageCostCase struct {
	Name         string                      `json:"name"`
	Usage        llmsdk.ModelUsage           `json:"usage"`
	Pricing      llmsdk.LanguageModelPricing `json:"pricing"`
	ExpectedCost float64                     `json:"expected_cost"`
}

func TestSharedUsageCostCases(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve usage cost test path")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "sdk-tests", "usage-costs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var suite struct {
		TestCases []usageCostCase `json:"test_cases"`
	}
	if err := json.Unmarshal(data, &suite); err != nil {
		t.Fatal(err)
	}

	for _, testCase := range suite.TestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			actual := testCase.Usage.CalculateCost(&testCase.Pricing)
			tolerance := math.Max(1e-12, math.Abs(testCase.ExpectedCost)*1e-12)
			if math.Abs(actual-testCase.ExpectedCost) > tolerance {
				t.Fatalf("cost = %v, want %v", actual, testCase.ExpectedCost)
			}
		})
	}
}
