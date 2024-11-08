/* eslint-disable @typescript-eslint/no-floating-promises */
import { describe, it } from "node:test";
import { CohereModel } from "../src/cohere/cohere.js";
import { getLanguageModelTests } from "./test-language-model.js";

const model = new CohereModel({
  apiKey: process.env["CO_API_KEY"] as string,
  modelId: "command-r-08-2024",
});

describe("CohereModel", () => {
  const tests = getLanguageModelTests(model);
  tests.forEach(({ name, fn }) => {
    it(name, fn);
  });
});
