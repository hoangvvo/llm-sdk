import type { z, ZodType } from "zod";
import { zodToJsonSchema } from "zod-to-json-schema";
import { AgentTool, type AgentToolResult } from "../tool.ts";

export function zodTool<TZodSchema extends ZodType, TContext>(params: {
  name: string;
  description: string;
  parameters: TZodSchema;
  execute(
    args: z.infer<TZodSchema>,
    context: TContext,
  ): AgentToolResult | Promise<AgentToolResult>;
}) {
  return new AgentTool({
    ...params,
    parameters: zodToJsonSchema(params.parameters),
  });
}
