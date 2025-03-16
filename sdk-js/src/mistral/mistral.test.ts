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
} from "#test-common/cases";
import assert from "node:assert";
import test, { suite } from "node:test";
import { MistralModel } from "./mistral.ts";

suite("MistralModel", () => {
  assert(process.env["MISTRAL_API_KEY"], "MISTRAL_API_KEY must be set");
  const model = new MistralModel({
    apiKey: process.env["MISTRAL_API_KEY"],
    modelId: "mistral-small-2409",
  });

  const reasoningModel = new MistralModel({
    apiKey: process.env["MISTRAL_API_KEY"],
    modelId: "mistral-small-2409",
  });

  const runTestOptions = { compatibleSchema: true };

  test(TEST_CASE_GENERATE_TEXT.name, (t) => {
    return runTestCase(t, model, TEST_CASE_GENERATE_TEXT, runTestOptions);
  });

  test(TEST_CASE_STREAM_TEXT.name, (t) => {
    return runTestCase(t, model, TEST_CASE_STREAM_TEXT, runTestOptions);
  });

  test(TEST_CASE_GENERATE_WITH_SYSTEM_PROMPT.name, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_GENERATE_WITH_SYSTEM_PROMPT,
      runTestOptions,
    );
  });

  test(TEST_CASE_GENERATE_TOOL_CALL.name, (t) => {
    return runTestCase(t, model, TEST_CASE_GENERATE_TOOL_CALL, runTestOptions);
  });

  test(TEST_CASE_STREAM_TOOL_CALL.name, (t) => {
    return runTestCase(t, model, TEST_CASE_STREAM_TOOL_CALL, runTestOptions);
  });

  test(TEST_CASE_GENERATE_TEXT_FROM_TOOL_RESULT.name, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_GENERATE_TEXT_FROM_TOOL_RESULT,
      runTestOptions,
    );
  });

  test(TEST_CASE_STREAM_TEXT_FROM_TOOL_RESULT.name, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_STREAM_TEXT_FROM_TOOL_RESULT,
      runTestOptions,
    );
  });

  test(TEST_CASE_GENERATE_PARALLEL_TOOL_CALLS.name, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_GENERATE_PARALLEL_TOOL_CALLS,
      runTestOptions,
    );
  });

  test(TEST_CASE_STREAM_PARALLEL_TOOL_CALLS.name, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_STREAM_PARALLEL_TOOL_CALLS,
      runTestOptions,
    );
  });

  test(TEST_CASE_STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME.name, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME,
      runTestOptions,
    );
  });

  test(TEST_CASE_STRUCTURED_RESPONSE_FORMAT.name, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_STRUCTURED_RESPONSE_FORMAT,
      runTestOptions,
    );
  });

  test(TEST_CASE_SOURCE_PART_INPUT.name, (t) => {
    return runTestCase(t, model, TEST_CASE_SOURCE_PART_INPUT, runTestOptions);
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
      reasoningModel,
      TEST_CASE_GENERATE_REASONING,
      runTestOptions,
    );
  });

  test(TEST_CASE_STREAM_REASONING.name, (t) => {
    return runTestCase(
      t,
      reasoningModel,
      TEST_CASE_STREAM_REASONING,
      runTestOptions,
    );
  });

  test(TEST_CASE_INPUT_REASONING.name, (t) => {
    return runTestCase(
      t,
      reasoningModel,
      TEST_CASE_INPUT_REASONING,
      runTestOptions,
    );
  });
});
