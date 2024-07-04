import type {
  LanguageModelInput,
  ModelResponse,
  PartialModelResponse,
} from "../schemas/index.js";

export type LanguageModelCapability =
  | "streaming"
  | "tool"
  | "response-format-json";

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
   * The capabilities of the model.
   */
  capabilities: LanguageModelCapability[];
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
