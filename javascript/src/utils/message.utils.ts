import type { Message } from "../schema/types.gen.js";

export function convertAudioPartsToTextParts<T extends Message>(message: T): T {
  return {
    ...message,
    content: message.content.map((part) => {
      if (part.type === "audio" && part.transcript) {
        return {
          type: "text",
          text: part.transcript,
        };
      }
      return part;
    }),
  };
}
