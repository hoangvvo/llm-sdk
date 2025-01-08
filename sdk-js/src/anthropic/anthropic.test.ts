/* eslint-disable @typescript-eslint/no-floating-promises */

import { COMMON_TEST_CASES } from "#test-utils/cases";
import assert from "node:assert";
import { suite } from "node:test";
import { runTests } from "../../test-utils/assert.ts";
import { AnthropicModel } from "./anthropic.ts";

suite("AnthropicModel", () => {
  assert(process.env["ANTHROPIC_API_KEY"], "ANTHROPIC_API_KEY must be set");
  const model = new AnthropicModel(
    {
      apiKey: process.env["ANTHROPIC_API_KEY"],
      modelId: "claude-3-5-sonnet-20241022",
    },
    {
      capabilities: ["function-calling", "image-input"],
    },
  );

  runTests(COMMON_TEST_CASES, model);
});
