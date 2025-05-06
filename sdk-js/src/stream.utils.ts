import { InvariantError } from "./errors.ts";
import type {
  ContentDelta,
  Part,
  PartDelta,
  ReasoningPartDelta,
  ToolCallPartDelta,
} from "./types.ts";

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
  // contentDeltas may have the structure of [part0 partial, part0 partial, part1 partial].
  // For the purpose of this matching, we want only [part0, part1]
  const uniqueContentDeltas = allContentDeltas.filter(
    (part, index) =>
      allContentDeltas.findIndex(
        (findPart) => findPart.index === part.index,
      ) === index,
  );

  if (part.type === "tool-call" && typeof toolCallIndex === "number") {
    // Providers like OpenAI track tool calls in a separate field, so we
    // need to reconcile that. To understand how this matching works:
    // [Provider]
    // toolCalls: [index 0] [index 1]
    // [LLM-SDK state]
    // parts: [index 0 text] [index 1 tool] [index 2 text] [index 3 tool]
    // In this case, we need to map the tool index 0 -> 1 and 1 -> 3
    const toolPartDeltas = uniqueContentDeltas.filter(
      (contentDelta) => contentDelta.part.type === "tool-call",
    );
    const existingToolCallDelta = toolPartDeltas[toolCallIndex];
    if (existingToolCallDelta) {
      return existingToolCallDelta.index;
    } else {
      // If no matching tool call delta found, return the length of unique_content_deltas
      // This is because we want to append a new tool call delta
      return uniqueContentDeltas.length;
    }
  }

  const matchingDelta = uniqueContentDeltas.findLast((contentDelta) => {
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
    ...uniqueContentDeltas.map((contentDelta) => contentDelta.index),
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
    case "tool-call": {
      const toolCall: ToolCallPartDelta = { type: "tool-call" };
      if (part.tool_call_id) {
        toolCall.tool_call_id = part.tool_call_id;
      }
      if (part.tool_name) {
        toolCall.tool_name = part.tool_name;
      }
      if (typeof part.args === "object") {
        toolCall.args = JSON.stringify(part.args);
      }
      if (part.id) {
        toolCall.id = part.id;
      }
      return toolCall;
    }
    case "reasoning": {
      const reasoning: ReasoningPartDelta = {
        type: "reasoning",
        text: part.text,
      };
      if (part.signature) {
        reasoning.signature = part.signature;
      }
      if (part.id) {
        reasoning.id = part.id;
      }
      return reasoning;
    }
    case "image":
      return part;
    default: {
      throw new InvariantError(
        "",
        `Cannot convert Part of type ${part.type} to PartDelta`,
      );
    }
  }
}
