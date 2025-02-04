/* eslint-disable @typescript-eslint/no-floating-promises */

import {
  TEST_CASE_GENERATE_PARALLEL_TOOL_CALLS,
  TEST_CASE_GENERATE_TEXT,
  TEST_CASE_GENERATE_TEXT_FROM_TOOL_RESULT,
  TEST_CASE_GENERATE_TOOL_CALL,
  TEST_CASE_GENERATE_WITH_SYSTEM_PROMPT,
  TEST_CASE_SOURCE_PART_INPUT,
  TEST_CASE_STREAM_PARALLEL_TOOL_CALLS,
  TEST_CASE_STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME,
  TEST_CASE_STREAM_TEXT,
  TEST_CASE_STREAM_TEXT_FROM_TOOL_RESULT,
  TEST_CASE_STREAM_TOOL_CALL,
  TEST_CASE_STRUCTURED_RESPONSE_FORMAT,
  testTestCase,
} from "#test-common/cases";
import assert from "node:assert";
import test, { suite, type TestContext } from "node:test";
import { StreamAccumulator } from "../accumulator.ts";
import { OpenAIModel } from "./openai.ts";

suite("OpenAIModel", () => {
  assert(process.env["OPENAI_API_KEY"], "OPENAI_API_KEY must be set");
  const model = new OpenAIModel(
    {
      apiKey: process.env["OPENAI_API_KEY"],
      modelId: "gpt-4o",
    },
    { capabilities: ["function-calling", "image-input", "structured-output"] },
  );

  const audioModel = new OpenAIModel({
    modelId: "gpt-4o-audio-preview",
    apiKey: process.env["OPENAI_API_KEY"],
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

  testTestCase(model, TEST_CASE_SOURCE_PART_INPUT);

  test("generate audio", async (t: TestContext) => {
    const response = await audioModel.generate({
      modalities: ["text", "audio"],
      messages: [
        {
          role: "user",
          content: [
            {
              type: "text",
              text: "Hello",
            },
          ],
        },
      ],
      extra: {
        audio: {
          voice: "alloy",
          format: "mp3",
        },
      },
    });

    const audioPart = response.content.find((part) => part.type === "audio");

    t.assert.ok(audioPart, "Audio part must be present");
    t.assert.ok(audioPart.audio_data, "Audio data must be present");
    t.assert.ok(audioPart.transcript, "Transcript must be present");
    t.assert.ok(audioPart.id, "Audio part ID must be present");
  });

  test("stream audio", async (t: TestContext) => {
    const stream = audioModel.stream({
      modalities: ["text", "audio"],
      messages: [
        {
          role: "user",
          content: [
            {
              type: "text",
              text: "Hello",
            },
          ],
        },
      ],
      extra: {
        audio: {
          voice: "alloy",
          format: "pcm16",
        },
      },
    });

    const accumulator = new StreamAccumulator();

    let current = await stream.next();
    while (!current.done) {
      accumulator.addPartial(current.value);
      current = await stream.next();
    }

    const response = accumulator.computeResponse();

    const audioPart = response.content.find((part) => part.type === "audio");

    t.assert.ok(audioPart, "Audio part must be present");
    t.assert.ok(audioPart.audio_data, "Audio data must be present");
    t.assert.ok(audioPart.transcript, "Transcript must be present");
    t.assert.ok(audioPart.id, "Audio part ID must be present");
  });
});
