import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";
import {
  StreamableHTTPClientTransport,
  type StreamableHTTPClientTransportOptions,
} from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import type { Transport } from "@modelcontextprotocol/sdk/shared/transport.js";
import {
  ToolListChangedNotificationSchema,
  type Tool,
} from "@modelcontextprotocol/sdk/types.js";
import type { AgentTool } from "../tool.ts";
import type { Toolkit, ToolkitSession } from "../toolkit.ts";
import { convertMCPContentToParts, type MCPContent } from "./content.ts";
import type { MCPInit, MCPParams } from "./types.ts";

// ToolkitSession implementation backed by an MCP client; this lets the core
// toolkit primitive hydrate dynamic tools at run time.
export class MCPToolkitSession<TContext> implements ToolkitSession<TContext> {
  // Underlying MCP client instance for issuing protocol requests.
  #client: Client;
  // Transport selected for this session (stdio or streamable HTTP).
  #transport: Transport;

  // Captures errors raised during background tool discovery (e.g., when a tool list
  // change notification arrives after the server updates tools on demand) so we can surface them on demand.
  #toolListError: Error | null;

  // Latest set of tools exposed to the agent runtime.
  #tools: AgentTool<TContext>[] = [];

  // Prepare transport + client scaffolding for the session. The MCP handshake
  // is kicked off later in initialize so we stay aligned with the Toolkit
  // createSession lifecycle.
  constructor(params: MCPParams) {
    this.#client = new Client({
      name: params.type,
      version: "1.0.0",
    });

    switch (params.type) {
      case "stdio":
        this.#transport = new StdioClientTransport({
          command: params.command,
          ...(params.args && { args: params.args }),
        });
        break;
      case "streamable-http": {
        const url = new URL(params.url);
        const transportOptions:
          | StreamableHTTPClientTransportOptions
          | undefined = params.authorization
          ? {
              requestInit: {
                headers: {
                  // Caller supplies bearer token; OAuth negotiation is not handled here.
                  Authorization: params.authorization.startsWith("Bearer ")
                    ? params.authorization
                    : `Bearer ${params.authorization}`,
                },
              },
            }
          : undefined;
        this.#transport = new StreamableHTTPClientTransport(
          url,
          transportOptions,
        ) as Transport;
        break;
      }
    }

    this.#toolListError = null;
  }

  // MCP does not expose a static system prompt, so we return undefined to keep
  // the toolkit contract satisfied without adding instructions.
  getSystemPrompt(): string | undefined {
    return undefined;
  }
  // Surface the latest tool snapshot. If discovery failed asynchronously we
  // rethrow the cached error so the agent can fail fast during turn prep.
  getTools(): AgentTool<TContext>[] {
    if (this.#toolListError) {
      throw this.#toolListError;
    }
    return this.#tools;
  }
  // Tear down the MCP wiring once the toolkit session is released by the agent.
  async close(): Promise<void> {
    await this.#client.close();
    await this.#transport.close();
  }

  // Load remote tools and convert them into AgentTool objects understood by the
  // core agent loop.
  async #loadTools(): Promise<void> {
    try {
      const mcpTools: Tool[] = [];

      let cursor: string | undefined = undefined;
      do {
        const res = await this.#client.listTools(
          cursor ? { cursor } : undefined,
        );
        mcpTools.push(...res.tools);
        cursor = res.nextCursor ?? undefined;
      } while (cursor);

      this.#tools = mcpTools.map((mcpTool) => {
        return {
          name: mcpTool.name,
          parameters: mcpTool.inputSchema,
          description: mcpTool.description ?? "",
          execute: async (args) => {
            const res = await this.#client.callTool({
              name: mcpTool.name,
              arguments: args as Record<string, unknown>,
            });
            return {
              content: convertMCPContentToParts(res.content as MCPContent[]),
              is_error: Boolean(res.isError),
            };
          },
        };
      });

      this.#toolListError = null;
    } catch (error) {
      this.#toolListError = error as Error;
      throw error;
    }
  }

  // Connect to the MCP server, perform the initial tool sync, and subscribe to
  // tool change notifications so future updates flow back into this session.
  async initialize(): Promise<void> {
    this.#client.setNotificationHandler(
      ToolListChangedNotificationSchema,
      () => {
        void this.#loadTools().catch((err: unknown) => {
          this.#toolListError = err as Error;
        });
      },
    );

    await this.#client.connect(this.#transport);
    await this.#loadTools();
  }
}

// Build a Toolkit implementation that sources tools from MCP so agent sessions
// can hydrate dynamic tools on demand. The init function can inspect context to
// pick per-user transports or credentials.
export function mcpToolkit<TContext>(
  init: MCPInit<TContext>,
): Toolkit<TContext> {
  async function createMcpSession(
    context: TContext,
  ): Promise<ToolkitSession<TContext>> {
    const params = typeof init === "function" ? await init(context) : init;
    const session = new MCPToolkitSession<TContext>(params);
    await session.initialize();
    return session;
  }

  return { createSession: createMcpSession };
}
