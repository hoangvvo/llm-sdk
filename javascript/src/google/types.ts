export interface GoogleModelOptions {
  apiKey: string;
  modelId: string;
  pricing?: {
    inputTokensCost: number;
    outputTokensCost: number;
  };
}
