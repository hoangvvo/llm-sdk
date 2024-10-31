import type {
  LanguageModelInput,
  ModelResponse,
  PartialModelResponse,
} from "../schemas/index.js";

export type LanguageModelCapability =
  | "streaming"
  | "tool"
  | "structured-output"
  | "structured-output-strict" // openai-specific
  | "audio-input"
  | "audio-output"
  | "image-input";

export type LanguageModelPricing = {
  inputCostPerTextToken?: number;
  outputCostPerTextToken?: number;
  inputCostPerAudioToken?: number;
  outputCostPerAudioToken?: number;
  inputCostPerImageToken?: number;
  outputCostPerImageToken?: number;
};

export type LanguageModelMetadata = {
  /**
   * The pricing per single token for the model. Used to calculate the cost of a response.
   */
  pricing?: LanguageModelPricing;
  /**
   * The capabilities of the model.
   */
  capabilities?: LanguageModelCapability[];
};

export interface LanguageModel {
  /**
   * The provider of the model, e.g. "openai", "anthropic", "google"
   */
  provider: string;
  /**
   * The ID of the model, e.g. "gpt-3.5-turbo", "haiku"
   */
  modelId: string;
  /**
   * The metadata of the model.
   */
  metadata?: LanguageModelMetadata;
  /**
   * Generates a response to the given input.
   */
  generate(input: LanguageModelInput): Promise<ModelResponse>;
  /**
   * Generates a response to the given input, returning a stream of partial responses.
   */
  stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse, ModelResponse>;
}
