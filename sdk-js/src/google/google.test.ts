import { runTestGroup, SHARED_BEHAVIOR_TEST_GROUPS } from "#test-common/cases";
import { runTransportTestGroup } from "#test-common/transports";
import assert from "node:assert";
import test, { suite } from "node:test";
import { GoogleModel } from "./google.ts";

suite("GoogleModel", () => {
  const apiKey = process.env["GOOGLE_API_KEY"];
  const models = new Map<string, GoogleModel>();
  function getModel(modelId = "gemini-3.1-flash-lite") {
    assert(apiKey, "GOOGLE_API_KEY must be set");
    const existing = models.get(modelId);
    if (existing) return existing;
    const model = new GoogleModel({ apiKey, modelId });
    models.set(modelId, model);
    return model;
  }

  for (const group of SHARED_BEHAVIOR_TEST_GROUPS) {
    test(group, { timeout: 120 * 1000 }, (t) => {
      return runTestGroup(t, getModel(), group);
    });
  }

  test("multimodal_tool_result", (t) =>
    runTestGroup(
      t,
      getModel("gemini-3.1-pro-preview"),
      "multimodal_tool_result",
    ));
  test("web_search", (t) =>
    runTestGroup(t, getModel(), "web_search", {
      profile: "google_web_search",
    }));
  test("image_generation", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel("gemini-3.1-flash-image"), "image_generation"),
  );
  test("image_input", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel("gemini-3.1-flash-image"), "image_input"),
  );
  test("audio_generation", (t) =>
    runTestGroup(
      t,
      getModel("gemini-3.1-flash-tts-preview"),
      "audio_generation",
      {
        profile: "google_audio",
      },
    ));
  test("reasoning", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel("gemini-3.1-pro-preview"), "reasoning"),
  );
  test("reasoning_tool_use", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel("gemini-3.1-pro-preview"), "reasoning_tool_use"),
  );
  test("transport", (t) =>
    runTransportTestGroup(
      t,
      "google_transport",
      (baseURL) =>
        new GoogleModel({
          apiKey: "test-token",
          modelId: "test-model",
          baseURL,
        }),
    ));
});
