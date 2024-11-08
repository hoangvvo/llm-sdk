/* eslint-disable @typescript-eslint/no-non-null-assertion */
/* eslint-disable @typescript-eslint/no-floating-promises */
import test from "node:test";
import {
  ContentDelta,
  LanguageModel,
  Message,
  ModelResponse,
  PartialModelResponse,
  TextPart,
  Tool,
} from "../src/index.js";

const interactive = !!process.env["INTERACTIVE"];

export const tools: Tool[] = [
  {
    name: "get_weather",
    description: "Get the weather",
    parameters: {
      type: "object",
      properties: {
        location: { type: "string" },
        unit: { type: "string", enum: ["c", "f"] },
      },
      required: ["location"],
    },
  },
  {
    name: "get_stock_price",
    description: "Get the stock price",
    parameters: {
      type: "object",
      properties: {
        symbol: { type: "string" },
      },
      required: ["symbol"],
    },
  },
];

/**
 * A tool with a complex parameter schema.
 */
export const complexTool: Tool = {
  name: "register_user",
  description: "Register a user",
  parameters: {
    type: "object",
    properties: {
      id: {
        type: "string",
        description: "Unique identifier in UUID format",
      },
      name: {
        type: "string",
        minLength: 2,
        maxLength: 50,
        description: "The name of the user, between 2 and 50 characters",
      },
      email: {
        type: "string",
        format: "email",
        description: "A valid email address",
      },
      birthDate: {
        type: "string",
        format: "date",
        description: "Date of birth in YYYY-MM-DD format",
      },
      age: {
        type: "integer",
        minimum: 0,
        maximum: 120,
        description: "Age of the user, must be between 0 and 120",
      },
      isActive: {
        type: "boolean",
        default: true,
        description: "Indicates if the account is active",
      },
      role: {
        type: "string",
        enum: ["user", "admin", "moderator"],
        description: "Role of the user in the system",
      },
      accountBalance: {
        type: "number",
        minimum: 0,
        description: "User's account balance, must be greater than 0",
      },
      phoneNumber: {
        type: "string",
        pattern: "^[+][0-9]{10,15}$",
        description: "Phone number in international format, e.g., +1234567890",
      },
      tags: {
        type: "array",
        items: {
          type: "string",
          minLength: 1,
          maxLength: 20,
        },
        uniqueItems: true,
        maxItems: 10,
        description: "An array of unique tags, each up to 20 characters long",
      },
      lastLogin: {
        type: "string",
        format: "date-time",
        description: "The last login date and time",
      },
    },
    required: ["id", "name", "email", "age", "isActive"],
    additionalProperties: false,
  },
};

export function testLanguageModel(languageModel: LanguageModel) {
  test("generate text", async (t) => {
    const response = await languageModel.generate({
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
    });

    log(response);

    const part = response.content[0];

    t.assert.equal(part?.type, "text");
    t.assert.equal((part as TextPart).text.length > 0, true);
  });

  test("stream text", async (t) => {
    const response = languageModel.stream({
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
    });

    let current = await response.next();
    while (!current.done) {
      log(current.value);
      t.assert.deepEqual(current.value.delta.part.type, "text");
      current = await response.next();
    }

    log(current.value);

    const part = current.value.content[0];

    t.assert.equal(part?.type, "text");
    t.assert.equal((part as TextPart).text.length > 0, true);
  });

  test("generate tool call", async (t) => {
    const response = await languageModel.generate({
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
      tools,
    });

    log(response);

    const toolCallPart = response.content.find(
      (part) => part.type === "tool-call",
    );

    t.assert.equal(toolCallPart!.toolCallId.length > 0, true);

    t.assert.equal(toolCallPart?.toolName, "get_weather");
    t.assert.equal(
      (toolCallPart?.args?.["location"] as string).includes("Boston"),
      true,
    );
  });

  test("stream tool call", async (t) => {
    const response = languageModel.stream({
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
      tools,
    });

    let current = await response.next();
    while (!current.done) {
      log(current.value);
      current = await response.next();
    }

    log(current.value);

    const toolCallPart = current.value.content.find(
      (part) => part.type === "tool-call",
    );

    t.assert.equal(
      !!toolCallPart?.toolCallId && toolCallPart.toolCallId.length > 0,
      true,
    );

    t.assert.equal(toolCallPart?.toolName, "get_weather");
    t.assert.equal(
      (toolCallPart?.args?.["location"] as string).includes("Boston"),
      true,
    );
  });

  test("generate text from tool result", async (t) => {
    const response = await languageModel.generate({
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
              toolCallId: "0mbnj08nt",
              toolName: "get_weather",
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
              toolCallId: "0mbnj08nt",
              toolName: "get_weather",
              result: {
                temperature: 70,
                unit: "f",
                description: "Sunny",
              },
            },
          ],
        },
      ],
      tools,
    });

    log(response);

    const part = response.content[0];

    t.assert.equal(part?.type, "text");
    t.assert.equal((part as TextPart).text.length > 0, true);
  });

  test("stream text from tool result", async (t) => {
    const response = languageModel.stream({
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
              toolCallId: "0mbnj08nt",
              toolName: "get_weather",
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
              toolCallId: "0mbnj08nt",
              toolName: "get_weather",
              result: {
                temperature: 70,
                unit: "f",
                description: "Sunny",
              },
            },
          ],
        },
      ],
      tools,
    });

    let current = await response.next();
    while (!current.done) {
      log(current.value);
      current = await response.next();
    }

    log(current.value);

    const part = current.value.content[0];

    t.assert.equal(part?.type, "text");
    t.assert.equal((part as TextPart).text.length > 0, true);
  });

  test("generate tool call for complex schema", async (t) => {
    const response = await languageModel.generate({
      messages: [
        {
          role: "user",
          content: [
            {
              type: "text",
              text: "Register a user. You must always fill in random details for all fields.",
            },
          ],
        },
      ],
      tools: [complexTool],
    });

    log(response);

    const toolCallPart = response.content.find(
      (part) => part.type === "tool-call",
    );

    t.assert.equal(
      !!toolCallPart?.toolCallId && toolCallPart.toolCallId.length > 0,
      true,
    );
    t.assert.equal(toolCallPart?.toolName, "register_user");
  });

  test("calculate usage and cost", async (t) => {
    const response = await languageModel.generate({
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
    });

    t.assert.equal(response.usage!.inputTokens > 0, true);
    t.assert.equal(response.usage!.outputTokens > 0, true);
    t.assert.equal(typeof response.cost, "number");
  });
}

