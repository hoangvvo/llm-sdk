/* eslint-disable @typescript-eslint/no-floating-promises */
import { describe, it } from "node:test";
import { AnthropicModel } from "../src/anthropic/anthropic.js";
import { getLanguageModelTests } from "./test-language-model.js";

const model = new AnthropicModel({
  apiKey: process.env["ANTHROPIC_API_KEY"] as string,
  modelId: "claude-3-5-sonnet-20241022",
});

describe("AnthropicModel", () => {
  const tests = getLanguageModelTests(model);
  tests.forEach(({ name, fn }) => {
    it(name, fn);
  });
});
