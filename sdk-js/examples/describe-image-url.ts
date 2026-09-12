import { getModel } from "./get-model.ts";

const provider = process.env["PROVIDER"] ?? "openai";
const modelId = process.env["MODEL"] ?? "gpt-5.6-terra";
const model = getModel(provider, modelId);

const response = await model.generate({
  messages: [
    {
      role: "user",
      content: [
        {
          type: "text",
          text: "Describe this image",
        },
        {
          type: "image",
          url: "https://images.unsplash.com/photo-1464809142576-df63ca4ed7f0",
          mime_type: "image/jpeg",
        },
      ],
    },
  ],
});

console.dir(response, { depth: null });
