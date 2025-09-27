import type {
  AudioOptions,
  LanguageModelMetadata,
  Modality,
  ReasoningOptions,
} from "@hoangvvo/llm-sdk";
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
  metadata?: LanguageModelMetadata;
  input: AgentRequest<MyContext>;
  enabled_tools?: string[];
  mcp_servers?: McpServerConfig[];
  temperature?: number;
  top_p?: number;
  top_k?: number;
  frequency_penalty?: number;
  presence_penalty?: number;
  audio?: AudioOptions;
  reasoning?: ReasoningOptions;
  modalities?: Modality[];
}
