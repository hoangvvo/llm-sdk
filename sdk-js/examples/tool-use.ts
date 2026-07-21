import type {
  Message,
  ModelResponse,
  Tool,
  ToolMessage,
} from "@hoangvvo/llm-sdk";
import { getModel } from "./get-model.ts";

let MY_BALANCE = 1000;
const STOCK_PRICE = 100;

function trade({
  action,
  quantity,
  symbol,
}: {
  action: "buy" | "sell";
  quantity: number;
  symbol: string;
}) {
  console.log(
    `[TOOLS trade()] Trading ${String(quantity)} shares of ${symbol} with action: ${action}`,
  );
  const balanceChange =
    action === "buy" ? -quantity * STOCK_PRICE : quantity * STOCK_PRICE;

  MY_BALANCE += balanceChange;

  return {
    success: true,
    balance: MY_BALANCE,
    balance_change: balanceChange,
  };
}

let MAX_TURN_LEFT = 10;

const provider = process.env["PROVIDER"] ?? "openai";
const modelId = process.env["MODEL"] ?? "gpt-5.6-terra";
const model = getModel(provider, modelId);

const tools: Tool[] = [
  {
    type: "function",
    name: "trade",
    description: "Trade stocks",
    parameters: {
      type: "object",
      properties: {
        action: {
          type: "string",
          enum: ["buy", "sell"],
          description: "The action to perform",
        },
        quantity: {
          type: "number",
          description: "The number of stocks to trade",
        },
        symbol: {
          type: "string",
          description: "The stock symbol",
        },
      },
      required: ["action", "quantity", "symbol"],
      additionalProperties: false,
    },
  },
];

const messages: Message[] = [
  {
    role: "user",
    content: [
      {
        type: "text",
        text: "I would like to buy 50 NVDA stocks.",
      },
    ],
  },
];

let response: ModelResponse;

do {
  response = await model.generate({
    messages,
    tools,
  });

  messages.push({
    role: "assistant",
    content: response.content,
  });

  const toolCallParts = response.content.filter((c) => c.type === "tool-call");

  if (toolCallParts.length === 0) {
    break;
  }

  let toolMessage: ToolMessage | undefined;

  for (const toolCallPart of toolCallParts) {
    if (toolCallPart.call.type !== "function") continue;
    const { tool_call_id, call } = toolCallPart;
    const { name: toolName, args } = call;

    let toolResult;
    switch (toolName) {
      case "trade": {
        toolResult = trade(
          args as {
            action: "buy" | "sell";
            quantity: number;
            symbol: string;
          },
        );
        break;
      }
      default:
        throw new Error(`Tool ${toolName} not found`);
    }

    toolMessage = toolMessage ?? {
      role: "tool",
      content: [],
    };

    toolMessage.content.push({
      type: "tool-result",
      status: "completed",
      tool_call_id,
      result: {
        type: "function",
        name: toolName,
        content: [
          {
            type: "text",
            text: JSON.stringify(toolResult),
          },
        ],
      },
    });
  }

  if (toolMessage) messages.push(toolMessage);
} while (MAX_TURN_LEFT-- > 0);

console.dir(response, { depth: null });
