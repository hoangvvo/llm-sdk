import type { AgentRequest } from "../../src/types.ts";
import type { MyContext } from "./context.ts";

export type McpServerConfig =
  | {
      type: "streamable-http";
      url: string;
      authorization?: string;
    }
  | {
      type: "stdio";
      command: string;
      args?: string[];
    };

export interface RunStreamBody {
  provider: string;
  model_id: string;
  input: AgentRequest<MyContext>;
  enabled_tools?: string[];
  mcp_servers?: McpServerConfig[];
  disabled_instructions?: boolean;
  temperature?: number;
  top_p?: number;
  top_k?: number;
  frequency_penalty?: number;
  presence_penalty?: number;
}
