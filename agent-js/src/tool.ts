import type { JSONSchema, Part } from "@hoangvvo/llm-sdk";
import type { RunState } from "./run.ts";

/**
 * Agent tool that can be used by the agent to perform specific tasks. Any object
 * that implements the `AgentTool` interface can be used as a tool.
 */
export interface AgentTool<TArgs extends Record<string, unknown>, TContext> {
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
  parameters: JSONSchema;
  /**
   * The function that will be called to execute the tool with given parameters and context.
   *
   * If the tool throws an error, the agent will be interrupted and the error will be propagated.
   * To avoid interrupting the agent, the tool must return an `AgentToolResult` with `is_error` set to true.
   */
  execute: (
    args: TArgs,
    ctx: TContext,
    state: RunState,
  ) => AgentToolResult | Promise<AgentToolResult>;
}

export interface AgentToolResult {
  content: Part[];
  is_error: boolean;
}

/**
 * A helper function to create an agent tool.
 */
export function tool<TArgs extends Record<string, unknown>, TContext>(params: {
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
  parameters: JSONSchema;
  /**
   * The function that will be called to execute the tool with given parameters and context.
   *
   * If the tool throws an error, the agent will be interrupted and the error will be propagated.
   * To avoid interrupting the agent, the tool must return an `AgentToolResult` with `is_error` set to true.
   */
  execute: (
    args: TArgs,
    ctx: TContext,
  ) => AgentToolResult | Promise<AgentToolResult>;
}): AgentTool<TArgs, TContext> {
  return params;
}
