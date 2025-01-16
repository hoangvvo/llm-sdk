import type { Static, TSchema as TTypeboxSchema } from "@sinclair/typebox";
import { AgentTool, type AgentToolResult } from "../tool.ts";

export function typeboxTool<TSchema extends TTypeboxSchema, TContext>(params: {
  name: string;
  description: string;
  parameters: TSchema;
  execute(
    args: Static<TSchema>,
    context: TContext,
  ): AgentToolResult | Promise<AgentToolResult>;
}) {
  return new AgentTool(params);
}
