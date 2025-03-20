/* eslint-disable @typescript-eslint/no-floating-promises */

import {
  runTestCase,
  TEST_CASE_NAMES,
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

  test(TEST_CASE_NAMES.GENERATE_TEXT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_TEXT);
  });

  test(TEST_CASE_NAMES.STREAM_TEXT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_TEXT);
  });

  test(TEST_CASE_NAMES.GENERATE_WITH_SYSTEM_PROMPT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_WITH_SYSTEM_PROMPT);
  });

  test(TEST_CASE_NAMES.GENERATE_TOOL_CALL, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_TOOL_CALL);
  });

  test(TEST_CASE_NAMES.STREAM_TOOL_CALL, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_TOOL_CALL);
  });

  test(TEST_CASE_NAMES.GENERATE_TEXT_FROM_TOOL_RESULT, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.GENERATE_TEXT_FROM_TOOL_RESULT,
    );
  });

  test(TEST_CASE_NAMES.STREAM_TEXT_FROM_TOOL_RESULT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_TEXT_FROM_TOOL_RESULT);
  });

  test(TEST_CASE_NAMES.GENERATE_PARALLEL_TOOL_CALLS, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_PARALLEL_TOOL_CALLS);
  });

  test(TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS);
  });

  test(TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME,
    );
  });

  test(TEST_CASE_NAMES.STRUCTURED_RESPONSE_FORMAT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STRUCTURED_RESPONSE_FORMAT);
  });

  test(TEST_CASE_NAMES.SOURCE_PART_INPUT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.SOURCE_PART_INPUT);
  });

  test(
    TEST_CASE_NAMES.GENERATE_AUDIO,
    { skip: "model does not support audio" },
    (t) => {
      return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_AUDIO);
    },
  );

  test(
    TEST_CASE_NAMES.STREAM_AUDIO,
    { skip: "model does not support audio" },
    (t) => {
      return runTestCase(t, model, TEST_CASE_NAMES.STREAM_AUDIO);
    },
  );

  test(TEST_CASE_NAMES.GENERATE_REASONING, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.GENERATE_REASONING,
      reasoningOptions,
    );
  });

  test(TEST_CASE_NAMES.STREAM_REASONING, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.STREAM_REASONING,
      reasoningOptions,
    );
  });

  test(TEST_CASE_NAMES.INPUT_REASONING, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.INPUT_REASONING,
      reasoningOptions,
    );
  });
});
