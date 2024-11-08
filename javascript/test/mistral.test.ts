/* eslint-disable @typescript-eslint/no-floating-promises */
import test, { suite } from "node:test";
import { MistralModel } from "../src/mistral/mistral.js";
import {
  log,
  testLanguageModel,
  testParallelToolCalls,
} from "./test-language-model.js";

const model = new MistralModel(
  {
    apiKey: process.env["MISTRAL_API_KEY"] as string,
    modelId: "mistral-small-2409",
  },
  {
    pricing: {
      inputCostPerTextToken: 0.2 / 1_000_000,
      outputCostPerTextToken: 0.6 / 1_000_000,
    },
  },
);

suite("MistralModel", () => {
  testLanguageModel(model);

  testParallelToolCalls(model);

  test("convert audio part to text part if enabled", async () => {
    const model = new MistralModel({
      apiKey: process.env["MISTRAL_API_KEY"] as string,
      // not an audio model
      modelId: "mistral-small-2409",
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
              audioData: "",
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
});
