import { type TestContext } from "node:test";
import {
  getTestCaseInfo,
  prepareStage,
  TEST_CASE_NAMES,
  type StageContext,
  validateOutput,
} from "../../sdk-tests/protocol.mjs";
import { StreamAccumulator } from "../src/accumulator.ts";
import type { LanguageModel } from "../src/language-model.ts";
import type { LanguageModelInput, Message, Part } from "../src/types.ts";

export { TEST_CASE_NAMES };

export interface RunTestCaseOptions {
  /** Apply model-specific changes that cannot be represented by a shared profile. */
  additionalInputs?: (input: LanguageModelInput) => LanguageModelInput;
  /** A named input/expectation profile from sdk-tests/tests.json. */
  profile?: string;
}

function getToolCallParts(parts: Part[]): Part[] {
  return parts.filter((part) => part.type === "tool-call");
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

    let assistantContent: Part[];
    if (stage.method === "generate") {
      const result = await model.generate(modelInput);
      assistantContent = result.content;
    } else {
      const accumulator = new StreamAccumulator();
      for await (const chunk of model.stream(modelInput)) {
        accumulator.addPartial(chunk);
      }
      assistantContent = accumulator.computeResponse().content;
    }

    validateOutput({
      test_case: testCaseName,
      stage: stageIndex,
      content: assistantContent,
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
