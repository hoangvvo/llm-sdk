import audioContext from "audio-context";
import decodeAudio from "audio-decode";
import play from "audio-play";
import { openaiAudioModel } from "./model.js";

const response = await openaiAudioModel.generate({
  extra: {
    audio: {
      voice: "alloy",
      format: "mp3",
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

console.dir(response, { depth: null });

const audioPart = response.content.find((part) => part.type === "audio");

if (audioPart) {
  const audioBuffer = await decodeAudio(
    Buffer.from(audioPart.audioData, "base64"),
  );
  const playback = play(
    audioBuffer,
    {
      // @ts-expect-error: it works ok?
      context: audioContext,
    },
    () => {},
  );
  playback.play();
}
