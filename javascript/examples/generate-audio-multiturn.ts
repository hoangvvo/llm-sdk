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

const response2 = await openaiAudioModel.generate({
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
    {
      role: "assistant",
      content: response.content,
    },
    {
      role: "user",
      content: [
        {
          type: "text",
          text: "What about a labrador?",
        },
      ],
    },
  ],
});

console.dir(response2, { depth: null });

const audioPart2 = response2.content.find((part) => part.type === "audio");

if (audioPart2) {
  const audioBuffer2 = await decodeAudio(
    Buffer.from(audioPart2.audioData, "base64"),
  );
  const playback2 = play(
    audioBuffer2,
    {
      // @ts-expect-error: it works ok?
      context: audioContext,
    },
    () => {},
  );
  playback2.play();
}
