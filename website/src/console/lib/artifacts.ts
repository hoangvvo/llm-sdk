import type { Part } from "@hoangvvo/llm-sdk";
import type { Artifact, MyContext } from "../types.ts";

// Reduce tool output parts into updated context.artifacts
// In an actual implementation, you may want to persist changes to a database
export function reduceArtifactsFromToolParts(
  prev: MyContext,
  parts: Part[],
): MyContext {
  let next: MyContext = prev;
  for (const p of parts) {
    if (p.type !== "text" || typeof p.text !== "string") continue;
    try {
      const payload = JSON.parse(p.text) as { op?: string } & Record<
        string,
        unknown
      >;
      if (typeof payload.op !== "string") continue;
      const artifacts = [...(next.artifacts ?? [])];
      switch (payload.op) {
        case "artifact_create": {
          const a = payload.artifact as Artifact | undefined;
          if (!a || typeof a.id !== "string") break;
          const idx = artifacts.findIndex((x) => x.id === a.id);
          if (idx >= 0) artifacts[idx] = a;
          else artifacts.push(a);
          next = { ...next, artifacts };
          break;
        }
        case "artifact_update": {
          const a = payload.artifact as Artifact | undefined;
          if (!a || typeof a.id !== "string") break;
          const idx = artifacts.findIndex((x) => x.id === a.id);
          if (idx >= 0) artifacts[idx] = a;
          else artifacts.push(a);
          next = { ...next, artifacts };
          break;
        }
        case "artifact_delete": {
          const id = payload.id as string;
          if (typeof id !== "string") break;
          const filtered = artifacts.filter((x) => x.id !== id);
          next = { ...next, artifacts: filtered };
          break;
        }
        case "artifact_list": {
          const list = (payload.artifacts as Artifact[] | undefined) ?? [];
          next = { ...next, artifacts: list };
          break;
        }
        default:
          break;
      }
    } catch {
      // ignore parse errors
    }
  }
  return next;
}
