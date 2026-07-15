import type { JSONSchema, Part, WebSearchTool } from "@hoangvvo/llm-sdk";
import type { RunState } from "./run.ts";

/**
 * Agent function tool that can be executed by the agent runtime.
 */
export interface AgentFunctionTool<
  TContext,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  TArgs extends Record<string, unknown> = any,
> {
  /**
   * Discriminator for agent-executed function tools.
   */
  type: "function";
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

/**
 * Agent tool available to the model. This can either be an agent-executed
 * function tool or a provider-hosted web search tool.
 */
export type AgentTool<
  TContext,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  TArgs extends Record<string, unknown> = any,
> = AgentFunctionTool<TContext, TArgs> | WebSearchTool;

export interface AgentToolResult {
  content: Part[];
  is_error: boolean;
}

/**
 * A helper function to create an agent tool.
 */
export function tool<TContext, TArgs extends Record<string, unknown>>(params: {
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
}): AgentFunctionTool<TContext, TArgs> {
  return {
    type: "function",
    ...params,
  };
}
