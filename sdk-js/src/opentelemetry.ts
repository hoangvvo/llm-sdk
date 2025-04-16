import { SpanStatusCode, trace, type Span } from "@opentelemetry/api";
import type { LanguageModel } from "./language-model.ts";
import type {
  LanguageModelInput,
  ModelResponse,
  ModelUsage,
  PartialModelResponse,
} from "./types.ts";
import { sumModelUsage } from "./usage.utils.ts";

const tracer = trace.getTracer("@hoangvvo/llm-sdk");

export class LMSpan {
  provider: string;
  model_id: string;
  method: "generate" | "stream";
  usage: ModelUsage | null;
  cost: number | null;
  start_time: Date;
  /**
   * Time to first token, in seconds
   */
  time_to_first_token: number | undefined;
  max_tokens?: number;
  temperature?: number;
  top_p?: number;
  top_k?: number;
  presence_penalty?: number;
  frequency_penalty?: number;
  seed?: number;

  #span: Span;

  constructor(
    provider: string,
    modelId: string,
    method: "generate" | "stream",
    input: LanguageModelInput,
  ) {
    this.provider = provider;
    this.model_id = modelId;
    this.#span = tracer.startSpan(`llm_sdk.${method}`);
    this.start_time = new Date();
    this.method = method;
    this.usage = null;
    this.cost = null;

    if (input.max_tokens !== undefined) this.max_tokens = input.max_tokens;
    if (input.temperature !== undefined) this.temperature = input.temperature;
    if (input.top_p !== undefined) this.top_p = input.top_p;
    if (input.top_k !== undefined) this.top_k = input.top_k;
    if (input.presence_penalty !== undefined)
      this.presence_penalty = input.presence_penalty;
    if (input.frequency_penalty !== undefined)
      this.frequency_penalty = input.frequency_penalty;
    if (input.seed !== undefined) this.seed = input.seed;
  }

  onStreamPartial(partial: PartialModelResponse): void {
    if (partial.usage) {
      this.usage = this.usage ?? { input_tokens: 0, output_tokens: 0 };
      this.usage = sumModelUsage([this.usage, partial.usage]);
    }
    if (partial.cost !== undefined) {
      this.cost = (this.cost ?? 0) + partial.cost;
    }
    if (partial.delta && !this.time_to_first_token) {
      this.time_to_first_token =
        (Date.now() - this.start_time.getTime()) / 1000;
    }
  }

  onResponse(response: ModelResponse): void {
    if (response.usage) {
      this.usage = response.usage;
    }
  }

  onEnd(): void {
    this.#span.setAttributes({
      // https://opentelemetry.io/docs/specs/semconv/gen-ai/
      "gen_ai.operation.name": "generate_content",
      "gen_ai.provider.name": this.provider,
      "gen_ai.request.model": this.model_id,
      "gen_ai.usage.input_tokens": this.usage?.input_tokens,
      "gen_ai.usage.output_tokens": this.usage?.output_tokens,
      "gen_ai.server.time_to_first_token": this.time_to_first_token,
      "gen_ai.request.max_tokens": this.max_tokens,
      "gen_ai.request.temperature": this.temperature,
      "gen_ai.request.top_p": this.top_p,
      "gen_ai.request.top_k": this.top_k,
      "gen_ai.request.presence_penalty": this.presence_penalty,
      "gen_ai.request.frequency_penalty": this.frequency_penalty,
      "gen_ai.request.seed": this.seed,
      "llm_sdk.cost": this.cost ?? undefined,
    });
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
