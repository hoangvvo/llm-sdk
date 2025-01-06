import audioContext from "audio-context";
import decodeAudio from "audio-decode";
import play from "audio-play";
import { getModel } from "./get-model.ts";

const model = getModel("openai", "gpt-4o-audio-preview");

const response = await model.generate({
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
    Buffer.from(audioPart.audio_data, "base64"),
  );
  const playback = play(
    audioBuffer,
    { context: audioContext } as unknown as play.Options,
    () => {
      console.log("Playback finished");
    },
  );
  playback.play();
}
