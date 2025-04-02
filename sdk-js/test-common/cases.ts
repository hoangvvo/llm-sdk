import * as fs from "node:fs";
import * as path from "node:path";
import { type TestContext } from "node:test";
import { StreamAccumulator } from "../src/accumulator.ts";
import type { LanguageModel } from "../src/language-model.ts";
import type { LanguageModelInput, Tool } from "../src/types.ts";
import { assertContentPart, type PartAssertion } from "./assert.ts";

export interface TestCase {
  name: string;
  input: LanguageModelInput;
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
  type: "generate" | "stream";
  input: LanguageModelInput;
  input_tools?: string[];
  output: {
    content: {
      type: "text" | "tool_call" | "audio" | "reasoning";
      text?: string;
      tool_name?: string;
      args?: Record<string, string>;
      audio_id?: boolean;
      transcript?: string;
    }[];
  };
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
function jsonToTestCase(jsonCase: TestCaseJSON): TestCase {
  const input: LanguageModelInput = { ...jsonCase.input };

  // Resolve tool references
  if (jsonCase.input_tools) {
    input.tools = resolveTools(jsonCase.input_tools);
  }

  // Convert output assertions - always treat text as regex
  const output = {
    content: jsonCase.output.content.map((part): PartAssertion => {
      if (part.type === "text" && part.text) {
        return {
          type: "text",
          text: new RegExp(part.text),
        } as PartAssertion;
      } else if (part.type === "tool_call" && part.tool_name) {
        return {
          type: "tool_call",
          tool_name: part.tool_name,
          args: part.args
            ? Object.entries(part.args).reduce<Record<string, RegExp>>(
                (acc, [key, value]) => {
                  if (typeof value === "string") {
                    acc[key] = new RegExp(value);
                  }
                  return acc;
                },
                {},
              )
            : {},
        } as PartAssertion;
      } else if (part.type === "audio") {
        return {
          type: "audio",
          audio_id: part.audio_id,
          transcript: part.transcript ? new RegExp(part.transcript) : undefined,
        } as PartAssertion;
      } else if (part.type === "reasoning" && part.text) {
        return {
          type: "reasoning",
          text: new RegExp(part.text),
        } as PartAssertion;
      }
      throw new Error(`Invalid part assertion in test case ${jsonCase.name}`);
    }),
  };

  return {
    name: jsonCase.name,
    input,
    type: jsonCase.type,
    output,
  };
}

// Load test cases from JSON
const testCasesFromJson = testData.test_cases.map(jsonToTestCase);

// Test case names for easy reference
export const TEST_CASE_NAMES = {
  GENERATE_TEXT: "generate_text",
  STREAM_TEXT: "stream_text",
  GENERATE_WITH_SYSTEM_PROMPT: "generate_with_system_prompt",
  GENERATE_TOOL_CALL: "generate_tool_call",
  STREAM_TOOL_CALL: "stream_tool_call",
  GENERATE_TEXT_FROM_TOOL_RESULT: "generate_text_from_tool_result",
  STREAM_TEXT_FROM_TOOL_RESULT: "stream_text_from_tool_result",
  GENERATE_PARALLEL_TOOL_CALLS: "generate_parallel_tool_calls",
  STREAM_PARALLEL_TOOL_CALLS: "stream_parallel_tool_calls",
  STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME:
    "stream_parallel_tool_calls_of_same_name",
  STRUCTURED_RESPONSE_FORMAT: "structured_response_format",
  SOURCE_PART_INPUT: "source_part_input",
  GENERATE_AUDIO: "generate_audio",
  STREAM_AUDIO: "stream_audio",
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

  const { input, type, output } = testCase;
  const modelInput = options?.additionalInputs
    ? options.additionalInputs(input)
    : input;

  const content = options?.customOutputContent
    ? options.customOutputContent(output.content)
    : output.content;

  if (type === "generate") {
    const result = await model.generate(modelInput);
    if (content.length > 0) {
      assertContentPart(t, result.content, content);
    }
  } else {
    const stream = model.stream(modelInput);
    const accumulator = new StreamAccumulator();

    for await (const chunk of stream) {
      accumulator.addPartial(chunk);
    }

    const result = accumulator.computeResponse();
    if (content.length > 0) {
      assertContentPart(t, result.content, content);
    }
  }
}
