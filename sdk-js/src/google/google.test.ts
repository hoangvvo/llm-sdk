import { runTestGroup, SHARED_BEHAVIOR_TEST_GROUPS } from "#test-common/cases";
import assert from "node:assert";
import test, { suite } from "node:test";
import { GoogleModel } from "./google.ts";

suite("GoogleModel", () => {
  assert(process.env["GOOGLE_API_KEY"], "GOOGLE_API_KEY must be set");
  const model = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-3.1-flash-lite",
  });

  const audioModel = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-3.1-flash-tts-preview",
  });

  const imageModel = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-3.1-flash-image",
  });

  const multimodalToolModel = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-3.1-pro-preview",
  });

  const thinkingModel = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-3.1-pro-preview",
  });

  for (const group of SHARED_BEHAVIOR_TEST_GROUPS) {
    test(group, { timeout: 120 * 1000 }, (t) => {
      return runTestGroup(t, model, group);
    });
  }

  test("multimodal_tool_result", (t) =>
    runTestGroup(t, multimodalToolModel, "multimodal_tool_result"));
  test("web_search", (t) =>
    runTestGroup(t, model, "web_search", { profile: "google_web_search" }));
  test("image_generation", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, imageModel, "image_generation"),
  );
  test("image_input", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, imageModel, "image_input"),
  );
  test("audio_generation", (t) =>
    runTestGroup(t, audioModel, "audio_generation", {
      profile: "google_audio",
    }));
  test("reasoning", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, thinkingModel, "reasoning"),
  );
  test("reasoning_tool_use", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, thinkingModel, "reasoning_tool_use"),
  );
});
