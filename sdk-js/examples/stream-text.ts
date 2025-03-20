import { StreamAccumulator } from "@hoangvvo/llm-sdk";
import { getModel } from "./get-model.ts";

const model = getModel("openai", "gpt-4o");

const response = model.stream({
  messages: [
    {
      role: "user",
      content: [
        {
          type: "text",
          text: "Tell me a story.",
        },
      ],
    },
    {
      role: "assistant",
      content: [
        {
          type: "text",
          text: "What kind of story would you like to hear?",
        },
      ],
    },
    {
      role: "user",
      content: [
        {
          type: "text",
          text: "A fairy tale.",
        },
      ],
    },
  ],
});

const accumulator = new StreamAccumulator();

let current = await response.next();
while (!current.done) {
  console.dir(current.value, { depth: null });
  accumulator.addPartial(current.value);
  current = await response.next();
}

const finalResponse = accumulator.computeResponse();
console.dir(finalResponse, { depth: null });
