// Requires ffplay (https://ffmpeg.org/) on PATH.
import { spawn, type ChildProcessByStdio } from "node:child_process";
import type Stream from "node:stream";
import { getModel } from "./get-model.ts";

const model = getModel("openai-chat-completion", "gpt-4o-audio-preview");

const stream = model.stream({
  modalities: ["text", "audio"],
  audio: {
    format: "linear16",
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

type Player = ChildProcessByStdio<Stream.Writable, null, null>;

let player: Player | undefined;
let sampleRate: number | undefined;
let channels: number | undefined;

for await (const partial of stream) {
  console.dir(redactAudioData(partial), { depth: null });

  const part = partial.delta?.part;
  if (part?.type !== "audio" || !part.data) continue;

  if (part.format && part.format !== "linear16") {
    throw new Error(`Unsupported audio format: ${part.format}`);
  }

  sampleRate ??= part.sample_rate ?? 24_000;
  channels ??= part.channels ?? 1;

  if (!player) {
    player = await startFfplay(sampleRate, channels);
    console.log(
      `Streaming audio with ffplay (${sampleRate} Hz, ${channels} channel${channels === 1 ? "" : "s"}).`,
    );
  }

  const currentPlayer = player;
  currentPlayer.stdin.write(Buffer.from(part.data, "base64"), (err) => {
    if (err) {
      console.error("Error writing to ffplay stdin:", err);
    }
  });
}

if (player) {
  await finishFfplay(player);
}

async function startFfplay(sampleRate: number, channels: number) {
  const child = spawn(
    "ffplay",
    [
      "-loglevel",
      "error",
      "-autoexit",
      "-nodisp",
      "-f",
      "s16le",
      "-ar",
      String(sampleRate),
      "-i",
      "pipe:0",
      "-af",
      `aformat=channel_layouts=${channels === 1 ? "mono" : "stereo"}`,
    ],
    { stdio: ["pipe", "ignore", "inherit"] },
  );

  await waitForSpawn(child);

  return child;
}

async function finishFfplay(child: Player) {
  if (child.stdin.writable) {
    child.stdin.end();
  }

  await new Promise<void>((resolve, reject) => {
    child.once("close", (code) => {
      if (code === 0) {
        resolve();
      } else {
        reject(new Error(`ffplay exited with code ${code}`));
      }
    });
    child.once("error", reject);
  });
}

function waitForSpawn(child: Player) {
  return new Promise<void>((resolve, reject) => {
    child.once("spawn", resolve);
    child.once("error", reject);
  });
}

function redactAudioData(partial: unknown) {
  if (!partial || typeof partial !== "object") {
    return partial;
  }

  return JSON.parse(
    JSON.stringify(partial, (_key, value) => {
      if (
        value &&
        typeof value === "object" &&
        "data" in value &&
        typeof value.data === "string"
      ) {
        const byteLength = Buffer.from(value.data, "base64").length;
        return { ...value, data: `[${byteLength} bytes]` };
      }
      return value;
    }),
  );
}
