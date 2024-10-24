import { openaiAudioModel } from "./model.js";

const response = await openaiAudioModel.generate({
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

console.dir(response, { depth: null });
