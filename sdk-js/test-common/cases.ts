import { type TestContext } from "node:test";
import {
  getTestCaseInfo,
  getTestCasesByGroup,
  prepareStage,
  TEST_CASE_NAMES,
  type StageContext,
  validateError,
  validateOutput,
} from "../../sdk-tests/protocol.mjs";
import { StreamAccumulator } from "../src/accumulator.ts";
import type { LanguageModel } from "../src/language-model.ts";
import type {
  LanguageModelInput,
  Message,
  ModelResponse,
  Part,
} from "../src/types.ts";

export { TEST_CASE_NAMES };
export const SHARED_BEHAVIOR_TEST_GROUPS = [
  "text_generation",
  "conversation",
  "tool_use",
  "structured_output",
  "generation_options",
  "source_input",
] as const;

export interface RunTestCaseOptions {
  /** Apply model-specific changes that cannot be represented by a shared profile. */
  additionalInputs?: (input: LanguageModelInput) => LanguageModelInput;
  /** A named input/expectation profile from sdk-tests/tests.json. */
  profile?: string;
}

function getToolCallParts(parts: Part[]): Part[] {
  return parts.filter((part) => part.type === "tool-call");
}

function normalizeError(error: unknown): { kind: string; message: string } {
  if (!(error instanceof Error)) {
    return { kind: "error", message: String(error) };
  }
  const kind = error.name.replace(/([a-z0-9])([A-Z])/g, "$1_$2").toLowerCase();
  return { kind, message: error.message };
}

export async function runTestCase(
  _t: TestContext,
  model: LanguageModel,
  testCaseName: string,
  options?: RunTestCaseOptions,
) {
  const { stage_count: stageCount } = getTestCaseInfo(testCaseName);
  const context: StageContext = { stages: [] };
  let history: Message[] = [];

  for (let stageIndex = 0; stageIndex < stageCount; stageIndex += 1) {
    const stage = prepareStage({
      test_case: testCaseName,
      stage: stageIndex,
      context,
      ...(options?.profile ? { profile: options.profile } : {}),
    });
    const stageInput = stage.input as LanguageModelInput;
    const stagedInput = {
      ...stageInput,
      messages: [...history, ...stageInput.messages],
    };
    const modelInput = options?.additionalInputs
      ? options.additionalInputs(stagedInput)
      : stagedInput;

    let response: ModelResponse;
    let stream:
      { partials: number; deltas: number; usage_updates: number } | undefined;
    try {
      if (stage.method === "generate") {
        response = await model.generate(modelInput);
      } else {
        const accumulator = new StreamAccumulator();
        stream = { partials: 0, deltas: 0, usage_updates: 0 };
        for await (const chunk of model.stream(modelInput)) {
          stream.partials += 1;
          if (chunk.delta) stream.deltas += 1;
          if (chunk.usage) stream.usage_updates += 1;
          accumulator.addPartial(chunk);
        }
        response = accumulator.computeResponse();
      }
    } catch (error) {
      validateError({
        test_case: testCaseName,
        stage: stageIndex,
        error: normalizeError(error),
        ...(options?.profile ? { profile: options.profile } : {}),
      });
      return;
    }
    const assistantContent = response.content;

    validateOutput({
      test_case: testCaseName,
      stage: stageIndex,
      content: assistantContent,
      response,
      ...(stream ? { stream } : {}),
      ...(options?.profile ? { profile: options.profile } : {}),
    });

    history = [
      ...stagedInput.messages,
      { role: "assistant", content: assistantContent },
    ];
    context.stages.push({
      assistant: structuredClone(assistantContent),
      tool_calls: structuredClone(getToolCallParts(assistantContent)),
    });
  }
}

export async function runTestGroup(
  t: TestContext,
  model: LanguageModel,
  group: string,
  options?: RunTestCaseOptions,
) {
  for (const testCaseName of getTestCasesByGroup(group)) {
    await t.test(testCaseName, { timeout: 120 * 1000 }, (child) =>
      runTestCase(child, model, testCaseName, options),
    );
  }
}
