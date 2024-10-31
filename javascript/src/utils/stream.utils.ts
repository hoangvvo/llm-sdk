import type { ContentDelta, ModelResponse } from "../schemas/index.js";
import {
  arrayBufferToBase64,
  base64ToArrayBuffer,
  mergeInt16Arrays,
} from "./audio.utils.js";

export function mergeContentDeltas(
  deltas: ContentDelta[],
  incomingDeltas: ContentDelta[],
) {
  const mergedDeltas = deltas.slice();
  for (const incomingDelta of incomingDeltas) {
    const existingDelta = mergedDeltas.find(
      (delta) => delta.index === incomingDelta.index,
    );
    if (existingDelta) {
      if (existingDelta.part.type !== incomingDelta.part.type) {
        throw new Error(
          `unexpected part ${incomingDelta.part.type} at index ${incomingDelta.index}. existing part has type ${incomingDelta.part.type}`,
        );
      }
      if (
        existingDelta.part.type === "text" &&
        incomingDelta.part.type === "text"
      ) {
        existingDelta.part.text += incomingDelta.part.text;
      } else if (
        existingDelta.part.type === "audio" &&
        incomingDelta.part.type === "audio"
      ) {
        if (incomingDelta.part.audioData) {
          if (incomingDelta.part.encoding !== "linear16") {
            throw new Error(
              `only linear16 encoding is supported for audio. got ${existingDelta.part.encoding}`,
            );
          }
          const existingDeltaBuffer = existingDelta.part.audioData
            ? base64ToArrayBuffer(existingDelta.part.audioData)
            : null;
          const incomingDeltaBuffer = base64ToArrayBuffer(
            incomingDelta.part.audioData,
          );

          const mergedAudioData = existingDeltaBuffer
            ? mergeInt16Arrays(existingDeltaBuffer, incomingDeltaBuffer)
            : incomingDeltaBuffer;

          existingDelta.part.encoding =
            incomingDelta.part.encoding ?? existingDelta.part.encoding;

          existingDelta.part.audioData = arrayBufferToBase64(mergedAudioData);
        }
      } else if (
        existingDelta.part.type === "tool-call" &&
        incomingDelta.part.type === "tool-call"
      ) {
        if (incomingDelta.part.toolName) {
          existingDelta.part.toolName = existingDelta.part.toolName || "";
          existingDelta.part.toolName += incomingDelta.part.toolName;
        }
        if (incomingDelta.part.toolCallId) {
          existingDelta.part.toolCallId = existingDelta.part.toolCallId || "";
          existingDelta.part.toolCallId += incomingDelta.part.toolCallId;
        }
        if (incomingDelta.part.args) {
          existingDelta.part.args = existingDelta.part.args || "";
          existingDelta.part.args += incomingDelta.part.args;
        }
      }
    } else {
      mergedDeltas.push({
        ...incomingDelta,
        part: { ...incomingDelta.part },
      });
    }
  }
  return mergedDeltas.sort((a, b) => a.index - b.index);
}

export function mapContentDeltas(
  deltas: ContentDelta[],
): ModelResponse["content"] {
  return deltas.map((delta) => {
    switch (delta.part.type) {
      case "text":
        return {
          type: "text",
          text: delta.part.text,
        };
      case "audio":
        if (!delta.part.audioData) {
          throw new Error(
            `missing audioData at index ${delta.index} for audio part. audioData: ${delta.part.audioData}`,
          );
        }
        if (!delta.part.encoding) {
          throw new Error(
            `missing encoding at index ${delta.index} for audio part. encoding: ${delta.part.encoding}`,
          );
        }
        return {
          type: "audio",
          audioData: delta.part.audioData,
          encoding: delta.part.encoding,
          ...(delta.part.sampleRate && { sampleRate: delta.part.sampleRate }),
          ...(delta.part.channels && { channels: delta.part.channels }),
          ...(delta.part.transcript && { transcript: delta.part.transcript }),
        };
      case "tool-call":
        if (!delta.part.toolCallId || !delta.part.toolName) {
          throw new Error(
            `missing toolCallId or toolName at index ${delta.index}. toolCallId: ${delta.part.toolCallId}, toolName: ${delta.part.toolName}`,
          );
        }
        return {
          type: "tool-call",
          toolCallId: delta.part.toolCallId,
          args: delta.part.args ? JSON.parse(delta.part.args) : null,
          toolName: delta.part.toolName,
        };
      default:
        throw new Error(
          `unexpected part ${(delta.part as { type: string }).type} at index ${delta.index}`,
        );
    }
  });
}
