/* eslint-disable @typescript-eslint/no-floating-promises */

import {
  TEST_CASE_DOCUMENT_PART_INPUT,
  TEST_CASE_GENERATE_PARALLEL_TOOL_CALLS,
  TEST_CASE_GENERATE_TEXT,
  TEST_CASE_GENERATE_TEXT_FROM_TOOL_RESULT,
  TEST_CASE_GENERATE_TOOL_CALL,
  TEST_CASE_GENERATE_WITH_SYSTEM_PROMPT,
  TEST_CASE_STREAM_PARALLEL_TOOL_CALLS,
  TEST_CASE_STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME,
  TEST_CASE_STREAM_TEXT,
  TEST_CASE_STREAM_TEXT_FROM_TOOL_RESULT,
  TEST_CASE_STREAM_TOOL_CALL,
  TEST_CASE_STRUCTURED_RESPONSE_FORMAT,
  testTestCase,
} from "#test-common/cases";
import assert from "node:assert";
import { suite } from "node:test";
import { AnthropicModel } from "./anthropic.ts";

suite("AnthropicModel", () => {
  assert(process.env["ANTHROPIC_API_KEY"], "ANTHROPIC_API_KEY must be set");
  const model = new AnthropicModel({
    apiKey: process.env["ANTHROPIC_API_KEY"],
    modelId: "claude-3-5-sonnet-20241022",
  });

  testTestCase(model, TEST_CASE_GENERATE_TEXT);

  testTestCase(model, TEST_CASE_STREAM_TEXT);

  testTestCase(model, TEST_CASE_GENERATE_WITH_SYSTEM_PROMPT);

  testTestCase(model, TEST_CASE_GENERATE_TOOL_CALL);

  testTestCase(model, TEST_CASE_STREAM_TOOL_CALL);

  testTestCase(model, TEST_CASE_GENERATE_TEXT_FROM_TOOL_RESULT);

  testTestCase(model, TEST_CASE_STREAM_TEXT_FROM_TOOL_RESULT);

  testTestCase(model, TEST_CASE_GENERATE_PARALLEL_TOOL_CALLS);

  testTestCase(model, TEST_CASE_STREAM_PARALLEL_TOOL_CALLS);

  testTestCase(model, TEST_CASE_STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME);

  testTestCase(model, TEST_CASE_STRUCTURED_RESPONSE_FORMAT);

  testTestCase(model, TEST_CASE_DOCUMENT_PART_INPUT);
});
