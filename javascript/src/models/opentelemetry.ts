import { Span, SpanKind, SpanStatusCode, trace } from "@opentelemetry/api";
import {
  LanguageModelInput,
  ModelResponse,
  ModelUsage,
} from "../schema/schema.js";
import { LanguageModel } from "./language-model.js";

const tracer = trace.getTracer("@firefliesai/llm-sdk");

export class LMSpan {
  provider: string;
  modelId: string;
  method: "generate" | "stream";
  usage: ModelUsage | null;
  cost: number | null;
  /**
   * Time to first token, in seconds
   */
  timeToFirstToken: number | undefined;

  input: LanguageModelInput;

  #span: Span;
  startTime: Date;

  constructor(
    provider: string,
    modelId: string,
    method: "generate" | "stream",
    input: LanguageModelInput,
  ) {
    this.provider = provider;
    this.modelId = modelId;
    this.method = method;
    this.usage = null;
    this.cost = null;
    this.input = input;

    this.startTime = new Date();
    this.#span = tracer.startSpan(`llm_sdk.${method}`, {
      kind: SpanKind.SERVER,
    });
  }

  onStreamPartial(): void {
    if (this.timeToFirstToken === undefined) {
      this.timeToFirstToken = (Date.now() - this.startTime.getTime()) / 1000;
    }
  }

  onResponse(response: ModelResponse): void {
    if (response.usage) {
      this.usage = response.usage;
    }
  }

  onError(error: unknown): void {
    this.#span.recordException(error as Error);
    this.#span.setStatus({
      code: SpanStatusCode.ERROR,
      message: String(error),
    });
    this.#span.end();
  }

  onEnd(): void {
    this.#span.setAttributes({
      // https://opentelemetry.io/docs/specs/semconv/gen-ai/
      "gen_ai.operation.name": "generate_content",
      "gen_ai.provider.name": this.provider,
      "gen_ai.request.model": this.modelId,
      "gen_ai.usage.input_tokens": this.usage?.inputTokens,
      "gen_ai.usage.output_tokens": this.usage?.outputTokens,
      "gen_ai.server.time_to_first_token": this.timeToFirstToken,
      "gen_ai.request.max_tokens": this.input.maxTokens,
      "gen_ai.request.temperature": this.input.temperature,
      "gen_ai.request.top_p": this.input.topP,
      "gen_ai.request.top_k": this.input.topK,
      "gen_ai.request.presence_penalty": this.input.presencePenalty,
      "gen_ai.request.frequency_penalty": this.input.frequencyPenalty,
      "gen_ai.request.seed": this.input.seed,
    });
    this.#span.end();
  }
}

export function traceLanguageModel(self: LanguageModel) {
  const originalGenerate = self.generate.bind(self);
  const originalStream = self.stream.bind(self);

  self.generate = async function (input: LanguageModelInput) {
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

  self.stream = async function* (input: LanguageModelInput) {
    const span = new LMSpan(self.provider, self.modelId, "stream", input);
    const stream = originalStream(input);
    try {
      let current = await stream.next();
      while (!current.done) {
        span.onStreamPartial();
        yield current.value;
        current = await stream.next();
      }
      span.onResponse(current.value);
      return current.value;
    } catch (error: unknown) {
      span.onError(error);
      throw error;
    } finally {
      span.onEnd();
    }
  };
}
