import type {
  AudioPartDelta,
  ContentDelta,
  ModelResponse,
  TextPartDelta,
  ToolCallPart,
  ToolCallPartDelta,
} from "../types.js";
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
        if (incomingDelta.part.id) {
          existingDelta.part.id = incomingDelta.part.id;
        }

        if (
          existingDelta.part.type === "text" &&
          incomingDelta.part.type === "text"
        ) {
          existingDelta.part.text += incomingDelta.part.text;
        } else if (
          existingDelta.part.type === "tool-call" &&
          incomingDelta.part.type === "tool-call"
        ) {
          if (incomingDelta.part.tool_name) {
            existingDelta.part.tool_name =
              (existingDelta.part.tool_name || "") +
              incomingDelta.part.tool_name;
          }
          if (incomingDelta.part.tool_call_id) {
            existingDelta.part.tool_call_id = incomingDelta.part.tool_call_id;
          }
          if (incomingDelta.part.args) {
            existingDelta.part.args =
              (existingDelta.part.args || "") + incomingDelta.part.args;
          }
        } else if (
          existingDelta.part.type === "audio" &&
          incomingDelta.part.type === "audio"
        ) {
          if (incomingDelta.part.audio_data) {
            const incomingAudioData = base64ToArrayBuffer(
              incomingDelta.part.audio_data,
            );
            // keep an array of audioBuffer internally and concat at the end
            existingDelta.part.audioData.push(incomingAudioData);
          }
          if (incomingDelta.part.encoding) {
            existingDelta.part.encoding = incomingDelta.part.encoding;
          }
          if (incomingDelta.part.sample_rate) {
            existingDelta.part.sample_rate = incomingDelta.part.sample_rate;
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
                  audioData: incomingDelta.part.audio_data
                    ? [base64ToArrayBuffer(incomingDelta.part.audio_data)]
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
            ...(delta.part.id && { id: delta.part.id }),
            type: "text",
            text: delta.part.text,
          };
        case "tool-call":
          if (!delta.part.tool_call_id || !delta.part.tool_name) {
            throw new Error(
              `missing tool_call_id or tool_name at index ${String(delta.index)}. tool_call_id: ${String(delta.part.tool_call_id)}, tool_name: ${String(delta.part.tool_name)}`,
            );
          }
          return {
            type: "tool-call",
            ...(delta.part.id && { id: delta.part.id }),
            tool_call_id: delta.part.tool_call_id,
            args: delta.part.args
              ? (JSON.parse(delta.part.args) as Record<string, unknown>)
              : null,
            tool_name: delta.part.tool_name,
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
            ...(delta.part.id && { id: delta.part.id }),
            audio_data: arrayBufferToBase64(concatenatedAudioData),
            encoding: delta.part.encoding,
            ...(delta.part.container && { container: delta.part.container }),
            ...(delta.part.sample_rate && {
              sample_rate: delta.part.sample_rate,
            }),
            ...(delta.part.channels && { channels: delta.part.channels }),
            ...(delta.part.transcript && { transcript: delta.part.transcript }),
          };
        }
        default: {
          const exhaustiveCheck: never = delta.part;
          throw new Error(
            `unexpected part ${String(exhaustiveCheck)} at index ${String(delta.index)}`,
          );
        }
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
