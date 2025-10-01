import { getModel } from "./get-model.ts";
import { spawn } from "node:child_process";
import { unlink, writeFile } from "node:fs/promises";

const model = getModel("google", "gemini-2.0-flash-exp-image-generation");

console.log("Requesting image generation...");
const response = await model.generate({
  modalities: ["text", "image"],
  messages: [
    {
      role: "user",
      content: [
        {
          type: "text",
          text:
            "A bright, sunlit green hill with a single large, leafy tree, " +
            "fluffy clouds drifting across a deep blue sky, painted in the warm, " +
            "detailed, hand-painted style of a Studio Ghibli landscape, soft colors, " +
            "gentle light, and a sense of quiet wonder.",
        },
      ],
    },
  ],
});

const imagePart = response.content.find((msg) => msg.type === "image");

if (!imagePart) {
  throw new Error("Image part not found in response");
}

const fileName = `image.${imagePart.mime_type.split("/")[1] ?? "png"}`;

await writeFile(fileName, imagePart.data, { encoding: "base64" });
console.log(`Saved image to ${fileName}`);

launchFile(fileName);

await new Promise((resolve) => setTimeout(resolve, 5000));

void unlink(fileName);

console.log("Done.");

function launchFile(path: string) {
  const platform = process.platform;
  let command: [string, string[]] | undefined;

  if (platform === "darwin") {
    command = ["open", [path]];
  } else if (platform === "linux") {
    command = ["xdg-open", [path]];
  } else if (platform === "win32") {
    command = ["cmd", ["/C", "start", "", path]];
  }

  if (!command) {
    console.warn(`Open ${path} manually; unsupported platform: ${platform}`);
    return;
  }

  try {
    const child = spawn(command[0], command[1], {
      stdio: "ignore",
      detached: true,
    });
    child.unref();
  } catch (error) {
    console.warn(`Failed to open ${path}:`, error);
  }
}
