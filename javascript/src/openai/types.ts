export interface OpenAIModelOptions {
  baseURL?: string;
  apiKey: string;
  modelId: string;
  /**
   * If true, use OpenAI new structured outputs feature available on new models.
   * https://platform.openai.com/docs/guides/structured-outputs
   */
  structuredOutputs?: boolean;
  /**
   * If true, for assistant audio parts, convert them to text parts if they have a transcript.
   *
   * For Audio objects, OpenAI provides message.audio.id that we can send back
   * https://platform.openai.com/docs/guides/audio#multi-turn-conversations
   *
   * However, we do not support the id field, and the audio id expires in 4 hours anyway.
   *
   * Therefore, we can convert the audio to text and send it back to OpenAI instead.
   * Setting this option to "false" currently throws an error if there is an assistant audio part.
   */
  convertAudioPartsToTextParts?: boolean;
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
  reasoning_tokens: number;
  text_tokens: number;
  audio_tokens: number;
};
