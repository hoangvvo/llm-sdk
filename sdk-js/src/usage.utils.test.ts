import assert from "node:assert/strict";
import test from "node:test";
import type { LanguageModelPricing, ModelUsage } from "./types.ts";
import { calculateCost } from "./usage.utils.ts";

const pricing: LanguageModelPricing = {
  input_cost_per_text_token: 2,
  input_cost_per_cached_token: 0.5,
  input_cost_per_cache_write_token: 2.5,
  output_cost_per_text_token: 3,
};

test("calculates included and additional cache tokens without double counting", () => {
  const included: ModelUsage = {
    input_tokens: 100,
    output_tokens: 10,
    input_tokens_details: { cached_tokens: 40, cache_write_tokens: 20 },
  };
  const additional: ModelUsage = {
    input_tokens: 40,
    output_tokens: 10,
    input_tokens_details: { cached_tokens: 40, cache_write_tokens: 20 },
  };

  assert.equal(
    calculateCost(included, pricing, {
      input_cache_tokens_are_additional: false,
      output_reasoning_tokens_are_additional: false,
    }),
    180,
  );
  assert.equal(
    calculateCost(additional, pricing, {
      input_cache_tokens_are_additional: true,
      output_reasoning_tokens_are_additional: false,
    }),
    180,
  );
});

test("uses a modality cache breakdown instead of its aggregate duplicate", () => {
  assert.equal(
    calculateCost(
      {
        input_tokens: 100,
        output_tokens: 0,
        input_tokens_details: {
          text_tokens: 100,
          cached_text_tokens: 80,
          cached_tokens: 80,
        },
      },
      {
        input_cost_per_text_token: 2,
        input_cost_per_cached_token: 0.1,
        input_cost_per_cached_text_token: 0.5,
      },
      {
        input_cache_tokens_are_additional: false,
        output_reasoning_tokens_are_additional: false,
      },
    ),
    80,
  );
});

test("adds reasoning only when it is outside the provider output total", () => {
  const usage: ModelUsage = {
    input_tokens: 0,
    output_tokens: 10,
    output_tokens_details: { reasoning_tokens: 5 },
  };
  assert.equal(
    calculateCost(
      usage,
      { output_cost_per_text_token: 3 },
      {
        input_cache_tokens_are_additional: false,
        output_reasoning_tokens_are_additional: false,
      },
    ),
    30,
  );
  assert.equal(
    calculateCost(
      usage,
      { output_cost_per_text_token: 3 },
      {
        input_cache_tokens_are_additional: false,
        output_reasoning_tokens_are_additional: true,
      },
    ),
    45,
  );
});

test("does not price a cache category without an explicit rate", () => {
  assert.equal(
    calculateCost(
      {
        input_tokens: 40,
        output_tokens: 0,
        input_tokens_details: { cache_write_tokens: 20 },
      },
      { input_cost_per_text_token: 2 },
      {
        input_cache_tokens_are_additional: true,
        output_reasoning_tokens_are_additional: false,
      },
    ),
    80,
  );
});

test("adjusts reported modality tokens from the aggregate rate", () => {
  assert.equal(
    calculateCost(
      {
        input_tokens: 100,
        output_tokens: 0,
        input_tokens_details: { audio_tokens: 20 },
      },
      {
        input_cost_per_text_token: 2,
        input_cost_per_audio_token: 3,
      },
      {
        input_cache_tokens_are_additional: true,
        output_reasoning_tokens_are_additional: false,
      },
    ),
    220,
  );
});

test("ignores zero-only modality details", () => {
  assert.equal(
    calculateCost(
      {
        input_tokens: 100,
        output_tokens: 10,
        input_tokens_details: { audio_tokens: 0 },
        output_tokens_details: { audio_tokens: 0 },
      },
      {
        input_cost_per_text_token: 2,
        input_cost_per_audio_token: 3,
        output_cost_per_text_token: 4,
        output_cost_per_audio_token: 5,
      },
      {
        input_cache_tokens_are_additional: false,
        output_reasoning_tokens_are_additional: false,
      },
    ),
    240,
  );
});
