/* eslint-disable @typescript-eslint/no-floating-promises */

import {
  TEST_CASE_GENERATE_PARALLEL_TOOL_CALLS,
  TEST_CASE_GENERATE_TEXT,
  TEST_CASE_GENERATE_TEXT_FROM_TOOL_RESULT,
  TEST_CASE_GENERATE_TOOL_CALL,
  TEST_CASE_GENERATE_WITH_SYSTEM_PROMPT,
  TEST_CASE_SOURCE_PART_INPUT,
  TEST_CASE_STREAM_PARALLEL_TOOL_CALLS,
  TEST_CASE_STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME,
  TEST_CASE_STREAM_TEXT,
  TEST_CASE_STREAM_TEXT_FROM_TOOL_RESULT,
  TEST_CASE_STREAM_TOOL_CALL,
  TEST_CASE_STRUCTURED_RESPONSE_FORMAT,
  testTestCase,
} from "#test-common/cases";
import assert from "assert";
import { suite } from "node:test";
import { CohereModel } from "./cohere.ts";

suite("CohereModel", () => {
  assert(process.env["CO_API_KEY"], "COHERE_API_KEY must be set");
  const model = new CohereModel({
    apiKey: process.env["CO_API_KEY"],
    modelId: "command-r-plus",
  });

  const runTestOptions = { compatibleSchema: true };

  testTestCase(model, TEST_CASE_GENERATE_TEXT, runTestOptions);

  testTestCase(model, TEST_CASE_STREAM_TEXT, runTestOptions);

  testTestCase(model, TEST_CASE_GENERATE_WITH_SYSTEM_PROMPT, runTestOptions);

  testTestCase(model, TEST_CASE_GENERATE_TOOL_CALL, runTestOptions);

  testTestCase(model, TEST_CASE_STREAM_TOOL_CALL, runTestOptions);

  testTestCase(model, TEST_CASE_GENERATE_TEXT_FROM_TOOL_RESULT, runTestOptions);

  testTestCase(model, TEST_CASE_STREAM_TEXT_FROM_TOOL_RESULT, runTestOptions);

  testTestCase(model, TEST_CASE_GENERATE_PARALLEL_TOOL_CALLS, runTestOptions);

  testTestCase(model, TEST_CASE_STREAM_PARALLEL_TOOL_CALLS, runTestOptions);

  testTestCase(
    model,
    TEST_CASE_STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME,
    runTestOptions,
  );

  testTestCase(model, TEST_CASE_STRUCTURED_RESPONSE_FORMAT, runTestOptions);

  testTestCase(model, TEST_CASE_SOURCE_PART_INPUT);
});
