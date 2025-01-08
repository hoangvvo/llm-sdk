/* eslint-disable @typescript-eslint/no-floating-promises */

import { runTests } from "#test-utils/assert";
import { COMMON_TEST_CASES } from "#test-utils/cases";
import assert from "assert";
import { suite } from "node:test";
import { CohereModel } from "./cohere.ts";

suite("CohereModel", () => {
  assert(process.env["CO_API_KEY"], "COHERE_API_KEY must be set");
  const model = new CohereModel(
    {
      apiKey: process.env["CO_API_KEY"],
      modelId: "command-r-plus",
    },
    {
      capabilities: ["function-calling", "image-input", "structured-output"],
    },
  );

  runTests(COMMON_TEST_CASES, model, {
    compatibleSchema: true,
  });
});
