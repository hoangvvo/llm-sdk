import type { Part } from "./types.ts";

/**
 * For providers that do not support source parts,
 * we translate them to compatible parts such as Text.
 * Inner source parts are flattened.
 */
export function getCompatiblePartsWithoutSourceParts(parts: Part[]): Part[] {
  return parts
    .map((part) => {
      if (part.type === "source") {
        return getCompatiblePartsWithoutSourceParts(part.content);
      }
      return part;
    })
    .flat();
}
