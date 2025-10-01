import { getModel } from "./get-model.ts";

const imageUrl = "https://images.unsplash.com/photo-1464809142576-df63ca4ed7f0";
const imageRes = await fetch(imageUrl);

const image = await imageRes.arrayBuffer();

const model = getModel("openai", "gpt-4o");

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
          data: Buffer.from(image).toString("base64"),
          mime_type: imageRes.headers.get("content-type") ?? "image/jpeg",
        },
      ],
    },
  ],
});

console.dir(response, { depth: null });
