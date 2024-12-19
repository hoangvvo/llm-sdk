/* eslint-disable @typescript-eslint/no-floating-promises */
import test, { suite } from "node:test";
import { GoogleModel } from "../src/google/google.js";
import {
  log,
  testLanguageModel,
  testParallelToolCalls,
} from "./test-language-model.js";

const model = new GoogleModel(
  {
    apiKey: process.env["GOOGLE_API_KEY"] as string,
    modelId: "gemini-1.5-pro",
  },
  {
    pricing: {
      input_cost_per_text_token: 1.25 / 1_000_000,
      output_cost_per_text_token: 5.0 / 1_000_000,
    },
  },
);

suite("GoogleModel", () => {
  testLanguageModel(model);

  testParallelToolCalls(model);

  test("convert audio part to text part if enabled", async () => {
    const model = new GoogleModel({
      apiKey: process.env["GOOGLE_API_KEY"] as string,
      // not an audio model
      modelId: "gemini-1.5-pro",
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
});
