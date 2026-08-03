import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import type { LanguageModelPricing, ModelUsage } from "./types.ts";
import {
  calculateCost,
  mergeModelUsageMax,
  type ModelUsageCostOptions,
} from "./usage.utils.ts";

interface UsageCostCase {
  name: string;
  usage: ModelUsage;
  pricing: LanguageModelPricing;
  options: ModelUsageCostOptions;
  expected_cost: number;
}

const suite = JSON.parse(
  readFileSync(
    new URL("../../sdk-tests/usage-costs.json", import.meta.url),
    "utf8",
  ),
) as { test_cases: UsageCostCase[] };

for (const testCase of suite.test_cases) {
  test(`calculateCost: ${testCase.name}`, () => {
    const actual = calculateCost(
      testCase.usage,
      testCase.pricing,
      testCase.options,
    );
    const tolerance = Math.max(1e-12, Math.abs(testCase.expected_cost) * 1e-12);
    assert.ok(
      Math.abs(actual - testCase.expected_cost) <= tolerance,
      `expected cost ${String(testCase.expected_cost)}, received ${String(actual)}`,
    );
  });
}

test("merges cumulative partial usage without erasing known counts", () => {
  assert.deepEqual(
    mergeModelUsageMax(
      {
        input_tokens: 10,
        output_tokens: 0,
        input_tokens_details: { cached_tokens: 2 },
      },
      {
        input_tokens: 0,
        output_tokens: 5,
        output_tokens_details: { reasoning_tokens: 1 },
      },
    ),
    {
      input_tokens: 10,
      output_tokens: 5,
      input_tokens_details: { cached_tokens: 2 },
      output_tokens_details: { reasoning_tokens: 1 },
    },
  );
});
