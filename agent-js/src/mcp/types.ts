// Either a fixed MCP config or a function that derives params from agent context
// (e.g., look up user-specific credentials).
export type MCPInit<TContext> =
  | MCPParams
  | ((context: TContext) => MCPParams | Promise<MCPParams>);

export type MCPParams = MCPStdioParams | MCPStreamableHTTPParams;

export interface MCPStdioParams {
  type: "stdio";
  // Executable that implements the MCP server.
  command: string;
  // Optional arguments passed to the command.
  args?: string[];
}

export interface MCPStreamableHTTPParams {
  type: "streamable-http";
  // Base URL for the MCP server.
  url: string;
  // Authorization header value; OAuth2 flows are not handled automatically so
  // callers must provide a token when required.
  authorization?: string;
}
