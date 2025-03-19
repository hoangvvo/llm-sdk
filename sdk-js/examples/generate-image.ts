import { unlink, writeFile } from "node:fs/promises";
import open from "open";
import { getModel } from "./get-model.ts";

const model = getModel("google", "gemini-2.0-flash-exp-image-generation");

const response = await model.generate({
  modalities: ["text", "image"],
  messages: [
    {
      role: "user",
      content: [
        {
          type: "text",
          text: "Generate an image of a sunset over the ocean",
        },
      ],
    },
  ],
});

console.dir(response, { depth: null });

const imagePart = response.content.find((msg) => msg.type === "image");

if (!imagePart) {
  throw new Error("Image part not found");
}

const fileName = `sunset.${imagePart.mime_type.split("/")[1] ?? "png"}`;

await writeFile(fileName, imagePart.image_data, { encoding: "base64" });

await open(fileName);

setTimeout(() => {
  // Cleanup
  void unlink(fileName);
}, 5000);
