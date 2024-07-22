import { openaiModel as model } from "./model.js";

const response = await model.generate({
  messages: [
    {
      role: "user",
      content: [
        {
          type: "text",
          text: "I would like to buy 50 NVDA stocks.",
        },
      ],
    },
    {
      role: "assistant",
      content: [
        {
          type: "tool-call",
          toolName: "trade",
          args: {
            action: "buy",
            quantity: 50,
            symbol: "NVDA",
          },
          toolCallId: "1",
        },
      ],
    },
    {
      role: "tool",
      content: [
        {
          type: "tool-result",
          toolCallId: "1",
          result: {
            status: "success",
          },
          toolName: "trade",
        },
      ],
    },
  ],
  tools: [
    {
      name: "trade",
      description: "Trade stocks",
      parameters: {
        type: "object",
        properties: {
          action: {
            type: "string",
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
      },
    },
  ],
});

console.dir(response, { depth: null });
