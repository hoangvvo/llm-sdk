import { runTestCase, TEST_CASE_NAMES } from "#test-common/cases";
import assert from "node:assert";
import test, { suite } from "node:test";
import { GoogleModel } from "./google.ts";

suite("GoogleModel", () => {
  assert(process.env["GOOGLE_API_KEY"], "GOOGLE_API_KEY must be set");
  const model = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-2.5-flash",
  });

  const audioModel = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-2.5-flash-preview-tts",
  });

  const imageModel = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-2.5-flash-image-preview",
  });

  const thinkingModel = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-2.0-flash-thinking-exp-01-21",
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
    return runTestCase(t, imageModel, TEST_CASE_NAMES.GENERATE_IMAGE);
  });

  test(TEST_CASE_NAMES.STREAM_IMAGE, { timeout: 60 * 1000 }, (t) => {
    return runTestCase(t, imageModel, TEST_CASE_NAMES.STREAM_IMAGE);
  });

  test(TEST_CASE_NAMES.GENERATE_IMAGE_INPUT, { timeout: 60 * 1000 }, (t) => {
    return runTestCase(t, imageModel, TEST_CASE_NAMES.GENERATE_IMAGE_INPUT);
  });

  test(TEST_CASE_NAMES.STREAM_IMAGE_INPUT, { timeout: 60 * 1000 }, (t) => {
    return runTestCase(t, imageModel, TEST_CASE_NAMES.STREAM_IMAGE_INPUT);
  });

  test(TEST_CASE_NAMES.GENERATE_AUDIO, (t) => {
    return runTestCase(t, audioModel, TEST_CASE_NAMES.GENERATE_AUDIO, {
      additionalInputs: (input) => ({
        ...input,
        modalities: ["audio"],
        audio: {
          voice: "Zephyr",
        },
      }),
      customOutputContent: (content) =>
        content.map((part) => {
          if (part.type === "audio") {
            return { ...part, id: false, transcript: undefined };
          }
          return part;
        }),
    });
  });

  test(TEST_CASE_NAMES.STREAM_AUDIO, (t) => {
    return runTestCase(t, audioModel, TEST_CASE_NAMES.STREAM_AUDIO, {
      additionalInputs: (input) => ({
        ...input,
        modalities: ["audio"],
        audio: {
          voice: "Zephyr",
        },
      }),
      customOutputContent: (content) =>
        content.map((part) => {
          if (part.type === "audio") {
            return { ...part, id: false, transcript: undefined };
          }
          return part;
        }),
    });
  });

  test(TEST_CASE_NAMES.GENERATE_REASONING, (t) => {
    return runTestCase(t, thinkingModel, TEST_CASE_NAMES.GENERATE_REASONING);
  });

  test(TEST_CASE_NAMES.STREAM_REASONING, (t) => {
    return runTestCase(t, thinkingModel, TEST_CASE_NAMES.STREAM_REASONING);
  });
});
