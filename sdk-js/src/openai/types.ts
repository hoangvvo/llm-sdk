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

export interface OpenAIPatchedResponsesImageGenerationCall {
  /**
   * The unique ID of the image generation call.
   */
  id: string;

  /**
   * The generated image encoded in base64.
   */
  result: string | null;

  /**
   * The status of the image generation call.
   */
  status: "in_progress" | "completed" | "generating" | "failed";

  /**
   * The type of the image generation call. Always `image_generation_call`.
   */
  type: "image_generation_call";

  output_format: string; // png, jpeg, etc.
  size?: `${number}x${number}`;
}

export interface OpenAIPatchedImageGenerationCallPartialImage {
  /**
   * The unique identifier of the image generation item being processed.
   */
  item_id: string;

  /**
   * The index of the output item in the response's output array.
   */
  output_index: number;

  /**
   * Base64-encoded partial image data, suitable for rendering as an image.
   */
  partial_image_b64: string;

  /**
   * 0-based index for the partial image (backend is 1-based, but this is 0-based for
   * the user).
   */
  partial_image_index: number;

  /**
   * The sequence number of the image generation item being processed.
   */
  sequence_number: number;

  /**
   * The type of the event. Always 'response.image_generation_call.partial_image'.
   */
  type: "response.image_generation_call.partial_image";

  size?: `${number}x${number}`;
  output_format?: string; // png, jpeg, etc.
}

/**
 * OpenAI does not have an equivalent of reasoning budget tokens, but
 * we can use the property to indicate the level of reasoning effort.
 */
export const OpenAIReasoningEffort = {
  Minimal: 1000,
  Low: 2000,
  Medium: 3000,
  High: 4000,
};
