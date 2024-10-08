export interface AnthropicModelOptions {
  baseURL?: string;
  apiKey: string;
  modelId: string;
  pricing?: {
    inputTokensCost: number;
    outputTokensCost: number;
  };
}
