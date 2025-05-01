import type { LanguageModel } from "./language-model.ts";
import type {
  LanguageModelInput,
  ModelResponse,
  PartialModelResponse,
} from "./types.ts";

/**
 * A result for a mocked `generate` call.
 * It can either be a full response or an error to throw.
 */
export type MockGenerateResult = { response: ModelResponse } | { error: Error };

/**
 * A result for a mocked `stream` call.
 * It can either be a set of partial responses or an error to throw.
 */
export type MockStreamResult =
  | { partials: PartialModelResponse[] }
  | { error: Error };

/**
 * A mock language model for testing purposes
 * that tracks inputs and allows mocking outputs.
 */
export class MockLanguageModel implements LanguageModel {
  #mockedGenerateResults: MockGenerateResult[] = [];
  #mockedStreamResults: MockStreamResult[] = [];

  /**
   * Tracked inputs for `generate` calls.
   * This can be used to assert that the correct inputs were passed to the model.
   */
  trackedGenerateInputs: LanguageModelInput[] = [];
  /**
   * Tracked inputs for `stream` calls.
   * This can be used to assert that the correct inputs were passed to the model.
   */
  trackedStreamInputs: LanguageModelInput[] = [];

  provider: string;
  modelId: string;

  constructor() {
    this.provider = "mock";
    this.modelId = "mock-model";
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const result = this.#mockedGenerateResults.shift();
    if (!result) {
      throw new Error("No mocked generate results available");
    }
    this.trackedGenerateInputs.push(input);
    if ("error" in result) {
      throw result.error;
    }
    return Promise.resolve(result.response);
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse> {
    const result = this.#mockedStreamResults.shift();
    if (!result) {
      throw new Error("No mocked stream results available");
    }
    this.trackedStreamInputs.push(input);
    if ("error" in result) {
      throw result.error;
    }
    for (const partial of result.partials) {
      yield Promise.resolve(partial);
    }
  }

  /**
   * Mock a full response for the next call to `generate`.
   */
  enqueueGenerateResult(...result: MockGenerateResult[]) {
    this.#mockedGenerateResults.push(...result);
  }

  /**
   * Mock a set of partial responses for the next call to `stream`.
   */
  enqueueStreamResult(...result: MockStreamResult[]) {
    this.#mockedStreamResults.push(...result);
  }

  /**
   * Reset the tracked inputs.
   */
  reset() {
    this.trackedGenerateInputs = [];
    this.trackedStreamInputs = [];
  }

  /**
   * Restore the mock to its initial state (clear mocked results and tracked inputs).
   */
  restore() {
    this.#mockedGenerateResults = [];
    this.#mockedStreamResults = [];
    this.reset();
  }
}
