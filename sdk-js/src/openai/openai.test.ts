/* eslint-disable @typescript-eslint/no-floating-promises */

import { runTests } from "#test-utils/assert";
import { COMMON_TEST_CASES } from "#test-utils/cases";
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

  const audioModel = new OpenAIModel(
    {
      modelId: "gpt-4o-audio-preview",
      apiKey: process.env["OPENAI_API_KEY"],
    },
    {
      capabilities: [
        "audio-input",
        "audio-output",
        "function-calling",
        "image-input",
        "structured-output",
      ],
    },
  );

  runTests(COMMON_TEST_CASES, model);

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
