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

const model = getModel("openai", "gpt-4o");

const tools: Tool[] = [
  {
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
    const { tool_call_id, tool_name, args } = toolCallPart;

    let toolResult;
    switch (tool_name) {
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
        throw new Error(`Tool ${tool_name} not found`);
    }

    toolMessage = toolMessage ?? {
      role: "tool",
      content: [],
    };

    toolMessage.content.push({
      type: "tool-result",
      tool_name,
      tool_call_id,
      content: [
        {
          type: "text",
          text: JSON.stringify(toolResult),
        },
      ],
    });
  }

  if (toolMessage) messages.push(toolMessage);
} while (MAX_TURN_LEFT-- > 0);

console.dir(response, { depth: null });
