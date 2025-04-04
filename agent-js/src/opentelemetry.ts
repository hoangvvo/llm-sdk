import type { ModelUsage } from "@hoangvvo/llm-sdk";
import { context, SpanStatusCode, trace, type Span } from "@opentelemetry/api";
import type { AgentToolResult } from "./tool.ts";
import type { AgentResponse } from "./types.ts";

// Initialie the tracer lazily to allow user to have a chance to configure the global tracer provider
const tracer = trace.getTracer("@hoangvvo/llm-agent");

export class AgentSpan {
  agent_name: string;
  method: "run" | "run_stream";
  usage: ModelUsage | null;
  cost: number | null;

  #span: Span;

  constructor(name: string, method: "run" | "run_stream") {
    this.#span = tracer.startSpan(`llm_agent.${method}`);
    this.agent_name = name;
    this.method = method;
    this.usage = null;
    this.cost = null;
  }

  onResponse(response: AgentResponse) {
    // Aggregate usage and cost from model items within output
    for (const item of response.output) {
      if (item.type === "model") {
        const usage = item.usage;
        const cost = item.cost;
        if (usage) {
          this.usage = this.usage ?? { input_tokens: 0, output_tokens: 0 };
          this.usage.input_tokens += usage.input_tokens;
          this.usage.output_tokens += usage.output_tokens;
        }
        if (typeof cost === "number") {
          this.cost = (this.cost ?? 0) + cost;
        }
      }
    }
  }

  onEnd() {
    this.#span.setAttributes({
      "gen_ai.operation.name": "invoke_agent",
      "gen_ai.agent.name": this.agent_name,
      "gen_ai.model.input_tokens": this.usage?.input_tokens ?? undefined,
      "gen_ai.model.output_tokens": this.usage?.output_tokens ?? undefined,
    });
    this.#span.end();
  }

  onError(error: unknown) {
    this.#span.recordException(error as Error);
    this.#span.setStatus({
      code: SpanStatusCode.ERROR,
      message: String(error),
    });
  }

  withContext<T>(fn: () => T): T {
    return context.with(trace.setSpan(context.active(), this.#span), fn);
  }
}

export function startActiveToolSpan(
  toolCallId: string,
  toolName: string,
  toolDescription: string,
  fn: () => Promise<AgentToolResult>,
) {
  return tracer.startActiveSpan("llm_agent.tool", async (span) => {
    try {
      const res = await fn();
      return res;
    } catch (error) {
      span.recordException(error as Error);
      span.setStatus({
        code: SpanStatusCode.ERROR,
        message: String(error),
      });
      throw error;
    } finally {
      // https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-spans/#execute-tool-span
      span.setAttributes({
        "gen_ai.operation.name": "execute_tool",
        "gen_ai.tool.call.id": toolCallId,
        "gen_ai.tool.description": toolDescription,
        "gen_ai.tool.name": toolName,
        "gen_ai.tool.type": "function",
      });
      span.end();
    }
  });
}
