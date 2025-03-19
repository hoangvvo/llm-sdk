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
import { GoogleModel } from "./google.ts";

suite("GoogleModel", () => {
  assert(process.env["GOOGLE_API_KEY"], "GOOGLE_API_KEY must be set");
  const model = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-2.5-flash",
  });

  const thinkingModel = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-2.0-flash-thinking-exp-01-21",
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
      thinkingModel,
      TEST_CASE_GENERATE_REASONING,
      runTestOptions,
    );
  });

  test(TEST_CASE_STREAM_REASONING.name, (t) => {
    return runTestCase(
      t,
      thinkingModel,
      TEST_CASE_STREAM_REASONING,
      runTestOptions,
    );
  });

  test(TEST_CASE_INPUT_REASONING.name, (t) => {
    return runTestCase(
      t,
      thinkingModel,
      TEST_CASE_INPUT_REASONING,
      runTestOptions,
    );
  });
});
