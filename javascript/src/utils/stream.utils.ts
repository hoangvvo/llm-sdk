import type { ContentDelta, ModelResponse } from "../schemas/index.js";

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
          `unexpected part ${existingDelta.part.type} at index ${existingDelta.index}`,
        );
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
    if (delta.part.type === "text") {
      return {
        type: "text",
        text: delta.part.text,
      };
    } else if (delta.part.type === "tool-call") {
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
    }
    throw new Error(
      `unexpected part ${(delta.part as { type: string }).type} at index ${delta.index}`,
    );
  });
}
