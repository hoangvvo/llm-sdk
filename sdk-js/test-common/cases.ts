import { test, type TestContext, type TestOptions } from "node:test";
import { StreamAccumulator } from "../src/accumulator.ts";
import type { LanguageModel } from "../src/language-model.ts";
import type { LanguageModelInput, Tool } from "../src/types.ts";
import { assertContentPart, type PartAssertion } from "./assert.ts";
import { transformCompatibleSchema } from "./utils.ts";

export interface TestCase {
  name: string;
  input: LanguageModelInput;
  type: "generate" | "stream";
  output: {
    content: PartAssertion[];
  };
}

const getWeatherTool: Tool = {
  name: "get_weather",
  description: "Get the weather",
  parameters: {
    type: "object",
    properties: {
      location: { type: "string" },
      unit: { type: ["string", "null"], enum: ["c", "f"] },
    },
    required: ["location", "unit"],
    additionalProperties: false,
  },
};

const getStockPriceTool: Tool = {
  name: "get_stock_price",
  description: "Get the stock price",
  parameters: {
    type: "object",
    properties: {
      symbol: { type: "string" },
    },
    required: ["symbol"],
    additionalProperties: false,
  },
};

export const TEST_CASE_GENERATE_TEXT: TestCase = {
  name: "generate text",
  input: {
    messages: [
      {
        role: "user",
        content: [
          {
            type: "text",
            text: `Respond by saying "Hello"`,
          },
        ],
      },
    ],
  },
  type: "generate",
  output: {
    content: [
      {
        type: "text",
        text: /Hello/,
      },
    ],
  },
};

export const TEST_CASE_STREAM_TEXT: TestCase = {
  name: "stream text",
  input: {
    messages: [
      {
        role: "user",
        content: [
          {
            type: "text",
            text: `Respond by saying "Hello"`,
          },
        ],
      },
    ],
  },
  type: "stream",
  output: {
    content: [
      {
        type: "text",
        text: /Hello/,
      },
    ],
  },
};

export const TEST_CASE_GENERATE_WITH_SYSTEM_PROMPT: TestCase = {
  name: "generate with system prompt",
  input: {
    messages: [
      {
        role: "user",
        content: [
          {
            type: "text",
            text: "Hello",
          },
        ],
      },
    ],
    system_prompt: 'You must always start your message with "🤖"',
  },
  type: "generate",
  output: {
    content: [
      {
        type: "text",
        text: /^🤖/,
      },
    ],
  },
};

export const TEST_CASE_GENERATE_TOOL_CALL: TestCase = {
  name: "generate tool call",
  input: {
    messages: [
      {
        role: "user",
        content: [
          {
            type: "text",
            text: "What's the weather like in Boston today?",
          },
        ],
      },
    ],
    tools: [getWeatherTool],
  },
  type: "generate",
  output: {
    content: [
      {
        type: "tool_call",
        tool_name: "get_weather",
        args: {
          location: /Boston/,
        },
      },
    ],
  },
};

export const TEST_CASE_STREAM_TOOL_CALL: TestCase = {
  name: "stream tool call",
  input: {
    messages: [
      {
        role: "user",
        content: [
          {
            type: "text",
            text: "What's the weather like in Boston today?",
          },
        ],
      },
    ],
    tools: [getWeatherTool],
  },
  type: "generate",
  output: {
    content: [
      {
        type: "tool_call",
        tool_name: "get_weather",
        args: {
          location: /Boston/,
        },
      },
    ],
  },
};

export const TEST_CASE_GENERATE_TEXT_FROM_TOOL_RESULT: TestCase = {
  name: "generate text from tool result",
  input: {
    messages: [
      {
        role: "user",
        content: [
          {
            type: "text",
            text: "What's the weather like in Boston today?",
          },
        ],
      },
      {
        role: "assistant",
        content: [
          {
            type: "tool-call",
            tool_call_id: "0mbnj08nt",
            tool_name: "get_weather",
            args: {
              location: "Boston",
            },
          },
        ],
      },
      {
        role: "tool",
        content: [
          {
            type: "tool-result",
            tool_call_id: "0mbnj08nt",
            tool_name: "get_weather",
            content: [
              {
                type: "text",
                text: JSON.stringify({
                  temperature: 70,
                  unit: "f",
                  description: "Sunny",
                }),
              },
            ],
          },
        ],
      },
    ],
    tools: [getWeatherTool],
  },
  type: "generate",
  output: {
    content: [
      {
        type: "text",
        text: /70.*sunny|sunny.*70/i,
      },
    ],
  },
};

export const TEST_CASE_STREAM_TEXT_FROM_TOOL_RESULT: TestCase = {
  name: "stream text from tool result",
  input: {
    messages: [
      {
        role: "user",
        content: [
          {
            type: "text",
            text: "What's the weather like in Boston today?",
          },
        ],
      },
      {
        role: "assistant",
        content: [
          {
            type: "tool-call",
            tool_call_id: "0mbnj08nt",
            tool_name: "get_weather",
            args: {
              location: "Boston",
            },
          },
        ],
      },
      {
        role: "tool",
        content: [
          {
            type: "tool-result",
            tool_call_id: "0mbnj08nt",
            tool_name: "get_weather",
            content: [
              {
                type: "text",
                text: JSON.stringify({
                  temperature: 70,
                  unit: "f",
                  description: "Sunny",
                }),
              },
            ],
          },
        ],
      },
    ],
    tools: [getWeatherTool],
  },
  type: "stream",
  output: {
    content: [
      {
        type: "text",
        text: /70.*sunny|sunny.*70/i,
      },
    ],
  },
};

