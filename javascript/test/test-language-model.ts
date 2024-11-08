/* eslint-disable @typescript-eslint/no-non-null-assertion */
/* eslint-disable @typescript-eslint/no-floating-promises */
import { TestContext } from "node:test";
import { LanguageModel, TextPart, Tool } from "../src/index.js";

export interface LanguageModelTest {
  name: string;
  fn: (t: TestContext) => Promise<void> | void;
}

const tools: Tool[] = [
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
const complexTool: Tool = {
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

export function getLanguageModelTests(
  languageModel: LanguageModel,
): LanguageModelTest[] {
  return [
    {
      name: "generate text",
      fn: async (t) => {
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

        console.log(response);

        const part = response.content[0];

        t.assert.equal(part?.type, "text");
        t.assert.equal((part as TextPart).text.length > 0, true);
      },
    },
    {
      name: "stream text",
      fn: async (t) => {
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
          console.log(current.value);
          t.assert.deepEqual(current.value.delta.part.type, "text");
          current = await response.next();
        }

        console.log(current.value);

        const part = current.value.content[0];

        t.assert.equal(part?.type, "text");
        t.assert.equal((part as TextPart).text.length > 0, true);
      },
    },
    {
      name: "generate tool call",
      fn: async (t) => {
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

        console.log(response);

        const toolCallPart = response.content.find(
          (part) => part.type === "tool-call",
        );

        t.assert.equal(toolCallPart!.toolCallId.length > 0, true);

        t.assert.equal(toolCallPart?.toolName, "get_weather");
        t.assert.equal(
          (toolCallPart?.args?.["location"] as string).includes("Boston"),
          true,
        );
      },
    },
    {
      name: "stream tool call",
      fn: async (t) => {
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
          console.log(current.value);
          current = await response.next();
        }

        console.log(current.value);

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
      },
    },
    {
      name: "generate text from tool result",
      fn: async (t) => {
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
                  toolCallId: "call-0",
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
                  toolCallId: "call-0",
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

        console.log(response);

        const part = response.content[0];

        t.assert.equal(part?.type, "text");
        t.assert.equal((part as TextPart).text.length > 0, true);
      },
    },
    {
      name: "stream text from tool result",
      fn: async (t) => {
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
                  toolCallId: "call-0",
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
                  toolCallId: "call-0",
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
          console.log(current.value);
          current = await response.next();
        }

        console.log(current.value);

        const part = current.value.content[0];

        t.assert.equal(part?.type, "text");
        t.assert.equal((part as TextPart).text.length > 0, true);
      },
    },
    {
      name: "generate text from complex tool result",
      fn: async (t) => {
        const response = await languageModel.generate({
          messages: [
            {
              role: "user",
              content: [
                {
                  type: "text",
                  text: "I would like to register. You must fill in random details. Do it anyway.",
                },
              ],
            },
          ],
          tools: [complexTool],
        });

        console.log(response);

        const toolCallPart = response.content.find(
          (part) => part.type === "tool-call",
        );

        t.assert.equal(
          !!toolCallPart?.toolCallId && toolCallPart.toolCallId.length > 0,
          true,
        );
        t.assert.equal(toolCallPart?.toolName, "register_user");
      },
    },
  ];
}

export function getAudioLanguageModelTests(
  languageModel: LanguageModel,
): LanguageModelTest[] {
  return [
    {
      name: "generate audio",
      fn: async (t) => {
        const response = await languageModel.generate({
          modalities: ["text", "audio"],
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
          extra: {
            audio: {
              voice: "alloy",
              format: "mp3",
            },
          },
        });

        console.log(response);

        const audioPart = response.content.find(
          (part) => part.type === "audio",
        );

        t.assert.equal(!!audioPart, true);
        t.assert.equal(audioPart?.type, "audio");
        t.assert.equal(audioPart!.audioData.length > 0, true);
        t.assert.equal(audioPart!.encoding!.length > 0, true);
        t.assert.equal(audioPart!.transcript!.length > 0, true);
      },
    },
    {
      name: "stream audio",
      fn: async (t) => {
        const response = languageModel.stream({
          modalities: ["text", "audio"],
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
          extra: {
            audio: {
              voice: "alloy",
              format: "pcm16",
            },
          },
        });

        let current = await response.next();
        while (!current.done) {
          console.log(current.value);
          current = await response.next();
        }

        console.log(current.value);

        const audioPart = current.value.content.find(
          (part) => part.type === "audio",
        );

        t.assert.equal(!!audioPart, true);
        t.assert.equal(audioPart?.type, "audio");
        t.assert.equal(audioPart!.audioData.length > 0, true);
        t.assert.equal(audioPart!.encoding!.length > 0, true);
        t.assert.equal(audioPart!.transcript!.length > 0, true);
      },
    },
  ];
}
