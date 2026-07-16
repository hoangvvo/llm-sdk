import type { JSONSchema } from "@hoangvvo/llm-sdk";
import { type Static, type TObject } from "typebox";
import { type AgentFunctionTool, type AgentToolResult } from "../tool.ts";
import type { RunState } from "../run.ts";

export function typeboxTool<TContext, TSchema extends TObject>(params: {
  /**
   * Name of the tool.
   */
  name: string;
  /**
   * A description of the tool to instruct the model how and when to use it.
   */
  description: string;
  /**
   * The JSON schema of the parameters that the tool accepts. The type must be "object".
   */
  parameters: TSchema;
  /**
   * The function that will be called to execute the tool with given parameters and context.
   *
   * If the tool throws an error, the agent will be interrupted and the error will be propagated.
   * To avoid interrupting the agent, the tool must return an `AgentToolResult` with `is_error` set to true.
   */
  execute(
    args: Static<TSchema>,
    context: TContext,
    state: RunState,
  ): AgentToolResult | Promise<AgentToolResult>;
}): AgentFunctionTool<TContext, Static<TSchema>> {
  return {
    type: "function",
    ...params,
    parameters: params.parameters as JSONSchema,
  };
}
