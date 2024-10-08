import { openaiModel as model } from "./model.js";

const response = await model.stream({
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

let current = await response.next();
while (!current.done) {
  current = await response.next();
  console.dir(current.value, { depth: null });
}

console.dir(current.value, { depth: null });
