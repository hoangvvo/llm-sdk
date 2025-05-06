import type { AgentResponse } from "./types.ts";

/**
 * Extracts all text content from the final content, separated by a space.
 */
export function getResponseText(response: AgentResponse): string {
  return response.content
    .map((part) => (part.type === "text" ? part.text : undefined))
    .filter((text) => !!text)
    .join(" ");
}
