import type { Part } from "./types.ts";

/**
 * For providers that do not support document parts,
 * we translate them to compatible parts such as Text.
 * Inner Document parts are flattened.
 */
export function getCompatiblePartsWithoutDocumentParts(parts: Part[]): Part[] {
  return parts
    .map((part) => {
      if (part.type === "document") {
        return getCompatiblePartsWithoutDocumentParts(part.content);
      }
      return part;
    })
    .flat();
}
