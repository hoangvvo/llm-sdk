import type { Tool } from "../src/types.ts";
import type { TestCase } from "./assert.ts";

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

export const COMMON_TEST_CASES: TestCase[] = [
  {
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
  },
  {
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
  },
  {
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
  },
  {
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
  },
  {
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
  },
  {
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
  },
  {
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
  },
  {
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
  },
  {
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
  },
  {
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
  },
  {
    name: "structured response format",
    requiredCapabilities: ["structured-output"],
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
  },
];
