import assert from "node:assert/strict";
import test from "node:test";
import { mapCohereUsage } from "./cohere/cohere.ts";
import { mapMistralUsageInfo } from "./mistral/mistral.ts";

test("Mistral derives either missing side from total tokens", () => {
  assert.deepEqual(mapMistralUsageInfo({ promptTokens: 10, totalTokens: 15 }), {
    input_tokens: 10,
    output_tokens: 5,
  });
  assert.deepEqual(
    mapMistralUsageInfo({ completionTokens: 5, totalTokens: 15 }),
    { input_tokens: 10, output_tokens: 5 },
  );
});

test("Mistral preserves a known side without requiring its peer", () => {
  assert.deepEqual(mapMistralUsageInfo({ promptTokens: 10 }), {
    input_tokens: 10,
    output_tokens: 0,
  });
});

test("Cohere falls back from billed units to raw tokens per side", () => {
  assert.deepEqual(
    mapCohereUsage({
      billedUnits: { inputTokens: 10 },
      tokens: { inputTokens: 11, outputTokens: 5 },
    }),
    { input_tokens: 10, output_tokens: 5 },
  );
});

test("Cohere preserves whichever usage counts are available", () => {
  assert.deepEqual(mapCohereUsage({ tokens: { outputTokens: 5 } }), {
    input_tokens: 0,
    output_tokens: 5,
  });
});
