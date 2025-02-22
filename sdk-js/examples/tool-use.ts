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
          tool_name: "trade",
          args: {
            action: "buy",
            quantity: 50,
            symbol: "NVDA",
          },
          tool_call_id: "1",
        },
      ],
    },
    {
      role: "tool",
      content: [
        {
          type: "tool-result",
          tool_call_id: "1",
          result: {
            status: "success",
          },
          tool_name: "trade",
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
