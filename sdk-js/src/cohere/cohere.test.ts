import { runTestGroup, SHARED_BEHAVIOR_TEST_GROUPS } from "#test-common/cases";
import assert from "assert";
import test, { suite } from "node:test";
import { CohereModel } from "./cohere.ts";

suite("CohereModel", () => {
  assert(process.env["CO_API_KEY"], "COHERE_API_KEY must be set");
  const model = new CohereModel({
    apiKey: process.env["CO_API_KEY"],
    modelId: "command-a-plus-05-2026",
  });

  const reasoningModel = new CohereModel({
    apiKey: process.env["CO_API_KEY"],
    modelId: "command-a-plus-05-2026",
  });

  const visionModel = new CohereModel({
    apiKey: process.env["CO_API_KEY"],
    modelId: "command-a-plus-05-2026",
  });

  for (const group of SHARED_BEHAVIOR_TEST_GROUPS) {
    test(group, { timeout: 120 * 1000 }, (t) => {
      return runTestGroup(t, model, group, {
        profile: "cohere_behavior_limits",
      });
    });
  }

  test("image_input", (t) => runTestGroup(t, visionModel, "image_input"));
  test("reasoning", (t) => runTestGroup(t, reasoningModel, "reasoning"));
});
