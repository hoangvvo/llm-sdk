import { runTestGroup, SHARED_BEHAVIOR_TEST_GROUPS } from "#test-common/cases";
import { runTransportTestGroup } from "#test-common/transports";
import assert from "node:assert";
import test, { suite } from "node:test";
import { OpenAIModel } from "./openai.ts";

suite("OpenAIModel", () => {
  const apiKey = process.env["OPENAI_API_KEY"];
  const models = new Map<string, OpenAIModel>();
  function getModel(modelId = "gpt-5.6-sol") {
    assert(apiKey, "OPENAI_API_KEY must be set");
    const existing = models.get(modelId);
    if (existing) return existing;
    const model = new OpenAIModel({ apiKey, modelId });
    models.set(modelId, model);
    return model;
  }

  for (const group of SHARED_BEHAVIOR_TEST_GROUPS) {
    test(group, { timeout: 120 * 1000 }, (t) => {
      return runTestGroup(t, getModel(), group);
    });
  }

  test("multimodal_tool_result", (t) =>
    runTestGroup(t, getModel(), "multimodal_tool_result"));
  test("web_search", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "web_search"),
  );
  test("image_generation", { timeout: 240 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "image_generation"),
  );
  test("image_input", (t) => runTestGroup(t, getModel(), "image_input"));
  test("reasoning", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel("o1"), "reasoning", {
      profile: "openai_opaque_reasoning",
    }),
  );
  test("reasoning_tool_use", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "reasoning_tool_use"),
  );
  test("transport", (t) =>
    runTransportTestGroup(
      t,
      "openai_transport",
      (baseURL) =>
        new OpenAIModel({
          modelId: "test-model",
          apiKey: "test-token",
          baseURL: `${baseURL}/v1`,
        }),
    ));
});
