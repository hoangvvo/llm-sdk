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

let current = await response.next();
while (!current.done) {
  console.dir(current.value, { depth: null });
  current = await response.next();
}
