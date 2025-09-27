import { zodTool } from "@hoangvvo/llm-agent/zod";
import { z } from "zod";
import type { MyContext } from "./context.ts";

export type ArtifactKind = "markdown" | "text" | "code";

export interface Artifact {
  id: string;
  title: string;
  kind: ArtifactKind;
  content: string;
  version?: number;
  updated_at?: string;
}

function findArtifact(ctx: MyContext, id: string): Artifact | undefined {
  const list = ctx.artifacts ?? [];
  return list.find((a) => a.id === id);
}

export const artifactCreateTool = zodTool({
  name: "artifact_create",
  description:
    "Create a new document and return an instruction for the client to persist it",
  parameters: z.object({
    title: z.string(),
    kind: z.enum(["markdown", "text", "code"]),
    content: z.string(),
  }),
  async execute({ title, kind, content }) {
    const now = new Date().toISOString();
    const id = Math.random().toString(36).slice(2, 11);
    const artifact: Artifact = {
      id,
      title,
      kind,
      content,
      version: 1,
      updated_at: now,
    };
    return {
      content: [
        {
          type: "text",
          text: JSON.stringify({ op: "artifact_create", artifact }),
        },
      ],
      is_error: false,
    };
  },
});

export const artifactUpdateTool = zodTool({
  name: "artifact_update",
  description:
    "Replace document content and return an instruction for the client to persist changes",
  parameters: z.object({ id: z.string(), content: z.string() }),
  async execute({ id, content }, ctx: MyContext) {
    const prev = findArtifact(ctx, id);
    const now = new Date().toISOString();
    const artifact: Artifact = {
      id,
      title: prev?.title ?? "Untitled",
      kind: prev?.kind ?? "markdown",
      content,
      version: (prev?.version ?? 0) + 1,
      updated_at: now,
    };
    return {
      content: [
        {
          type: "text",
          text: JSON.stringify({
            op: "artifact_update",
            id,
            prev_content: prev?.content ?? "",
            artifact,
          }),
        },
      ],
      is_error: false,
    };
  },
});

export const artifactGetTool = zodTool({
  name: "artifact_get",
  description: "Fetch a document from the current client context",
  parameters: z.object({ id: z.string() }),
  async execute({ id }, ctx: MyContext) {
    const artifact = findArtifact(ctx, id);
    return {
      content: [
        {
          type: "text",
          text: JSON.stringify({
            op: "artifact_get",
            id,
            artifact: artifact ?? null,
          }),
        },
      ],
      is_error: false,
    };
  },
});

export const artifactListTool = zodTool({
  name: "artifact_list",
  description: "List documents from the current client context",
  parameters: z.object({}).strict(),
  async execute(_args, ctx: MyContext) {
    const artifacts = ctx.artifacts ?? [];
    return {
      content: [
        {
          type: "text",
          text: JSON.stringify({ op: "artifact_list", artifacts }),
        },
      ],
      is_error: false,
    };
  },
});

export const artifactDeleteTool = zodTool({
  name: "artifact_delete",
  description: "Delete a document by id",
  parameters: z.object({ id: z.string() }),
  async execute({ id }) {
    return {
      content: [
        { type: "text", text: JSON.stringify({ op: "artifact_delete", id }) },
      ],
      is_error: false,
    };
  },
});

export function getArtifactTools() {
  return [
    artifactCreateTool,
    artifactUpdateTool,
    artifactGetTool,
    artifactListTool,
    artifactDeleteTool,
  ];
}
