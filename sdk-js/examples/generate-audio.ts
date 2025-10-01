// Requires ffplay (https://ffmpeg.org/) on PATH.
import { spawn, type ChildProcess } from "node:child_process";
import { getModel } from "./get-model.ts";

const model = getModel("openai-chat-completion", "gpt-4o-audio-preview");

const response = await model.generate({
  modalities: ["text", "audio"],
  audio: {
    format: "mp3",
    voice: "alloy",
  },
  messages: [
    {
      role: "user",
      content: [
        {
          type: "text",
          text: "Is a golden retriever a good family dog?",
        },
      ],
    },
  ],
});

console.dir(response, { depth: null });

const audioPart = response.content.find((part) => part.type === "audio");

if (!audioPart) {
  throw new Error("Audio part not found in response");
}

await play(Buffer.from(audioPart.data, "base64"));

async function play(audio: Buffer) {
  const player = spawn(
    "ffplay",
    ["-autoexit", "-nodisp", "-loglevel", "error", "-"],
    {
      stdio: ["pipe", "ignore", "inherit"],
    },
  );

  await waitForSpawn(player);

  const stdin = player.stdin;
  if (!stdin) {
    throw new Error("ffplay stdin unavailable");
  }

  stdin.end(audio);

  await new Promise<void>((resolve, reject) => {
    player.once("close", (code) => {
      if (code === 0) {
        resolve();
      } else {
        reject(new Error(`ffplay exited with code ${code}`));
      }
    });
    player.once("error", reject);
  });
}

function waitForSpawn(child: ChildProcess) {
  return new Promise<void>((resolve, reject) => {
    child.once("spawn", resolve);
    child.once("error", reject);
  });
}
