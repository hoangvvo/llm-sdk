import Speaker from "speaker";
import { getModel } from "./get-model.ts";

let speaker: Speaker | undefined;

const model = getModel("openai-chat-completion", "gpt-4o-audio-preview");

const response = model.stream({
  extra: {
    audio: {
      voice: "alloy",
      format: "pcm16",
    },
  },
  modalities: ["text", "audio"],
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

let current = await response.next();
while (!current.done) {
  console.dir(current.value, { depth: null });
  const part = current.value.delta?.part;
  if (part?.type === "audio") {
    if (part.audio_data) {
      speaker =
        speaker ??
        new Speaker({
          sampleRate: part.sample_rate ?? 24000,
          bitDepth: 16,
          channels: part.channels ?? 1,
        });
      speaker.write(Buffer.from(part.audio_data, "base64"));
    }
  }
  current = await response.next();
}
