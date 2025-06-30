import type { Part } from "@hoangvvo/llm-sdk";

/**
 * Extracts all text content from the final content, separated by a space.
 */
export function getContentText({ content }: { content: Part[] }): string {
  return content
    .map((part) => (part.type === "text" ? part.text : undefined))
    .filter((text) => !!text)
    .join(" ");
}
