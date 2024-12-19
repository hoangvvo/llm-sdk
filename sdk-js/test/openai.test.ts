/* eslint-disable @typescript-eslint/no-floating-promises */
/* eslint-disable @typescript-eslint/no-non-null-assertion */
import test, { suite } from "node:test";
import { OpenAIModel } from "../src/openai/openai.js";
import {
  log,
  testLanguageModel,
  testParallelToolCalls,
} from "./test-language-model.js";

const model = new OpenAIModel(
  {
    apiKey: process.env["OPENAI_API_KEY"] as string,
    modelId: "gpt-4o",
  },
  {
    pricing: {
      input_cost_per_text_token: 2.5 / 1_000_000,
      output_cost_per_text_token: 10 / 1_000_000,
    },
  },
);

const audioModel = new OpenAIModel({
  modelId: "gpt-4o-audio-preview",
  apiKey: process.env["OPENAI_API_KEY"] as string,
});

suite("OpenAIModel", () => {
  testLanguageModel(model);

  testParallelToolCalls(model);

  test("convert audio part to text part if enabled", async () => {
    const model = new OpenAIModel({
      apiKey: process.env["OPENAI_API_KEY"] as string,
      // not an audio model
      modelId: "gpt-4o",
      convertAudioPartsToTextParts: true,
    });

    const response = await model.generate({
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
        {
          role: "assistant",
          content: [
            {
              type: "audio",
              audio_data: "",
              transcript: "Hi there, how can I help you?",
            },
          ],
        },
        {
          role: "user",
          content: [
            {
              type: "text",
              text: "Goodbye",
            },
          ],
        },
      ],
    });

    log(response);

    // it should not throw a part unsupported error
  });

  test("generate audio", async (t) => {
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

    log(response);

    const audioPart = response.content.find((part) => part.type === "audio");

    t.assert.equal(!!audioPart, true);
    t.assert.equal(audioPart?.type, "audio");
    t.assert.equal(audioPart!.audio_data.length > 0, true);
    t.assert.equal(audioPart!.encoding!.length > 0, true);
    t.assert.equal(audioPart!.transcript!.length > 0, true);
    t.assert.equal(!!audioPart?.id, true);
  });

  test("stream audio", async (t) => {
    const response = audioModel.stream({
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

    let current = await response.next();
    while (!current.done) {
      log(current.value);
      current = await response.next();
    }

    log(current.value);

    const audioPart = current.value.content.find(
      (part) => part.type === "audio",
    );

    t.assert.equal(!!audioPart, true);
    t.assert.equal(audioPart?.type, "audio");
    t.assert.equal(audioPart!.audio_data.length > 0, true);
    t.assert.equal(audioPart!.encoding!.length > 0, true);
    t.assert.equal(audioPart!.transcript!.length > 0, true);
    t.assert.equal(!!audioPart?.id, true);
  });

  test("generate with assistant audio part", async (t) => {
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
    t.assert.equal(!!audioPart?.id, true);

    const response2 = await audioModel.generate({
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
        {
          role: "assistant",
          content: response.content,
        },
      ],
      extra: {
        audio: {
          voice: "alloy",
          format: "mp3",
        },
      },
    });

    log(response2);

    const audioPart2 = response2.content.find((part) => part.type === "audio");
    t.assert.equal(!!audioPart2, true);
  });
});
