import { runTestGroup, SHARED_BEHAVIOR_TEST_GROUPS } from "#test-common/cases";
import assert from "node:assert";
import test, { suite } from "node:test";
import { transformInputForCompatibleSchema } from "../../test-common/utils.ts";
import { MistralModel } from "./mistral.ts";

suite("MistralModel", () => {
  assert(process.env["MISTRAL_API_KEY"], "MISTRAL_API_KEY must be set");
  const model = new MistralModel({
    apiKey: process.env["MISTRAL_API_KEY"],
    modelId: "mistral-small-latest",
  });

  const reasoningModel = new MistralModel({
    apiKey: process.env["MISTRAL_API_KEY"],
    modelId: "mistral-small-latest",
  });

  const visionModel = new MistralModel({
    apiKey: process.env["MISTRAL_API_KEY"],
    modelId: "mistral-medium-latest",
  });

  for (const group of SHARED_BEHAVIOR_TEST_GROUPS) {
    test(group, { timeout: 120 * 1000 }, (t) => {
      return runTestGroup(t, model, group, {
        additionalInputs: transformInputForCompatibleSchema,
      });
    });
  }

  test("image_input", (t) => runTestGroup(t, visionModel, "image_input"));
  test("reasoning", (t) => runTestGroup(t, reasoningModel, "reasoning"));
});
