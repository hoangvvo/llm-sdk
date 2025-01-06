import { InvariantError } from "./errors.ts";
import type { ContentDelta, Part, PartDelta } from "./types.ts";

/**
 * Because of the difference in mapping, especially in `OpenAI` cases,
 * where text and audio part does not have indexes
 * or in Google cases, where no parts have indexes,
 * we need to guess an index for the incoming delta
 * which is required in our unified interface.
 *
 * toolCallIndex does not always correspond to the index of the tool call in the deltas
 * because some providers keep tool call separate from other parts (e.g openai). We
 * can match this against the existing tool call deltas
 */
export function guessDeltaIndex(
  part: PartDelta,
  allContentDeltas: ContentDelta[],
  toolCallIndex?: number,
) {
  if (part.type === "tool-call" && typeof toolCallIndex === "number") {
    const toolPartDeltas = allContentDeltas.filter(
      (contentDelta) => contentDelta.part.type === "tool-call",
    );
    const existingToolCallDelta = toolPartDeltas[toolCallIndex];
    if (existingToolCallDelta) {
      return existingToolCallDelta.index;
    }
  }

  const matchingDelta = allContentDeltas.findLast((contentDelta) => {
    // For text and audio parts, they are the matching delta
    // if their types are the same. This is because providers that do not
    // provide indexes like only have 1 part for each type (e.g openai has only 1 message.content or 1 message.audio)
    if (part.type === "text" || part.type === "audio") {
      return contentDelta.part.type === part.type;
    }
    // For tool calls, we can't reliably match them
    // because there can be multiple tool calls with the same tool name
    // Different types don't match
    return false;
  });

  if (matchingDelta) {
    return matchingDelta.index;
  }

  const maxIndex = Math.max(
    ...allContentDeltas.map((contentDelta) => contentDelta.index),
    -1,
  );
  return maxIndex + 1;
}

export function looselyConvertPartToPartDelta(part: Part): PartDelta {
  switch (part.type) {
    case "text":
      return part;
    case "audio":
      return part;
    case "tool-call":
      return {
        type: "tool-call",
        ...(part.tool_call_id && { tool_call_id: part.tool_call_id }),
        ...(part.tool_name && { tool_name: part.tool_name }),
        ...(typeof part.args === "object" && {
          args: JSON.stringify(part.args),
        }),
        ...(part.id && { id: part.id }),
      };
    default: {
      throw new InvariantError(
        `Cannot convert Part of type ${part.type} to PartDelta`,
      );
    }
  }
}
