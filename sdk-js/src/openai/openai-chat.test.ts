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
import { OpenAIChatCompletionModel } from "./openai-chat.ts";

suite("OpenAIChatCompletionModel", () => {
  assert(process.env["OPENAI_API_KEY"], "OPENAI_API_KEY must be set");
  const model = new OpenAIChatCompletionModel(
    {
      apiKey: process.env["OPENAI_API_KEY"],
      modelId: "gpt-4o",
    },
    { capabilities: ["function-calling", "image-input", "structured-output"] },
  );

  const audioModel = new OpenAIChatCompletionModel({
    modelId: "gpt-4o-audio-preview",
    apiKey: process.env["OPENAI_API_KEY"],
  });

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

  test(TEST_CASE_GENERATE_AUDIO.name, (t) => {
    return runTestCase(t, audioModel, TEST_CASE_GENERATE_AUDIO, {
      additionalInputs: {
        extra: {
          audio: {
            voice: "alloy",
            format: "mp3",
          },
        },
      },
    });
  });

  test(TEST_CASE_STREAM_AUDIO.name, (t) => {
    return runTestCase(t, audioModel, TEST_CASE_STREAM_AUDIO, {
      additionalInputs: {
        extra: {
          audio: {
            voice: "alloy",
            format: "pcm16",
          },
        },
      },
    });
  });

  test(
    TEST_CASE_GENERATE_REASONING.name,
    { skip: "chat completion does not support reasoning" },
    (t) => {
      return runTestCase(t, model, TEST_CASE_GENERATE_REASONING);
    },
  );

  test(
    TEST_CASE_STREAM_REASONING.name,
    { skip: "chat completion does not support reasoning" },
    (t) => {
      return runTestCase(t, model, TEST_CASE_STREAM_REASONING);
    },
  );

  test(
    TEST_CASE_INPUT_REASONING.name,
    { skip: "chat completion does not support reasoning" },
    (t) => {
      return runTestCase(t, model, TEST_CASE_INPUT_REASONING);
    },
  );
});
