/* eslint-disable @typescript-eslint/no-floating-promises */
import test, { suite } from "node:test";
import { AnthropicModel } from "../src/anthropic/anthropic.js";
import {
  log,
  testLanguageModel,
  testParallelToolCalls,
} from "./test-language-model.js";

const model = new AnthropicModel(
  {
    apiKey: process.env["ANTHROPIC_API_KEY"] as string,
    modelId: "claude-3-5-sonnet-20241022",
  },
  {
    pricing: {
      inputCostPerTextToken: 3.0 / 1_000_000,
      outputCostPerTextToken: 15.0 / 1_000_000,
    },
  },
);

suite("AnthropicModel", () => {
  testLanguageModel(model);

  testParallelToolCalls(model);

  test("convert audio part to text part if enabled", async () => {
    const model = new AnthropicModel({
      apiKey: process.env["ANTHROPIC_API_KEY"] as string,
      // not an audio model
      modelId: "claude-3-5-sonnet-20241022",
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
