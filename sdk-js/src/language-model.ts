import type {
  LanguageModelCapability,
  LanguageModelInput,
  LanguageModelPricing,
  ModelResponse,
  PartialModelResponse,
} from "./types.ts";

export interface LanguageModelMetadata {
  /**
   * The pricing per single token for the model. Used to calculate the cost of a response.
   */
  pricing?: LanguageModelPricing;
  /**
   * The capabilities of the model.
   */
  capabilities?: LanguageModelCapability[];
}

export abstract class LanguageModel {
  /**
   * The provider of the model, e.g. "openai", "anthropic", "google"
   */
  abstract provider: string;
  /**
   * The ID of the model, e.g. "gpt-3.5-turbo", "haiku"
   */
  abstract modelId: string;
  /**
   * The metadata of the model.
   */
  abstract metadata?: LanguageModelMetadata;
  /**
   * Generates a response to the given input.
   */
  abstract generate(input: LanguageModelInput): Promise<ModelResponse>;
  /**
   * Generates a response to the given input, returning a stream of partial responses.
   */
  abstract stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse>;
}
