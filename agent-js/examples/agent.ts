import { Agent, tool, type AgentItem } from "@hoangvvo/llm-agent";
import { typeboxTool } from "@hoangvvo/llm-agent/typebox";
import { zodTool } from "@hoangvvo/llm-agent/zod";
import { Type } from "@sinclair/typebox";
import readline from "node:readline/promises";
import { z } from "zod";
import { getModel } from "./get-model.ts";

// Define the context interface that can be accessed in the instructions and tools
interface MyContext {
  userName: string;
}

// Define the model to use for the Agent
const model = getModel("openai", "gpt-4o");

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
    { additionalProperties: false },
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

const items: AgentItem[] = [];

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
  items.push({
    type: "message",
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
    input: items,
  });

  // Append items with the output items
  items.push(...response.output);

  console.dir(response, { depth: null });
}
