import { runTestCase, TEST_CASE_NAMES } from "#test-common/cases";
import assert from "node:assert";
import test, { suite } from "node:test";
import { OpenAIModel } from "./openai.ts";

suite("OpenAIModel", () => {
  assert(process.env["OPENAI_API_KEY"], "OPENAI_API_KEY must be set");
  const model = new OpenAIModel({
    apiKey: process.env["OPENAI_API_KEY"],
    modelId: "gpt-5",
  });

  const audioModel = new OpenAIModel({
    modelId: "gpt-4o-audio-preview",
    apiKey: process.env["OPENAI_API_KEY"],
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

  test(TEST_CASE_NAMES.GENERATE_IMAGE, { timeout: 60 * 1000 }, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_IMAGE);
  });

  test(TEST_CASE_NAMES.STREAM_IMAGE, { timeout: 60 * 1000 }, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_IMAGE);
  });

  test(TEST_CASE_NAMES.GENERATE_IMAGE_INPUT, { timeout: 60 * 1000 }, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_IMAGE_INPUT);
  });

  test(TEST_CASE_NAMES.STREAM_IMAGE_INPUT, { timeout: 60 * 1000 }, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_IMAGE_INPUT);
  });

  test(
    TEST_CASE_NAMES.GENERATE_AUDIO,
    { skip: "responses api does not support audio" },
    (t) => {
      return runTestCase(t, audioModel, TEST_CASE_NAMES.GENERATE_AUDIO, {
        additionalInputs: (input) => ({
          ...input,
          audio: {
            format: "mp3",
            voice: "alloy",
          },
        }),
      });
    },
  );

  test(
    TEST_CASE_NAMES.STREAM_AUDIO,
    { skip: "responses api does not support audio" },
    (t) => {
      return runTestCase(t, audioModel, TEST_CASE_NAMES.STREAM_AUDIO, {
        additionalInputs: (input) => ({
          ...input,
          audio: {
            format: "linear16",
            voice: "alloy",
          },
        }),
      });
    },
  );

  test(TEST_CASE_NAMES.GENERATE_REASONING, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_REASONING);
  });

  test(TEST_CASE_NAMES.STREAM_REASONING, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_REASONING);
  });
});
