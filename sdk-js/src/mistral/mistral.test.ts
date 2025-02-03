/* eslint-disable @typescript-eslint/no-floating-promises */

import {
  TEST_CASE_DOCUMENT_PART_INPUT,
  testTestCase,
} from "#test-common/cases";
import assert from "node:assert";
import { suite } from "node:test";
import { MistralModel } from "./mistral.ts";

suite("MistralModel", () => {
  assert(process.env["MISTRAL_API_KEY"], "MISTRAL_API_KEY must be set");
  const model = new MistralModel({
    apiKey: process.env["MISTRAL_API_KEY"],
    modelId: "mistral-small-2409",
  });

  const runTestOptions = { compatibleSchema: true };

  // testTestCase(model, TEST_CASE_GENERATE_TEXT, runTestOptions);

  // testTestCase(model, TEST_CASE_STREAM_TEXT, runTestOptions);

  // testTestCase(model, TEST_CASE_GENERATE_WITH_SYSTEM_PROMPT, runTestOptions);

  // testTestCase(model, TEST_CASE_GENERATE_TOOL_CALL, runTestOptions);

  // testTestCase(model, TEST_CASE_STREAM_TOOL_CALL, runTestOptions);

  // testTestCase(model, TEST_CASE_GENERATE_TEXT_FROM_TOOL_RESULT, runTestOptions);

  // testTestCase(model, TEST_CASE_STREAM_TEXT_FROM_TOOL_RESULT, runTestOptions);

  // testTestCase(model, TEST_CASE_GENERATE_PARALLEL_TOOL_CALLS, runTestOptions);

  // testTestCase(model, TEST_CASE_STREAM_PARALLEL_TOOL_CALLS, runTestOptions);

  // testTestCase(
  //   model,
  //   TEST_CASE_STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME,
  //   runTestOptions,
  // );

  // testTestCase(model, TEST_CASE_STRUCTURED_RESPONSE_FORMAT, runTestOptions);

  testTestCase(model, TEST_CASE_DOCUMENT_PART_INPUT, runTestOptions);
});
