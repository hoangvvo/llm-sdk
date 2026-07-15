import { runTestGroup, SHARED_BEHAVIOR_TEST_GROUPS } from "#test-common/cases";
import assert from "node:assert";
import test, { suite } from "node:test";
import { AnthropicModel } from "./anthropic.ts";

suite("AnthropicModel", () => {
  assert(process.env["ANTHROPIC_API_KEY"], "ANTHROPIC_API_KEY must be set");
  const model = new AnthropicModel({
    apiKey: process.env["ANTHROPIC_API_KEY"],
    modelId: "claude-sonnet-5",
  });

  const reasoningOptions = { profile: "anthropic_adaptive_reasoning" };

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
  test("image_input", (t) => runTestGroup(t, model, "image_input"));
  test("reasoning", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, model, "reasoning", reasoningOptions),
  );
  test("reasoning_tool_use", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, model, "reasoning_tool_use"),
  );
  test("anthropic_refusal", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, model, "anthropic_refusal"),
  );
});
