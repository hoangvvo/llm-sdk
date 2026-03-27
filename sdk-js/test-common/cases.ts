import * as fs from "node:fs";
import * as path from "node:path";
import { type TestContext } from "node:test";
import { getProperty } from "dot-prop";
import { StreamAccumulator } from "../src/accumulator.ts";
import type { LanguageModel } from "../src/language-model.ts";
import type { LanguageModelInput, Message, Part, Tool } from "../src/types.ts";
import { assertContentPart, type PartAssertion } from "./assert.ts";

export interface TestCase {
  name: string;
  stages: TestStage[];
}

interface TestStage {
  inputTemplate: unknown;
  type: "generate" | "stream";
  output: {
    content: PartAssertion[];
  };
}

// Define types for JSON data structure
interface TestDataJSON {
  tools: Tool[];
  test_cases: TestCaseJSON[];
}

interface TestCaseJSON {
  name: string;
  stages: TestStageJSON[];
}

interface TestStageJSON {
  type: "generate" | "stream";
  input: unknown;
  input_tools?: string[];
  expect: StageOutputJSON;
}

interface StageOutputJSON {
  content: {
    type: "text" | "tool_call" | "audio" | "image" | "reasoning";
    text?: string;
    tool_name?: string;
    args?: string;
    id?: boolean;
    transcript?: string;
  }[];
}

function compilePattern(pattern: string) {
  return new RegExp(pattern, "s");
}

// Load test data from JSON
const testDataPath = path.join(
  import.meta.dirname,
  "../../sdk-tests/tests.json",
);
const testDataContent = fs.readFileSync(testDataPath, "utf-8");
const testData: TestDataJSON = JSON.parse(testDataContent) as TestDataJSON;

// Create tool map from JSON
const toolsMap = new Map<string, Tool>();
for (const tool of testData.tools) {
  toolsMap.set(tool.name, tool);
}

// Helper function to resolve tools from names
function resolveTools(toolNames: string[]): Tool[] {
  return toolNames.map((name) => {
    const tool = toolsMap.get(name);
    if (!tool) throw new Error(`Tool ${name} not found in test data`);
    return tool;
  });
}

// Helper function to convert JSON test case to TestCase
function convertOutput(output: StageOutputJSON, testCaseName: string) {
  return {
    content: output.content.map((part): PartAssertion => {
      if (part.type === "text" && part.text) {
        return {
          type: "text",
          text: compilePattern(part.text),
        } as PartAssertion;
      } else if (part.type === "tool_call" && part.tool_name) {
        return {
          type: "tool_call",
          tool_name: part.tool_name,
          args: part.args ? compilePattern(part.args) : undefined,
        } as PartAssertion;
      } else if (part.type === "audio") {
        return {
          type: "audio",
          id: part.id,
          transcript: part.transcript
            ? compilePattern(part.transcript)
            : undefined,
        } as PartAssertion;
      } else if (part.type === "image") {
        return {
          type: "image",
          id: part.id,
        } as PartAssertion;
      } else if (part.type === "reasoning" && part.text) {
        return {
          type: "reasoning",
          text: compilePattern(part.text),
        } as PartAssertion;
      }
      throw new Error(`Invalid part assertion in test case ${testCaseName}`);
    }),
  };
}

function jsonToTestCase(jsonCase: TestCaseJSON): TestCase {
  const stages = jsonCase.stages.map((stage) => {
    const inputTemplate = structuredClone(stage.input);
    const input = inputTemplate as LanguageModelInput;
    if (stage.input_tools) {
      input.tools = resolveTools(stage.input_tools);
    }

    return {
      inputTemplate,
      type: stage.type,
      output: convertOutput(stage.expect, jsonCase.name),
    };
  });

  return {
    name: jsonCase.name,
    stages,
  };
}

// Load test cases from JSON
const testCasesFromJson = testData.test_cases.map(jsonToTestCase);

interface StageRef {
  $ref: string;
}

interface StageExecutionContext {
  stages: Array<{
    assistant: Part[];
    tool_calls: Part[];
  }>;
}

