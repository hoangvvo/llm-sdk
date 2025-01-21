# @hoangvvo/llm-agent

A JavaScript library to implement LLM agents that work with any LLM providers.

## Installation

```bash
npm install @hoangvvo/llm-agent
```

## Usage

```typescript
import { Agent, tool } from "@hoangvvo/llm-agent";
import { typeboxTool } from "@hoangvvo/llm-agent/typebox";
import { zodTool } from "@hoangvvo/llm-agent/zod";
import type { Message } from "@hoangvvo/llm-sdk";
import { OpenAIModel } from "@hoangvvo/llm-sdk/openai";
import { Type } from "@sinclair/typebox";
import assert from "node:assert";
import readline from "node:readline/promises";
import { z } from "zod";

// Define the context interface that can be accessed in the instructions and tools
interface MyContext {
  userName: string;
}

// Define the model to use for the Agent
assert(process.env["OPENAI_API_KEY"], "OPENAI_API_KEY must be set");
const model = new OpenAIModel({
  apiKey: process.env["OPENAI_API_KEY"],
  modelId: "gpt-4o",
});

// Define the agent tools
const getTimeTool = tool({
  name: "get_time",
  description: "Get the current time",
  parameters: {
    type: "object",
    properties: {},
    additionalProperties: false,
  },
  execute() {
    return {
      content: [
        {
          type: "text",
          text: JSON.stringify({
            current_time: new Date().toISOString(),
          }),
        },
      ],
      is_error: false,
    };
  },
});

// Create an agent tool using @sinclair/typebox with type inference
// npm install @sinclair/typebox
const getWeatherTool = typeboxTool({
  name: "get_weather",
  description: "Get weather for a given city",
  parameters: Type.Object(
    {
      city: Type.String({ description: "The name of the city" }),
    },
    { additionalProperties: false }
  ),
  execute(params) {
    // inferred as { city: string }
    const { city } = params;
    console.log(`Getting weather for ${city}`);
    return {
      content: [
        {
          type: "text",
          text: JSON.stringify({
            city,
            forecast: "Sunny",
            temperatureC: 25,
          }),
        },
      ],
      is_error: false,
    };
  },
});

// Create an agent tool using zod with type inference
// npm install zod zod-to-json-schema
const sendMessageTool = zodTool({
  name: "send_message",
  description: "Send a text message",
  parameters: z.object({
    message: z.string().min(1).max(500),
    phoneNumber: z.string(),
  }),
  execute(params) {
    // inferred as { message: string, phoneNumber: string }
    const { message, phoneNumber } = params;
    console.log(`Sending message to ${phoneNumber}: ${message}`);
    return {
      content: [
        {
          type: "text",
          text: JSON.stringify({
            success: true,
          }),
        },
      ],
      is_error: false,
    };
  },
});

// Create the Agent
const myAssistant = new Agent<MyContext>({
  name: "Mai",
  model,
  instructions: [
    "You are Mai, a helpful assistant. Answer questions to the best of your ability.",
    // Dynamic instruction
    (context) => `You are talking to ${context.userName}.`,
  ],
  tools: [getTimeTool, getWeatherTool, sendMessageTool],
});

// Implement the CLI to interact with the Agent
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
});

const userName = await rl.question("Your name: ");

const context: MyContext = {
  userName,
};

console.log(`Type 'exit' to quit`);

const messages: Message[] = [];

let userInput = "";

while (userInput !== "exit") {
  userInput = (await rl.question("> ")).trim();
  if (!userInput) {
    continue;
  }

  if (userInput.toLowerCase() === "exit") {
    break;
  }

  // Add user message as the input
  messages.push({
    role: "user",
    content: [
      {
        type: "text",
        text: userInput,
      },
    ],
  });

  // Call assistant
  const response = await myAssistant.run({
    context,
    messages,
  });

  // Update messages with the new items
  messages.push(...response.items);

  console.dir(response, { depth: null });
}
```

Find examples in the [examples](./examples/) folder:

- [Assistant CLI Example](./examples/assistant.ts)
- [Structured Output](./examples/structured-output.ts)
- [Multi-agent Delegation](./examples/agents-delegation.ts)

```bash
node --env-file=../.env examples/assistant.ts
```

## License

[MIT](https://github.com/hoangvvo/llm-sdk/blob/main/LICENSE)
