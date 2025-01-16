/* eslint-disable @typescript-eslint/no-floating-promises */
/* eslint-disable @typescript-eslint/no-non-null-assertion */
import test, { type TestContext } from "node:test";
import { StreamAccumulator } from "../src/accumulator.ts";
import type { LanguageModel } from "../src/language-model.ts";
import type {
  LanguageModelCapability,
  LanguageModelInput,
  Part,
} from "../src/types.ts";
import { transformCompatibleSchema } from "./utils.ts";

interface TextPartAssertion {
  type: "text";
  text: RegExp;
}

interface ToolCallPartAssertionArgProp {
  [key: string]: RegExp | ToolCallPartAssertionArgProp;
}

export type PartAssertion = TextPartAssertion | ToolCallPartAssertion;

export interface ToolCallPartAssertion {
  type: "tool_call";
  tool_name: string;
  args: ToolCallPartAssertionArgProp;
}

export interface TestCase {
  name: string;
  input: LanguageModelInput;
  type: "generate" | "stream";
  requiredCapabilities?: LanguageModelCapability[];
  output: {
    content: PartAssertion[];
  };
}

export interface RunTestCaseOptions {
  /**
   * For newer models with structured outputs, all properties are required
   * but they can be marked optional using type: [type, "null"]. However,
   * old model do not support that. To make the schema compatible with older models,
   * we turn the array type back into a single type and remove that property
   * from the required list.
   */
  compatibleSchema?: boolean;
}

export async function runTestCase(
  t: TestContext,
  model: LanguageModel,
  testCase: TestCase,
  options?: RunTestCaseOptions,
) {
  const { input, type, output } = testCase;
  const modelInput = { ...input };

  if (options?.compatibleSchema) {
    if (modelInput.tools) {
      modelInput.tools = modelInput.tools.map((tool) => {
        return {
          ...tool,
          parameters: transformCompatibleSchema(tool.parameters),
        };
      });
    }
    if (
      modelInput.response_format?.type === "json" &&
      modelInput.response_format.schema
    ) {
      modelInput.response_format.schema = transformCompatibleSchema(
        modelInput.response_format.schema,
      );
    }
  }

  if (type === "generate") {
    const result = await model.generate(modelInput);
    if (output.content.length > 0) {
      assertContentPart(t, result.content, output.content);
    }
  } else {
    const stream = model.stream(modelInput);
    const accumulator = new StreamAccumulator();

    for await (const chunk of stream) {
      accumulator.addPartial(chunk);
    }

    const result = accumulator.computeResponse();
    if (output.content.length > 0) {
      assertContentPart(t, result.content, output.content);
    }
  }
}
export interface RunTestOptions extends RunTestCaseOptions {
  /**
   * Delay per test case
   * Useful if testing using a free tier API with rate limits
   */
  delay?: number;
}

export function runTests(
  testCases: TestCase[],
  model: LanguageModel,
  options?: RunTestOptions,
) {
  for (const testCase of testCases) {
    if (testCase.requiredCapabilities) {
      const missingCapabilities = testCase.requiredCapabilities.filter(
        (cap) => !model.metadata?.capabilities?.includes(cap),
      );

      if (missingCapabilities.length > 0) {
        test(`${model.provider} / ${testCase.name} `, (t, done) => {
          t.skip(
            `Skipping test case due to missing capability: ${missingCapabilities.join(", ")}`,
          );
          done();
        });
        continue;
      }
    }

    test(`${model.provider} / ${testCase.name}`, async (t) => {
      if (options?.delay) {
        await new Promise((resolve) => setTimeout(resolve, options.delay));
      }
      return runTestCase(t, model, testCase, options);
    });
  }
}

export function assertContentPart(
  t: TestContext,
  content: Part[],
  assertions: PartAssertion[],
) {
  for (const assertion of assertions) {
    switch (assertion.type) {
      case "text": {
        assertTextPart(t, content, assertion);
        break;
      }
      case "tool_call": {
        assertToolCallPart(t, content, assertion);
        break;
      }
    }
  }
}

export function assertTextPart(
  t: TestContext,
  content: Part[],
  assertion: TextPartAssertion,
) {
  const foundPart = content.find(
    (part) => part.type === "text" && assertion.text.test(part.text),
  );
  t.assert.ok(
    foundPart,
    `Expected matching text part:
Expected: ${String(assertion.text)}
Received:
${JSON.stringify(content, null, 2)}`,
  );
}

export function assertToolCallPart(
  t: TestContext,
  content: Part[],
  assertion: ToolCallPartAssertion,
) {
  const foundPart = content.find(
    (part) =>
      part.type === "tool-call" &&
      part.tool_name === assertion.tool_name &&
      matchToolCallArgs(part.args, assertion.args),
  );
  t.assert.ok(
    foundPart,
    `Expected matching tool call part:
Expected tool ${assertion.tool_name} with args ${JSON.stringify(assertion.args)}
Received:
${JSON.stringify(content, null, 2)}`,
  );
}

function matchToolCallArgs(
  actual: Record<string, unknown>,
  expected: ToolCallPartAssertionArgProp,
): boolean {
  for (const expectedKey in expected) {
    const expectedValue = expected[expectedKey]!;
    const actualValue = actual[expectedKey];

    if (actualValue === undefined) {
      return false;
    }

    if (expectedValue instanceof RegExp) {
      if (
        !(
          typeof actualValue !== "object" &&
          // eslint-disable-next-line @typescript-eslint/no-base-to-string
          expectedValue.test(String(actualValue))
        )
      ) {
        return false;
      }
    } else {
      if (
        !(
          typeof actualValue === "object" &&
          matchToolCallArgs(
            actualValue as Record<string, unknown>,
            expectedValue,
          )
        )
      ) {
        return false;
      }
    }
  }

  return true;
}
