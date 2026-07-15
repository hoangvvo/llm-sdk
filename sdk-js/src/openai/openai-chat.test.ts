import {
  runTestCase,
  runTestGroup,
  SHARED_BEHAVIOR_TEST_GROUPS,
  TEST_CASE_NAMES,
} from "#test-common/cases";
import assert from "node:assert";
import test, { suite } from "node:test";
import { OpenAIChatModel } from "./openai-chat.ts";

suite("OpenAIChatModel", () => {
  assert(process.env["OPENAI_API_KEY"], "OPENAI_API_KEY must be set");
  const model = new OpenAIChatModel({
    apiKey: process.env["OPENAI_API_KEY"],
    modelId: "gpt-5.6-terra",
  });

  const audioModel = new OpenAIChatModel({
    modelId: "gpt-audio-1.5",
    apiKey: process.env["OPENAI_API_KEY"],
  });

  const noReasoningOptions = { profile: "reasoning_disabled" };

  for (const group of SHARED_BEHAVIOR_TEST_GROUPS) {
    test(group, { timeout: 120 * 1000 }, (t) => {
      return runTestGroup(t, model, group, noReasoningOptions);
    });
  }

  test("image_input", (t) => runTestGroup(t, model, "image_input"));
  test(TEST_CASE_NAMES.GENERATE_AUDIO, (t) =>
    runTestCase(t, audioModel, TEST_CASE_NAMES.GENERATE_AUDIO, {
      profile: "openai_audio_mp3",
    }),
  );
  test(TEST_CASE_NAMES.STREAM_AUDIO, (t) =>
    runTestCase(t, audioModel, TEST_CASE_NAMES.STREAM_AUDIO, {
      profile: "openai_audio_linear16",
    }),
  );
});
