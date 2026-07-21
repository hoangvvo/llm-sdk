import { runTestGroup, SHARED_BEHAVIOR_TEST_GROUPS } from "#test-common/cases";
import { runTransportTestGroup } from "#test-common/transports";
import assert from "node:assert";
import test, { suite } from "node:test";
import { AnthropicModel } from "./anthropic.ts";

suite("AnthropicModel", () => {
  const apiKey = process.env["ANTHROPIC_API_KEY"];
  let model: AnthropicModel | undefined;
  function getModel() {
    assert(apiKey, "ANTHROPIC_API_KEY must be set");
    model ??= new AnthropicModel({ apiKey, modelId: "claude-sonnet-5" });
    return model;
  }

  const reasoningOptions = { profile: "anthropic_adaptive_reasoning" };

  for (const group of SHARED_BEHAVIOR_TEST_GROUPS) {
    test(group, { timeout: 120 * 1000 }, (t) => {
      return runTestGroup(t, getModel(), group);
    });
  }

  test("multimodal_tool_result", (t) =>
    runTestGroup(t, getModel(), "multimodal_tool_result"));
  test("web_search", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "web_search", {
      profile: "anthropic_web_search",
    }),
  );
  test("image_input", (t) => runTestGroup(t, getModel(), "image_input"));
  test("reasoning", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "reasoning", reasoningOptions),
  );
  test("reasoning_tool_use", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "reasoning_tool_use"),
  );
  test("anthropic_refusal", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "anthropic_refusal"),
  );
  test("anthropic_web_search_failure", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "anthropic_web_search_failure"),
  );
  test("transport", (t) =>
    runTransportTestGroup(
      t,
      "anthropic_transport",
      (baseURL) =>
        new AnthropicModel({
          apiKey: "test-token",
          modelId: "test-model",
          baseURL,
        }),
    ));
});
