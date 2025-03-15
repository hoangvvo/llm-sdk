import { getModel } from "./get-model.ts";

const model = getModel("openai", "o1");

const response = await model.generate({
  messages: [
    {
      role: "user",
      content: [
        {
          type: "text",
          text: `A car starts from rest and accelerates at a constant rate of 4 m/s^2 for 10 seconds.
1. What is the final velocity of the car after 10 seconds?
2. How far does the car travel in those 10 seconds?`,
        },
      ],
    },
  ],
  extra: {
    include: ["reasoning.encrypted_content"],
  },
});

console.dir(response, { depth: null });
