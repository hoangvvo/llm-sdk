import {
  SpanStatusCode,
  trace,
  type Span,
  type Tracer,
} from "@opentelemetry/api";
import type { LanguageModel } from "./language-model.ts";
import type {
  LanguageModelInput,
  ModelResponse,
  ModelUsage,
  PartialModelResponse,
} from "./types.ts";

function getTracer(): Tracer {
  return trace.getTracer("@hoangvvo/llm-sdk");
}

export class LMSpan {
  /**
   * OpenTelemetry span
   */
  #span: Span;

  /**
   * Start time in milliseconds
   */
  #startTime: number;

  #streamPartialUsage: ModelUsage | undefined;

  /**
   * Time to first token, in seconds
   */
  #timeToFirstToken: number | undefined;

  constructor(
    provider: string,
    modelId: string,
    method: "generate" | "stream",
    input: LanguageModelInput,
  ) {
    this.#span = getTracer().startSpan(`llm_sdk.${method}`).setAttributes({
      // https://opentelemetry.io/docs/specs/semconv/gen-ai/
      "gen_ai.operation.name": "generate_content",
      "gen_ai.provider.name": provider,
      "gen_ai.request.model": modelId,
      "gen_ai.request.seed": input.seed,
      "gen_ai.request.frequency_penalty": input.frequency_penalty,
      "gen_ai.request.max_tokens": input.max_tokens,
      "gen_ai.request.presence_penalty": input.presence_penalty,
      "gen_ai.request.temperature": input.temperature,
      "gen_ai.request.top_k": input.top_k,
      "gen_ai.request.top_p": input.top_p,
    });
    this.#startTime = Date.now();
  }

  onStreamPartial(partial: PartialModelResponse): void {
    if (partial.usage) {
      this.#streamPartialUsage = this.#streamPartialUsage ?? {
        input_tokens: 0,
        output_tokens: 0,
      };
      this.#streamPartialUsage.input_tokens += partial.usage.input_tokens;
      this.#streamPartialUsage.output_tokens += partial.usage.output_tokens;
      this.#span.setAttributes({
        "gen_ai.usage.input_tokens": this.#streamPartialUsage.input_tokens,
        "gen_ai.usage.output_tokens": this.#streamPartialUsage.output_tokens,
      });
    }
    if (partial.delta && !this.#timeToFirstToken) {
      this.#timeToFirstToken = (Date.now() - this.#startTime) / 1000;
      this.#span.setAttribute(
        "gen_ai.server.time_to_first_token",
        this.#timeToFirstToken,
      );
    }
  }

  onResponse(response: ModelResponse): void {
    if (response.usage) {
      this.#span.setAttributes({
        "gen_ai.usage.input_tokens": response.usage.input_tokens,
        "gen_ai.usage.output_tokens": response.usage.output_tokens,
      });
    }
  }

  onEnd(): void {
    this.#span.end();
  }

  onError(error: unknown): void {
    this.#span.recordException(error as Error);
    this.#span.setStatus({
      code: SpanStatusCode.ERROR,
      message: String(error),
    });
  }
}

export function traceLanguageModel(self: LanguageModel) {
  const originalGenerate = self.generate.bind(self);
  const originalStream = self.stream.bind(self);

  self.generate = function generate(
    input: LanguageModelInput,
  ): Promise<ModelResponse> {
    const span = new LMSpan(self.provider, self.modelId, "generate", input);
    return originalGenerate(input)
      .then(
        (response) => {
          span.onResponse(response);
          return response;
        },
        (error: unknown) => {
          span.onError(error);
          throw error;
        },
      )
      .finally(() => {
        span.onEnd();
      });
  };

  self.stream = async function* stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse> {
    const span = new LMSpan(self.provider, self.modelId, "stream", input);
    const stream = originalStream(input);
    for await (const partial of stream) {
      span.onStreamPartial(partial);
      yield partial;
    }
    span.onEnd();
  };
}
