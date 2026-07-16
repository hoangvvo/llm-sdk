import type {
  AudioOptions,
  LanguageModelMetadata,
  Modality,
  ReasoningOptions,
  WebSearchTool,
} from "@hoangvvo/llm-sdk";

export interface LoggedEvent {
  id: string;
  direction: "console" | "agent";
  name: string;
  timestamp: string;
  payload: unknown;
}

export type ApiKeys = Record<string, string | undefined>;

export interface Artifact {
  id: string;
  title: string;
  kind: "markdown" | "text" | "code";
  content: string;
  version?: number;
  updated_at?: string;
}

export interface MyContext {
  name?: string;
  location?: string;
  language?: string;
  geo_api_key?: string;
  tomorrow_api_key?: string;
  artifacts?: Artifact[];
}

export interface ToolInfo {
  name: string;
  description?: string;
  providers?: string[];
}

export interface ToolkitInfo {
  name: string;
  description?: string;
}

export function getCredentialProvider(provider: string): string {
  return provider === "openai-chat-completion" ? "openai" : provider;
}

export type WebSearchSettings = Omit<WebSearchTool, "type"> & {
  enabled: boolean;
};

export const WEB_SEARCH_PROVIDERS = ["openai", "google", "anthropic"];
export const WEB_SEARCH_OPTIONS_PROVIDERS = ["openai", "anthropic"];

export interface McpServerConfig {
  type: "streamable-http";
  url: string;
  authorization?: string;
}

export interface AgentBehaviorSettings {
  temperature?: number;
  top_p?: number;
  top_k?: number;
  frequency_penalty?: number;
  presence_penalty?: number;
}

export interface ModelInfo {
  provider: string;
  model_id: string;
  metadata: LanguageModelMetadata;
  default_modalities?: Modality[];
  default_reasoning?: ReasoningOptions;
  default_audio?: AudioOptions;
}
