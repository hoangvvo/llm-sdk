import { runTestGroup, SHARED_BEHAVIOR_TEST_GROUPS } from "#test-common/cases";
import assert from "node:assert";
import test, { suite } from "node:test";
import { OpenAIModel } from "./openai.ts";

suite("OpenAIModel", () => {
  assert(process.env["OPENAI_API_KEY"], "OPENAI_API_KEY must be set");
  const model = new OpenAIModel({
    apiKey: process.env["OPENAI_API_KEY"],
    modelId: "gpt-5.6-sol",
  });
  const reasoningModel = new OpenAIModel({
    apiKey: process.env["OPENAI_API_KEY"],
    modelId: "o1",
  });

  for (const group of SHARED_BEHAVIOR_TEST_GROUPS) {
    test(group, { timeout: 120 * 1000 }, (t) => {
      return runTestGroup(t, model, group);
    });
  }

  test("multimodal_tool_result", (t) =>
    runTestGroup(t, model, "multimodal_tool_result"));
  test("web_search", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, model, "web_search"),
  );
  test("image_generation", { timeout: 240 * 1000 }, (t) =>
    runTestGroup(t, model, "image_generation"),
  );
  test("image_input", (t) => runTestGroup(t, model, "image_input"));
  test("reasoning", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, reasoningModel, "reasoning", {
      profile: "openai_opaque_reasoning",
    }),
  );
  test("reasoning_tool_use", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, model, "reasoning_tool_use"),
  );
});
