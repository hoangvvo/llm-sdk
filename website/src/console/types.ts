import type { LanguageModelMetadata } from "@hoangvvo/llm-sdk";

export interface LoggedEvent {
  id: string;
  direction: "client" | "server";
  name: string;
  timestamp: string;
  payload: unknown;
}

export interface ModelInfo {
  provider: string;
  model_id: string;
  metadata?: LanguageModelMetadata;
}

export type ApiKeys = Record<string, string | undefined>;

export interface MyContext {
  name?: string;
  location?: string;
  language?: string;
  geo_api_key?: string;
  tomorrow_api_key?: string;
  news_api_key?: string;
}

export interface ToolInfo {
  name: string;
  description?: string;
}

export interface AgentBehaviorSettings {
  temperature?: number;
  top_p?: number;
  top_k?: number;
  frequency_penalty?: number;
  presence_penalty?: number;
}