export function testParallelToolCalls(languageModel: LanguageModel) {
  test("generate parallel tool calls", async (t) => {
    const response = await languageModel.generate({
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
      tools,
    });

    log(response);

    const toolCallParts = response.content.filter(
      (part) => part.type === "tool-call",
    );

    t.assert.equal(toolCallParts.length, 2);

    const weatherCall = toolCallParts.find(
      (part) => part.toolName === "get_weather",
    );

    t.assert.equal(
      (weatherCall?.args?.["location"] as string).includes("Boston"),
      true,
    );

    const stockCall = toolCallParts.find(
      (part) => part.toolName === "get_stock_price",
    );

    t.assert.equal(
      (stockCall?.args?.["symbol"] as string).includes("AAPL"),
      true,
    );
  });

  test("stream parallel tool calls", async (t) => {
    const response = languageModel.stream({
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
      tools,
    });

    let current = await response.next();
    while (!current.done) {
      log(current.value);
      current = await response.next();
    }

    log(current.value);

    const toolCallParts = current.value.content.filter(
      (part) => part.type === "tool-call",
    );

    t.assert.equal(toolCallParts.length, 2);

    const weatherCall = toolCallParts.find(
      (part) => part.toolName === "get_weather",
    );

    t.assert.equal(
      (weatherCall?.args?.["location"] as string).includes("Boston"),
      true,
    );

    const stockCall = toolCallParts.find(
      (part) => part.toolName === "get_stock_price",
    );

    t.assert.equal(
      (stockCall?.args?.["symbol"] as string).includes("AAPL"),
      true,
    );
  });
}

export function log(value: ModelResponse | PartialModelResponse) {
  if (!interactive) {
    return;
  }

  let obj: unknown;

  if ("content" in value) {
    obj = {
      ...value,
      content: value.content.map(cleanContentPartObjectForDisplay),
    };
  } else {
    obj = {
      ...value,
      delta: {
        ...value.delta,
        part: cleanContentPartObjectForDisplay(value.delta.part),
      },
    };
  }

  console.log(obj);
}

function cleanContentPartObjectForDisplay(
  part: Message["content"][number] | ContentDelta["part"],
) {
  switch (part.type) {
    case "audio": {
      return {
        ...part,
        audioData: part.audioData
          ? `<<${String(base64ByteLength(part.audioData))} bytes>>`
          : undefined,
      };
    }
    case "image":
      return {
        ...part,
        imageData: part.imageData
          ? `<<${String(base64ByteLength(part.imageData))} bytes>>`
          : undefined,
      };
    case "tool-result": {
      const toolResultStr = JSON.stringify(part.result);
      return {
        ...part,
        // put in same line to avoid scrolling
        result: toolResultStr,
      };
    }
    default:
      return part;
  }
}

function base64ByteLength(base64: string) {
  let padding = 0;
  if (base64.endsWith("==")) padding = 2;
  else if (base64.endsWith("=")) padding = 1;

  // Each base64 character represents 6 bits (3/4 of a byte).
  return (base64.length * 3) / 4 - padding;
}
