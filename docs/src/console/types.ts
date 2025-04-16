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
