import type { z, ZodType } from "zod";
import { zodToJsonSchema } from "zod-to-json-schema";
import { type AgentToolResult } from "../tool.ts";

export function zodTool<TContext, TZodSchema extends ZodType>(params: {
  /**
   * Name of the tool.
   */
  name: string;
  /**
   * A description of the tool to instruct the model how and when to use it.
   */
  description: string;
  /**
   * Zod schema of the parameters that the tool accepts. It must be a Zod object schema.
   */
  parameters: TZodSchema;
  /**
   * The function that will be called to execute the tool with given parameters and context.
   *
   * If the tool throws an error, the agent will be interrupted and the error will be propagated.
   * To avoid interrupting the agent, the tool must return an `AgentToolResult` with `is_error` set to true.
   */
  execute(
    args: z.infer<TZodSchema>,
    context: TContext,
  ): AgentToolResult | Promise<AgentToolResult>;
}) {
  return {
    ...params,
    parameters: zodToJsonSchema(params.parameters, {
      target: "openAi",
    }),
  };
}
