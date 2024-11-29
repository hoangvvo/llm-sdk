import type {
  AudioPartDelta,
  ContentDelta,
  ModelResponse,
  TextPartDelta,
  ToolCallPart,
  ToolCallPartDelta,
} from "../schema/index.js";
import {
  arrayBufferToBase64,
  base64ToArrayBuffer,
  mergeInt16Arrays,
} from "./audio.utils.js";

export type InternalAudioPartDelta = Omit<AudioPartDelta, "audioData"> & {
  audioData: ArrayBuffer[];
};

export interface InternalContentDelta {
  index: number;
  part: TextPartDelta | ToolCallPartDelta | InternalAudioPartDelta;
}

export class ContentDeltaAccumulator {
  deltas: InternalContentDelta[] = [];

  addChunks(incomingDeltas: ContentDelta[]) {
    for (const incomingDelta of incomingDeltas) {
      const existingDelta = this.deltas.find(
        (delta) => delta.index === incomingDelta.index,
      );

      if (existingDelta) {
        if (
          existingDelta.part.type === "text" &&
          incomingDelta.part.type === "text"
        ) {
          existingDelta.part.text += incomingDelta.part.text;
        } else if (
          existingDelta.part.type === "tool-call" &&
          incomingDelta.part.type === "tool-call"
        ) {
          if (incomingDelta.part.toolName) {
            existingDelta.part.toolName =
              (existingDelta.part.toolName || "") + incomingDelta.part.toolName;
          }
          if (incomingDelta.part.toolCallId) {
            existingDelta.part.toolCallId = incomingDelta.part.toolCallId;
          }
          if (incomingDelta.part.args) {
            existingDelta.part.args =
              (existingDelta.part.args || "") + incomingDelta.part.args;
          }
        } else if (
          existingDelta.part.type === "audio" &&
          incomingDelta.part.type === "audio"
        ) {
          if (incomingDelta.part.audioData) {
            const incomingAudioData = base64ToArrayBuffer(
              incomingDelta.part.audioData,
            );
            // keep an array of audioBuffer internally and concat at the end
            existingDelta.part.audioData.push(incomingAudioData);
          }
          if (incomingDelta.part.encoding) {
            existingDelta.part.encoding = incomingDelta.part.encoding;
          }
          if (incomingDelta.part.sampleRate) {
            existingDelta.part.sampleRate = incomingDelta.part.sampleRate;
          }
          if (incomingDelta.part.channels) {
            existingDelta.part.channels = incomingDelta.part.channels;
          }
          if (incomingDelta.part.transcript) {
            existingDelta.part.transcript =
              (existingDelta.part.transcript || "") +
              incomingDelta.part.transcript;
          }
        } else {
          throw new Error(
            `unexpected part at index ${String(incomingDelta.index)}. existing part has type ${existingDelta.part.type}, incoming part has type ${incomingDelta.part.type}`,
          );
        }
      } else {
        this.deltas.push({
          index: incomingDelta.index,
          part: {
            ...(incomingDelta.part.type === "audio"
              ? {
                  ...incomingDelta.part,
                  audioData: incomingDelta.part.audioData
                    ? [base64ToArrayBuffer(incomingDelta.part.audioData)]
                    : [],
                }
              : incomingDelta.part),
          },
        });
      }
    }
  }

  computeContent(): ModelResponse["content"] {
    return this.deltas.map((delta): ModelResponse["content"][number] => {
      switch (delta.part.type) {
        case "text":
          return {
            type: "text",
            text: delta.part.text,
          };
        case "tool-call":
          if (!delta.part.toolCallId || !delta.part.toolName) {
            throw new Error(
              `missing toolCallId or toolName at index ${String(delta.index)}. toolCallId: ${String(delta.part.toolCallId)}, toolName: ${String(delta.part.toolName)}`,
            );
          }
          return {
            type: "tool-call",
            toolCallId: delta.part.toolCallId,
            args: delta.part.args
              ? (JSON.parse(delta.part.args) as Record<string, unknown>)
              : null,
            toolName: delta.part.toolName,
          };
        case "audio": {
          if (delta.part.encoding !== "linear16") {
            throw new Error(
              `only linear16 encoding is supported for audio concatenation. encoding: ${String(delta.part.encoding)}`,
            );
          }
          const concatenatedAudioData = mergeInt16Arrays(delta.part.audioData);
          return {
            type: "audio",
            audioData: arrayBufferToBase64(concatenatedAudioData),
            encoding: delta.part.encoding,
            ...(delta.part.container && { container: delta.part.container }),
            ...(delta.part.sampleRate && { sampleRate: delta.part.sampleRate }),
            ...(delta.part.channels && { channels: delta.part.channels }),
            ...(delta.part.transcript && { transcript: delta.part.transcript }),
          };
        }
        default:
          throw new Error(
            `unexpected part ${(delta.part as { type: string }).type} at index ${String(delta.index)}`,
          );
      }
    });
  }
}

// Because of difference in mapping, especially in
// openai cases, where text and audio part does not have indexes
// or google cases, where no parts have indexes
// we need to guess the index of the incoming delta
export function guessDeltaIndex(
  part: ContentDelta["part"] | ToolCallPart,
  allContentDeltas: (ContentDelta | InternalContentDelta)[],
  existingMatchingDelta?: ContentDelta | InternalContentDelta,
) {
  let matchingDelta = existingMatchingDelta;
  if (!matchingDelta) {
    matchingDelta = allContentDeltas.findLast((contentDelta) => {
      if (part.type === "text" || part.type === "audio") {
        return contentDelta.part.type === part.type;
      }
      // we won't be able to reliably match tool calls
      // because there can be multiple tool calls with the same tool name
      return false;
    });
  }
  if (matchingDelta) {
    return matchingDelta.index;
  }
  const maxIndex = Math.max(
    ...allContentDeltas.map((contentDelta) => contentDelta.index),
    -1,
  );
  return maxIndex + 1;
}
