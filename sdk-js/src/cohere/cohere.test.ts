import { runTestCase, TEST_CASE_NAMES } from "#test-common/cases";
import assert from "assert";
import test, { suite } from "node:test";
import { CohereModel } from "./cohere.ts";

suite("CohereModel", () => {
  assert(process.env["CO_API_KEY"], "COHERE_API_KEY must be set");
  const model = new CohereModel({
    apiKey: process.env["CO_API_KEY"],
    modelId: "command-a-03-2025",
  });

  const reasoningModel = new CohereModel({
    apiKey: process.env["CO_API_KEY"],
    modelId: "command-a-reasoning-08-2025",
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
    TEST_CASE_NAMES.GENERATE_IMAGE,
    { skip: "model does not support image generation" },
    (t) => {
      return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_IMAGE);
    },
  );

  test(
    TEST_CASE_NAMES.STREAM_IMAGE,
    { skip: "model does not support image generation" },
    (t) => {
      return runTestCase(t, model, TEST_CASE_NAMES.STREAM_IMAGE);
    },
  );

  test(TEST_CASE_NAMES.GENERATE_IMAGE_INPUT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_IMAGE_INPUT);
  });

  test(TEST_CASE_NAMES.STREAM_IMAGE_INPUT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_IMAGE_INPUT);
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
