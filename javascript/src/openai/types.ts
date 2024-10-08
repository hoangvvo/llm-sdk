export interface OpenAIModelOptions {
  baseURL?: string;
  apiKey: string;
  modelId: string;
  /**
   * If true, use OpenAI new structured outputs feature available on new models.
   * https://platform.openai.com/docs/guides/structured-outputs
   */
  structuredOutputs?: boolean;
  pricing?: {
    inputTokensCost: number;
    outputTokensCost: number;
  };
}
