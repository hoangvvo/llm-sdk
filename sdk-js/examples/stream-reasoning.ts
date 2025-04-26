import { StreamAccumulator } from "@hoangvvo/llm-sdk";
import { getModel } from "./get-model.ts";

const model = getModel("openai", "o1");

const stream = model.stream({
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
  reasoning: {
    enabled: true,
  },
});

const accumulator = new StreamAccumulator();

let current = await stream.next();
while (!current.done) {
  if (current.value.delta?.part.type === "reasoning") {
    console.log("Reasoning:");
    console.dir(current.value.delta.part, { depth: null });
  } else if (current.value.delta) {
    console.log("Answer:");
    console.dir(current.value.delta.part, { depth: null });
  }
  accumulator.addPartial(current.value);
  current = await stream.next();
}

const finalResponse = accumulator.computeResponse();
console.dir(finalResponse, { depth: null });
