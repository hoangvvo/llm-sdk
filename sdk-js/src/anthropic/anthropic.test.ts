/* eslint-disable @typescript-eslint/no-floating-promises */

import {
  runTestCase,
  TEST_CASE_GENERATE_AUDIO,
  TEST_CASE_GENERATE_PARALLEL_TOOL_CALLS,
  TEST_CASE_GENERATE_REASONING,
  TEST_CASE_GENERATE_TEXT,
  TEST_CASE_GENERATE_TEXT_FROM_TOOL_RESULT,
  TEST_CASE_GENERATE_TOOL_CALL,
  TEST_CASE_GENERATE_WITH_SYSTEM_PROMPT,
  TEST_CASE_INPUT_REASONING,
  TEST_CASE_SOURCE_PART_INPUT,
  TEST_CASE_STREAM_AUDIO,
  TEST_CASE_STREAM_PARALLEL_TOOL_CALLS,
  TEST_CASE_STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME,
  TEST_CASE_STREAM_REASONING,
  TEST_CASE_STREAM_TEXT,
  TEST_CASE_STREAM_TEXT_FROM_TOOL_RESULT,
  TEST_CASE_STREAM_TOOL_CALL,
  TEST_CASE_STRUCTURED_RESPONSE_FORMAT,
  type RunTestCaseOptions,
} from "#test-common/cases";
import assert from "node:assert";
import test, { suite } from "node:test";
import { AnthropicModel } from "./anthropic.ts";

suite("AnthropicModel", () => {
  assert(process.env["ANTHROPIC_API_KEY"], "ANTHROPIC_API_KEY must be set");
  const model = new AnthropicModel({
    apiKey: process.env["ANTHROPIC_API_KEY"],
    modelId: "claude-3-7-sonnet-20250219",
  });

  const reasoningOptions: Partial<RunTestCaseOptions> = {
    additionalInputs: {
      extra: {
        thinking: {
          type: "enabled",
          budget_tokens: 3000,
        },
      },
    },
  };

  test(TEST_CASE_GENERATE_TEXT.name, (t) => {
    return runTestCase(t, model, TEST_CASE_GENERATE_TEXT);
  });

  test(TEST_CASE_STREAM_TEXT.name, (t) => {
    return runTestCase(t, model, TEST_CASE_STREAM_TEXT);
  });

  test(TEST_CASE_GENERATE_WITH_SYSTEM_PROMPT.name, (t) => {
    return runTestCase(t, model, TEST_CASE_GENERATE_WITH_SYSTEM_PROMPT);
  });

  test(TEST_CASE_GENERATE_TOOL_CALL.name, (t) => {
    return runTestCase(t, model, TEST_CASE_GENERATE_TOOL_CALL);
  });

  test(TEST_CASE_STREAM_TOOL_CALL.name, (t) => {
    return runTestCase(t, model, TEST_CASE_STREAM_TOOL_CALL);
  });

  test(TEST_CASE_GENERATE_TEXT_FROM_TOOL_RESULT.name, (t) => {
    return runTestCase(t, model, TEST_CASE_GENERATE_TEXT_FROM_TOOL_RESULT);
  });

  test(TEST_CASE_STREAM_TEXT_FROM_TOOL_RESULT.name, (t) => {
    return runTestCase(t, model, TEST_CASE_STREAM_TEXT_FROM_TOOL_RESULT);
  });

  test(TEST_CASE_GENERATE_PARALLEL_TOOL_CALLS.name, (t) => {
    return runTestCase(t, model, TEST_CASE_GENERATE_PARALLEL_TOOL_CALLS);
  });

  test(TEST_CASE_STREAM_PARALLEL_TOOL_CALLS.name, (t) => {
    return runTestCase(t, model, TEST_CASE_STREAM_PARALLEL_TOOL_CALLS);
  });

  test(TEST_CASE_STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME.name, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME,
    );
  });

  test(TEST_CASE_STRUCTURED_RESPONSE_FORMAT.name, (t) => {
    return runTestCase(t, model, TEST_CASE_STRUCTURED_RESPONSE_FORMAT);
  });

  test(TEST_CASE_SOURCE_PART_INPUT.name, (t) => {
    return runTestCase(t, model, TEST_CASE_SOURCE_PART_INPUT);
  });

  test(
    TEST_CASE_GENERATE_AUDIO.name,
    { skip: "model does not support audio" },
    (t) => {
      return runTestCase(t, model, TEST_CASE_GENERATE_AUDIO);
    },
  );

  test(
    TEST_CASE_STREAM_AUDIO.name,
    { skip: "model does not support audio" },
    (t) => {
      return runTestCase(t, model, TEST_CASE_STREAM_AUDIO);
    },
  );

  test(TEST_CASE_GENERATE_REASONING.name, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_GENERATE_REASONING,
      reasoningOptions,
    );
  });

  test(TEST_CASE_STREAM_REASONING.name, (t) => {
    return runTestCase(t, model, TEST_CASE_STREAM_REASONING, reasoningOptions);
  });

  test(TEST_CASE_INPUT_REASONING.name, (t) => {
    return runTestCase(t, model, TEST_CASE_INPUT_REASONING, reasoningOptions);
  });
});
