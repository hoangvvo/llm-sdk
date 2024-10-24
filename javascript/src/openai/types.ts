export interface OpenAIModelOptions {
  baseURL?: string;
  apiKey: string;
  modelId: string;
  /**
   * If true, use OpenAI new structured outputs feature available on new models.
   * https://platform.openai.com/docs/guides/structured-outputs
   */
  structuredOutputs?: boolean;
}

// documented type is wrong
export type OpenAIPatchedPromptTokensDetails = {
  cached_tokens: number;
  text_tokens: number;
  image_tokens: number;
  audio_tokens: number;
};

// documented type is wrong
export type OpenAIPatchedCompletionTokenDetails = {
  reasoning_tokens: 0;
  text_tokens: 63;
  audio_tokens: 286;
};
