// documented type is wrong
export interface OpenAIPatchedPromptTokensDetails {
  cached_tokens?: number;
  text_tokens?: number;
  image_tokens?: number;
  audio_tokens?: number;
  // only found in realtime API, but I anticipate it will be added to the completion API
  cached_tokens_details?: {
    text_tokens?: number;
    audio_tokens?: number;
  };
}

// documented type is wrong
export interface OpenAIPatchedCompletionTokenDetails {
  reasoning_tokens?: number;
  text_tokens?: number;
  audio_tokens?: number;
}
