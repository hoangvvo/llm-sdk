import { InvariantError } from "./errors.js";
import type { ContentDelta, Part, PartDelta } from "./types.js";

/**
 * Because of the difference in mapping, especially in `OpenAI` cases,
 * where text and audio part does not have indexes
 * or in Google cases, where no parts have indexes,
 * we need to guess an index for the incoming delta
 * which is required in our unified interface.
 */
export function guessDeltaIndex(
  part: PartDelta,
  allContentDeltas: ContentDelta[],
  existingMatchingDelta?: ContentDelta,
) {
  if (existingMatchingDelta) {
    return existingMatchingDelta.index;
  }

  const matchingDelta = allContentDeltas.findLast((contentDelta) => {
    if (part.type === "text" || part.type === "audio") {
      return contentDelta.part.type === part.type;
    }
    // we won't be able to reliably match tool calls
    // because there can be multiple tool calls with the same tool name
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
