import { getModel } from "./get-model.ts";

const audioUrl = "https://archive.org/download/MLKDream/MLKDream.ogg";
const audioRes = await fetch(audioUrl);

const audio = await audioRes.arrayBuffer();

const model = getModel("google", "gemini-3.1-flash-lite-preview");

const response = await model.generate({
  messages: [
    {
      role: "user",
      content: [
        {
          type: "text",
          text: "What is this speech about?",
        },
        {
          type: "audio",
          data: Buffer.from(audio).toString("base64"),
          format: "opus",
        },
      ],
    },
  ],
});

console.dir(response, { depth: null });
