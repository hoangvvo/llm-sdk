/* eslint-disable @typescript-eslint/no-floating-promises */

import { runTestCase, TEST_CASE_NAMES } from "#test-common/cases";
import assert from "node:assert";
import test, { suite } from "node:test";
import { transformInputForCompatibleSchema } from "../../test-common/utils.ts";
import { MistralModel } from "./mistral.ts";

suite("MistralModel", () => {
  assert(process.env["MISTRAL_API_KEY"], "MISTRAL_API_KEY must be set");
  const model = new MistralModel({
    apiKey: process.env["MISTRAL_API_KEY"],
    modelId: "mistral-small-2506",
  });

  const reasoningModel = new MistralModel({
    apiKey: process.env["MISTRAL_API_KEY"],
    modelId: "magistral-small-2509",
  });

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
    return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_TOOL_CALL, {
      additionalInputs: transformInputForCompatibleSchema,
    });
  });

  test(TEST_CASE_NAMES.STREAM_TOOL_CALL, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_TOOL_CALL, {
      additionalInputs: transformInputForCompatibleSchema,
    });
  });

  test(TEST_CASE_NAMES.GENERATE_TEXT_FROM_TOOL_RESULT, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.GENERATE_TEXT_FROM_TOOL_RESULT,
      { additionalInputs: transformInputForCompatibleSchema },
    );
  });

  test(TEST_CASE_NAMES.STREAM_TEXT_FROM_TOOL_RESULT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_TEXT_FROM_TOOL_RESULT, {
      additionalInputs: transformInputForCompatibleSchema,
    });
  });

  test(TEST_CASE_NAMES.GENERATE_PARALLEL_TOOL_CALLS, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_PARALLEL_TOOL_CALLS, {
      additionalInputs: transformInputForCompatibleSchema,
    });
  });

  test(TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS, {
      additionalInputs: transformInputForCompatibleSchema,
    });
  });

  test(TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME,
      { additionalInputs: transformInputForCompatibleSchema },
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
    return runTestCase(t, reasoningModel, TEST_CASE_NAMES.GENERATE_REASONING);
  });

  test(TEST_CASE_NAMES.STREAM_REASONING, (t) => {
    return runTestCase(t, reasoningModel, TEST_CASE_NAMES.STREAM_REASONING);
  });
});
