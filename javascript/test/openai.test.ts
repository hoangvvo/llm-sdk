/* eslint-disable @typescript-eslint/no-floating-promises */
import test, { suite } from "node:test";
import { OpenAIModel } from "../src/openai/openai.js";
import {
  getAudioLanguageModelTests,
  getLanguageModelTests,
} from "./test-language-model.js";

const model = new OpenAIModel({
  apiKey: process.env["OPENAI_API_KEY"] as string,
  modelId: "gpt-4o",
});

const audioModel = new OpenAIModel({
  modelId: "gpt-4o-audio-preview",
  apiKey: process.env["OPENAI_API_KEY"] as string,
});

suite("OpenAIModel", () => {
  const tests = getLanguageModelTests(model);
  tests.forEach(({ name, fn }) => {
    test(name, fn);
  });
});

suite("OpenAIModel/audio", () => {
  const tests = getAudioLanguageModelTests(audioModel);
  tests.forEach(({ name, fn }) => {
    test(name, fn);
  });
});
