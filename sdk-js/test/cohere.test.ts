/* eslint-disable @typescript-eslint/no-floating-promises */
import test, { suite } from "node:test";
import { CohereModel } from "../src/cohere/cohere.js";
import {
  log,
  testLanguageModel,
  testParallelToolCalls,
} from "./test-language-model.js";

const model = new CohereModel(
  {
    apiKey: process.env["CO_API_KEY"] as string,
    modelId: "command-r-08-2024",
  },
  {
    pricing: {
      input_cost_per_text_token: 0.16 / 1_000_000,
      output_cost_per_text_token: 0.6 / 1_000_000,
    },
  },
);

suite("CohereModel", () => {
  testLanguageModel(model);

  testParallelToolCalls(model);

  test("convert audio part to text part if enabled", async () => {
    const model = new CohereModel({
      apiKey: process.env["CO_API_KEY"] as string,
      // not an audio model
      modelId: "command-r-08-2024",
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
