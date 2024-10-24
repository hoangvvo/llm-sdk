import Speaker from "speaker";
import { openaiAudioModel } from "./model.js";

let speaker: Speaker | undefined;

const response = await openaiAudioModel.stream({
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
  const part = current.value.delta.part;
  if (part.type === "audio") {
    if (part.audioData) {
      speaker =
        speaker ||
        new Speaker({
          sampleRate: part.sampleRate || 24000,
          bitDepth: 16,
          channels: part.channels || 1,
        });
      speaker.write(Buffer.from(part.audioData, "base64"));
    }
    if (part.transcript) {
      console.log(part.transcript);
    }
  }
  current = await response.next();
}

console.dir(current.value, { depth: null });
const audioPart = current.value.content.find((part) => part.type === "audio");

if (audioPart) {
  // will repeat that one more time
  speaker?.write(Buffer.from(audioPart.audioData, "base64"));
}
