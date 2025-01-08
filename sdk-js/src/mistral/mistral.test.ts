/* eslint-disable @typescript-eslint/no-floating-promises */

import { COMMON_TEST_CASES } from "#test-utils/cases";
import assert from "node:assert";
import { suite } from "node:test";
import { runTests } from "../../test-utils/assert.ts";
import { MistralModel } from "./mistral.ts";

suite("MistralModel", () => {
  assert(process.env["MISTRAL_API_KEY"], "MISTRAL_API_KEY must be set");
  const model = new MistralModel(
    {
      apiKey: process.env["MISTRAL_API_KEY"],
      modelId: "mistral-small-2409",
    },
    {
      capabilities: ["function-calling", "image-input", "structured-output"],
    },
  );

  runTests(COMMON_TEST_CASES, model, {
    compatibleSchema: true,
  });
});
