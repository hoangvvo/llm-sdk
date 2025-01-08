/* eslint-disable @typescript-eslint/no-floating-promises */

import { runTests } from "#test-utils/assert";
import { COMMON_TEST_CASES } from "#test-utils/cases";
import assert from "node:assert";
import { suite } from "node:test";
import { GoogleModel } from "./google.ts";

suite("GoogleModel", () => {
  assert(process.env["GOOGLE_API_KEY"], "GOOGLE_API_KEY must be set");
  const model = new GoogleModel(
    {
      apiKey: process.env["GOOGLE_API_KEY"],
      modelId: "gemini-1.5-pro",
    },
    {
      capabilities: [
        "function-calling",
        "image-input",
        "image-output",
        "audio-input",
        "structured-output",
      ],
    },
  );

  runTests(COMMON_TEST_CASES, model, {
    compatibleSchema: true,
  });
});