export const TEST_CASE_GENERATE_PARALLEL_TOOL_CALLS: TestCase = {
  name: "generate parallel tool calls",
  input: {
    messages: [
      {
        role: "user",
        content: [
          {
            type: "text",
            text: "Get me the weather in Boston and the stock price of AAPL.",
          },
        ],
      },
    ],
    tools: [getWeatherTool, getStockPriceTool],
  },
  type: "generate",
  output: {
    content: [
      {
        type: "tool_call",
        tool_name: "get_weather",
        args: {
          location: /Boston/,
        },
      },
      {
        type: "tool_call",
        tool_name: "get_stock_price",
        args: {
          symbol: /AAPL/,
        },
      },
    ],
  },
};

export const TEST_CASE_STREAM_PARALLEL_TOOL_CALLS: TestCase = {
  name: "stream parallel tool calls",
  input: {
    messages: [
      {
        role: "user",
        content: [
          {
            type: "text",
            text: "Get me the weather in Boston and the stock price of AAPL. You must do both of them in one go.",
          },
        ],
      },
    ],
    tools: [getWeatherTool, getStockPriceTool],
  },
  type: "generate",
  output: {
    content: [
      {
        type: "tool_call",
        tool_name: "get_weather",
        args: {
          location: /Boston/,
        },
      },
      {
        type: "tool_call",
        tool_name: "get_stock_price",
        args: {
          symbol: /AAPL/,
        },
      },
    ],
  },
};

export const TEST_CASE_STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME: TestCase = {
  name: "stream parallel tool calls of same name",
  input: {
    messages: [
      {
        role: "user",
        content: [
          {
            type: "text",
            text: "Get me the weather in Boston and the weather in New York.",
          },
        ],
      },
    ],
    tools: [getWeatherTool],
  },
  type: "stream",
  output: {
    content: [
      {
        type: "tool_call",
        tool_name: "get_weather",
        args: {
          location: /Boston/,
        },
      },
      {
        type: "tool_call",
        tool_name: "get_weather",
        args: {
          location: /New York/,
        },
      },
    ],
  },
};

export const TEST_CASE_STRUCTURED_RESPONSE_FORMAT: TestCase = {
  name: "structured response format",
  input: {
    messages: [
      {
        role: "user",
        content: [
          {
            type: "text",
            text: 'Create a user with the id "a1b2c3", name "John Doe", email "john.doe@example.com", birthDate "1990-05-15", age 34, isActive true, role "user", accountBalance 500.75, phoneNumber "+1234567890123", tags ["developer", "gamer"], and lastLogin "2024-11-09T10:30:00Z".',
          },
        ],
      },
    ],
    response_format: {
      type: "json",
      name: "user",
      schema: {
        type: "object",
        properties: {
          id: { type: "string" },
          name: { type: "string" },
          email: { type: "string" },
          birthDate: { type: "string" },
          age: { type: "integer" },
          isActive: { type: "boolean" },
          role: { type: "string" },
          accountBalance: { type: "number" },
          phoneNumber: { type: "string" },
          tags: { type: "array", items: { type: "string" } },
          lastLogin: { type: "string" },
        },
        required: [
          "id",
          "name",
          "email",
          "birthDate",
          "age",
          "isActive",
          "role",
          "accountBalance",
          "phoneNumber",
          "tags",
          "lastLogin",
        ],
        additionalProperties: false,
      },
    },
  },
  type: "generate",
  output: {
    content: [
      {
        type: "text",
        text: /"id"\s*:\s*"a1b2c3"/,
      },
      {
        type: "text",
        text: /"name"\s*:\s*"John Doe"/,
      },
      {
        type: "text",
        text: /"email"\s*:\s*"john\.doe@example\.com"/,
      },
    ],
  },
};

export const TEST_CASE_DOCUMENT_PART_INPUT: TestCase = {
  // all providers must accept the document part or translate them to a compatible part
  name: "document part in content",
  input: {
    messages: [
      {
        role: "user",
        content: [
          {
            type: "document",
            title: "my secret number",
            content: [
              {
                type: "text",
                text: 'Remember that my secret number is "42".',
              },
            ],
          },
          {
            type: "text",
            text: "What is the my secret number?",
          },
        ],
      },
    ],
  },
  output: {
    content: [
      {
        type: "text",
        text: /42/,
      },
    ],
  },
  type: "generate",
};

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

export function testTestCase(
  model: LanguageModel,
  testCase: TestCase,
  options?: RunTestCaseOptions,
  testOptions?: TestOptions,
) {
  return test(testCase.name, testOptions, async (t) => {
    return runTestCase(t, model, testCase, options);
  });
}
