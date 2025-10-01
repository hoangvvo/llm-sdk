import { type TestContext } from "node:test";
import type { Part } from "../src/types.ts";

interface TextPartAssertion {
  type: "text";
  text: RegExp;
}

interface ToolCallPartAssertionArgProp {
  [key: string]: RegExp | ToolCallPartAssertionArgProp;
}

export type PartAssertion =
  | TextPartAssertion
  | ToolCallPartAssertion
  | ReasoningPartAssertion
  | AudioPartAssertion;

export interface ToolCallPartAssertion {
  type: "tool_call";
  tool_name: string;
  args: ToolCallPartAssertionArgProp;
}

export interface AudioPartAssertion {
  type: "audio";
  id?: boolean;
  transcript?: RegExp | undefined;
}

export interface ReasoningPartAssertion {
  type: "reasoning";
  text: RegExp;
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
      case "audio": {
        assertAudioPart(t, content, assertion);
        break;
      }
      case "reasoning": {
        assertReasoningPart(t, content, assertion);
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

export function assertAudioPart(
  t: TestContext,
  content: Part[],
  assertion: AudioPartAssertion,
) {
  const foundPart = content.find((part) => {
    if (part.type !== "audio") {
      return false;
    }
    if (!part.data) {
      return false;
    }
    if (assertion.id && !part.id) {
      return false;
    }
    if (assertion.transcript && !assertion.transcript.test(part.transcript!)) {
      return false;
    }
    return true;
  });
  t.assert.ok(
    foundPart,
    `Expected matching audio part:
Expected: ${JSON.stringify(assertion)}
Received:
${JSON.stringify(content, null, 2)}`,
  );
}

export function assertReasoningPart(
  t: TestContext,
  content: Part[],
  assertion: ReasoningPartAssertion,
) {
  const foundPart = content.find(
    (part) => part.type === "reasoning" && assertion.text.test(part.text),
  );
  t.assert.ok(
    foundPart,
    `Expected matching reasoning part:
Expected text ${String(assertion.text)}
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
