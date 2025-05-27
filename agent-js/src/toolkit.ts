import type { AgentTool } from "./tool.ts";

/**
 * Toolkit produces a per-session toolkit session that can provide dynamic prompt and tool data.
 */
export interface Toolkit<TContext> {
  /**
   * Create a new toolkit session for the supplied context value.
   * The function should also intialize the ToolkitSession instance with instructions and tools.
   */
  createSession(context: TContext): Promise<ToolkitSession<TContext>>;
}

/**
 * ToolkitSession exposes dynamically resolved tools and system prompt data for a run session.
 */
export interface ToolkitSession<TContext> {
  /**
   * Retrieve the current system prompt for the session.
   */
  getSystemPrompt(): string | undefined;
  /**
   * Retrieve the current set of tools that should be available to the session.
   */
  getTools(): AgentTool<TContext>[];
  /**
   * Release any resources that were allocated for the session.
   */
  close(): Promise<void> | void;
}
