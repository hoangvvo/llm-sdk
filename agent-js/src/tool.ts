import type { JSONSchema, Part } from "@hoangvvo/llm-sdk";

export class AgentTool<TArgs extends Record<string, unknown> | null, TContext> {
  /**
   * Name of the tool.
   * The name can only contain letters and underscores.
   */
  name: string;
  /**
   * A description of the tool.
   */
  description: string;
  /**
   * The JSON schema of the parameters that the tool accepts. The type must be "object".
   */
  parameters: JSONSchema;
  /**
   * The function that will be called to execute the tool with given parameters and context.
   */
  execute: (
    args: TArgs,
    ctx: TContext,
  ) => AgentToolResult | Promise<AgentToolResult>;

  constructor(params: AgentToolParams<TArgs, TContext>) {
    this.name = params.name;
    this.description = params.description;
    this.parameters = params.parameters;
    this.execute = params.execute;

    this.validate();
  }

  validate() {
    // Validate tool name
    if (!/^[a-zA-Z_]+$/.exec(this.name)) {
      throw new Error(
        `Invalid tool name: ${this.name}. It can only contain letters and underscores.`,
      );
    }
  }
}

export interface AgentToolResult {
  content: Part[];
  is_error: boolean;
}

/**
 * Parameters required to create an agent tool
 */
export interface AgentToolParams<
  TArgs extends Record<string, unknown> | null,
  TContext,
> {
  /**
   * Name of the tool.
   * The name can only contain letters and underscores.
   */
  name: string;
  /**
   * A description of the tool.
   */
  description: string;
  /**
   * The JSON schema of the parameters that the tool accepts. The type must be "object".
   */
  parameters: JSONSchema;
  /**
   * The function that will be called to execute the tool with given parameters and context.
   */
  execute: (
    args: TArgs,
    ctx: TContext,
  ) => AgentToolResult | Promise<AgentToolResult>;
}

/**
 * A helper function to create an agent tool.
 */
export function tool<TArgs extends Record<string, unknown> | null, TContext>(
  params: AgentToolParams<TArgs, TContext>,
): AgentTool<TArgs, TContext> {
  return new AgentTool(params);
}
