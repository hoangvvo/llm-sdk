import {
  Agent,
  type AgentTool,
  type InstructionParam,
  type Toolkit,
} from "@hoangvvo/llm-agent";
import { mcpToolkit } from "@hoangvvo/llm-agent/mcp";
import type {
  AudioOptions,
  LanguageModel,
  Modality,
  ReasoningOptions,
  WebSearchTool,
} from "@hoangvvo/llm-sdk";
import { artifactsToolkit } from "./artifacts.tools.ts";
import type { MyContext } from "./context.ts";
import { getStockPriceTool } from "./finance.tools.ts";
import type { McpServerConfig } from "./types.ts";
import { getCoordinatesTool, getWeatherTool } from "./weather.tools.ts";

const instructions: InstructionParam<MyContext>[] = [
  `Answer in markdown format.
To access certain tools, the user may have to provide corresponding API keys in the context fields on the UI.`,
  (context) =>
    `The user name is ${context.name ?? "<not provided>"}.
The user location is ${context.location ?? "<not provided>"}.
The user speaks ${context.language ?? "<not provided>"} language.`,
  () => `The current date is ${new Date().toDateString()}.`,
];

export const availableTools: AgentTool<MyContext>[] = [
  getStockPriceTool,
  getCoordinatesTool,
  getWeatherTool,
];

export const availableToolkits = [
  {
    name: "artifacts",
    description: "Create and manage documents in the Artifacts pane",
    toolkit: artifactsToolkit,
  },
] satisfies {
  name: string;
  description: string;
  toolkit: Toolkit<MyContext>;
}[];

export interface AgentOptions {
  enabledTools?: string[];
  enabledToolkits?: string[];
  webSearch?: Omit<WebSearchTool, "type">;
  temperature?: number;
  top_p?: number;
  top_k?: number;
  frequency_penalty?: number;
  presence_penalty?: number;
  mcpServers?: McpServerConfig[];
  audio?: AudioOptions;
  reasoning?: ReasoningOptions;
  modalities?: Modality[];
}

// Create a new instance of Agent for the request
// In your application, you may want to reuse the same instance of Agent throughout the lifetime of your app
export function createAgent(
  model: LanguageModel,
  options?: AgentOptions,
): Agent<MyContext> {
  const {
    enabledTools,
    enabledToolkits,
    webSearch,
    mcpServers,
    ...agentParams
  } = options ?? {};

  const toolNameSet = enabledTools ? new Set(enabledTools) : null;
  const tools: AgentTool<MyContext>[] =
    toolNameSet === null
      ? [...availableTools]
      : availableTools.filter(
          (tool) => tool.type === "function" && toolNameSet.has(tool.name),
        );
  if (webSearch) {
    tools.push({ ...webSearch, type: "web_search" });
  }
  const toolkitNameSet = enabledToolkits ? new Set(enabledToolkits) : null;
  const toolkits = availableToolkits
    .filter(({ name }) => toolkitNameSet === null || toolkitNameSet.has(name))
    .map(({ toolkit }) => toolkit);
  toolkits.push(...createMcpToolkits(mcpServers));

  return new Agent<MyContext>({
    name: "MyAgent",
    instructions,
    model,
    tools,
    toolkits,
    max_turns: 5,
    ...agentParams,
  });
}

function createMcpToolkits(
  servers: McpServerConfig[] | undefined,
): Toolkit<MyContext>[] {
  if (!servers || servers.length === 0) {
    return [];
  }

  const toolkits: Toolkit<MyContext>[] = [];
  for (const server of servers) {
    if (!server) {
      continue;
    }
    if (server.type === "streamable-http") {
      const url = typeof server.url === "string" ? server.url.trim() : "";
      if (!url) {
        continue;
      }
      const authorization =
        typeof server.authorization === "string"
          ? server.authorization.trim()
          : undefined;

      toolkits.push(
        mcpToolkit<MyContext>(() =>
          authorization
            ? {
                type: "streamable-http",
                url,
                authorization,
              }
            : {
                type: "streamable-http",
                url,
              },
        ),
      );
      continue;
    }

    if (server.type === "stdio") {
      const command =
        typeof server.command === "string" ? server.command.trim() : "";
      if (!command) {
        continue;
      }
      const args = Array.isArray(server.args)
        ? server.args.map((arg) => arg.trim()).filter((arg) => arg.length > 0)
        : [];

      toolkits.push(
        mcpToolkit<MyContext>(() =>
          args.length > 0
            ? {
                type: "stdio",
                command,
                args,
              }
            : {
                type: "stdio",
                command,
              },
        ),
      );
    }
  }

  return toolkits;
}
