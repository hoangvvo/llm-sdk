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
          result: null,
          toolName: "trade",
        },
      ],
    },
    {
      role: "user",
      content: [
        {
          type: "text",
          text: "What did you say?",
        },
      ],
    },
  ],
});

console.dir(response, { depth: null });