function getToolCallParts(parts: Part[]): Part[] {
  return parts.filter((part) => part.type === "tool-call");
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isStageRef(value: unknown): value is StageRef {
  return isRecord(value) && Object.keys(value).length === 1 && "$ref" in value;
}

function resolvePathValue(path: string, root: StageExecutionContext): unknown {
  const value = getProperty(root, path);
  if (value === undefined) {
    throw new Error(`Invalid stage ref path "${path}"`);
  }
  return structuredClone(value);
}

function resolveStageRefs(
  value: unknown,
  context: StageExecutionContext,
): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => resolveStageRefs(item, context));
  }
  if (isStageRef(value)) {
    return resolvePathValue(value.$ref, context);
  }
  if (isRecord(value)) {
    return Object.fromEntries(
      Object.entries(value).map(([key, child]) => [
        key,
        resolveStageRefs(child, context),
      ]),
    );
  }
  return value;
}

// Test case names for easy reference
export const TEST_CASE_NAMES = {
  GENERATE_TEXT: "generate_text",
  STREAM_TEXT: "stream_text",
  GENERATE_WITH_SYSTEM_PROMPT: "generate_with_system_prompt",
  GENERATE_TOOL_CALL: "generate_tool_call",
  STREAM_TOOL_CALL: "stream_tool_call",
  GENERATE_TEXT_FROM_TOOL_RESULT: "generate_text_from_tool_result",
  STREAM_TEXT_FROM_TOOL_RESULT: "stream_text_from_tool_result",
  GENERATE_TEXT_FROM_IMAGE_TOOL_RESULT: "generate_text_from_image_tool_result",
  GENERATE_PARALLEL_TOOL_CALLS: "generate_parallel_tool_calls",
  STREAM_PARALLEL_TOOL_CALLS: "stream_parallel_tool_calls",
  STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME:
    "stream_parallel_tool_calls_of_same_name",
  STRUCTURED_RESPONSE_FORMAT: "structured_response_format",
  SOURCE_PART_INPUT: "source_part_input",
  GENERATE_AUDIO: "generate_audio",
  STREAM_AUDIO: "stream_audio",
  GENERATE_IMAGE: "generate_image",
  STREAM_IMAGE: "stream_image",
  GENERATE_IMAGE_INPUT: "generate_image_input",
  STREAM_IMAGE_INPUT: "stream_image_input",
  GENERATE_REASONING: "generate_reasoning",
  STREAM_REASONING: "stream_reasoning",
} as const;

export interface RunTestCaseOptions {
  /**
   * Extra parameters to pass to the model input.
   */
  additionalInputs?: (input: LanguageModelInput) => LanguageModelInput;
  customOutputContent?: (content: PartAssertion[]) => PartAssertion[];
}

export async function runTestCase(
  t: TestContext,
  model: LanguageModel,
  testCaseName: string,
  options?: RunTestCaseOptions,
) {
  const testCase = testCasesFromJson.find((tc) => tc.name === testCaseName);
  if (!testCase) {
    throw new Error(`Test case "${testCaseName}" not found`);
  }

  const context: StageExecutionContext = { stages: [] };
  let history: Message[] = [];

  for (const stage of testCase.stages) {
    const resolvedInput = resolveStageRefs(stage.inputTemplate, context);
    const modelInputBase = structuredClone(resolvedInput) as LanguageModelInput;
    const stageMessages = structuredClone(modelInputBase.messages ?? []);
    const stagedInput: LanguageModelInput = {
      ...modelInputBase,
      messages: [...history, ...stageMessages],
    };
    const modelInput = options?.additionalInputs
      ? options.additionalInputs(stagedInput)
      : stagedInput;

    const outputContentBase = structuredClone(stage.output.content);
    const content = options?.customOutputContent
      ? options.customOutputContent(outputContentBase)
      : outputContentBase;

    if (stage.type === "generate") {
      const result = await model.generate(modelInput);
      if (content.length > 0) {
        assertContentPart(t, result.content, content);
      }
      history = [
        ...stagedInput.messages,
        {
          role: "assistant",
          content: result.content,
        },
      ];
      context.stages.push({
        assistant: structuredClone(result.content),
        tool_calls: structuredClone(getToolCallParts(result.content)),
      });
      continue;
    }

    const stream = model.stream(modelInput);
    const accumulator = new StreamAccumulator();

    for await (const chunk of stream) {
      accumulator.addPartial(chunk);
    }

    const result = accumulator.computeResponse();
    if (content.length > 0) {
      assertContentPart(t, result.content, content);
    }
    history = [
      ...stagedInput.messages,
      {
        role: "assistant",
        content: result.content,
      },
    ];
    context.stages.push({
      assistant: structuredClone(result.content),
      tool_calls: structuredClone(getToolCallParts(result.content)),
    });
  }
}
