import type { Agent } from "@hoangvvo/llm-agent";
import {
  availableTools,
  createAgent,
} from "../../../agent-js/examples/server/agent.ts";
import type { MyContext } from "../../../agent-js/examples/server/context.ts";
import type { RunStreamBody } from "../../../agent-js/examples/server/types.ts";
import { getModel } from "./model.ts";

const encoder = new TextEncoder();

export function listTools(): Response {
  const tools = availableTools.flatMap((tool) =>
    tool.type === "function"
      ? [{ name: tool.name, description: tool.description }]
      : [],
  );
  return Response.json(tools);
}

export async function runStream(request: Request): Promise<Response> {
  let agent: Agent<MyContext>;
  let input: RunStreamBody["input"];

  try {
    const json = (await request.json()) as RunStreamBody;
    const {
      provider,
      model_id: modelId,
      metadata,
      enabled_tools: enabledTools,
      web_search: webSearch,
      mcp_servers: mcpServers = [],
      input: requestInput,
      ...params
    } = json;

    if (!requestInput) {
      return jsonError("Missing input payload", 400);
    }
    input = requestInput;

    if (mcpServers.some((server) => server.type === "stdio")) {
      return jsonError(
        "Stdio MCP servers are not supported by the Cloudflare runtime",
        400,
      );
    }

    const model = getModel(
      provider,
      modelId,
      metadata,
      request.headers.get("authorization"),
    );
    const normalizedEnabledTools = Array.isArray(enabledTools)
      ? Array.from(
          new Set(
            enabledTools.filter(
              (toolName): toolName is string => typeof toolName === "string",
            ),
          ),
        )
      : undefined;

    agent = createAgent(model, {
      ...(normalizedEnabledTools
        ? { enabledTools: normalizedEnabledTools }
        : {}),
      ...(webSearch ? { webSearch } : {}),
      mcpServers,
      ...params,
    });
  } catch (error) {
    return jsonError(getErrorMessage(error), 400);
  }

  const events = agent.runStream(input);
  const body = new ReadableStream<Uint8Array>({
    async pull(controller) {
      try {
        const next = await events.next();
        if (next.done) {
          controller.close();
          return;
        }
        controller.enqueue(
          encoder.encode(`data: ${JSON.stringify(next.value)}\n\n`),
        );
      } catch (error) {
        controller.enqueue(
          encoder.encode(
            `data: ${JSON.stringify({ event: "error", error: getErrorMessage(error) })}\n\n`,
          ),
        );
        controller.close();
      }
    },
    async cancel() {
      await events.return(undefined as never);
    },
  });

  return new Response(body, {
    headers: {
      "Cache-Control": "no-cache",
      "Content-Type": "text/event-stream; charset=utf-8",
      "X-Accel-Buffering": "no",
    },
  });
}

function jsonError(message: string, status: number): Response {
  return Response.json({ error: message }, { status });
}

function getErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Unknown error";
}
