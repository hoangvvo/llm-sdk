import type { Agent } from "@hoangvvo/llm-agent";
import type { APIRoute } from "astro";
import {
  getModel,
  getModelList,
} from "../../../../../agent-js/examples/get-model.ts";
import { createAgent } from "../../../../../agent-js/examples/server/agent.ts";
import type { MyContext } from "../../../../../agent-js/examples/server/context.ts";
import type { RunStreamBody } from "../../../../../agent-js/examples/server/types.ts";

export const POST: APIRoute = async ({ request }) => {
  let json: RunStreamBody;
  let agent: Agent<MyContext>;

  try {
    const body = await request.text();
    const apiKey = request.headers.get("authorization") ?? undefined;

    json = JSON.parse(body) as RunStreamBody;

    const modelList = await getModelList();

    const { provider, model_id } = json;
    const modelInfo = modelList.find(
      (m) => m.provider === provider && m.model_id === model_id,
    );
    if (!modelInfo) {
      return Response.json(
        { error: `Model not found: ${provider} - ${model_id}` },
        { status: 500, headers: { "Content-Type": "application/json" } },
      );
    }

    const model = getModel(provider, model_id, modelInfo.metadata, apiKey);

    const { enabled_tools, ...params } = json;

    const enabledToolsParam = Array.isArray(enabled_tools)
      ? Array.from(
          new Set(
            enabled_tools.filter(
              (toolName: unknown): toolName is string =>
                typeof toolName === "string",
            ),
          ),
        )
      : undefined;

    agent = createAgent(model, modelInfo, {
      enabledTools: enabledToolsParam,
      ...params,
    });
  } catch (err: unknown) {
    return Response.json(
      { error: (err as Error).message },
      {
        status: 500,
        headers: { "Content-Type": "application/json" },
      },
    );
  }

  const { input } = json;

  const stream = agent.runStream(input);

  const encoder = new TextEncoder();

  const readable = new ReadableStream<Uint8Array>({
    async start(controller) {
      // optional: keep-alive comments so some proxies don’t close idle connections
      const heartbeat = setInterval(() => {
        controller.enqueue(encoder.encode(`: keep-alive\n\n`));
      }, 15000);

      try {
        for await (const event of stream) {
          controller.enqueue(
            encoder.encode(`data: ${JSON.stringify(event)}\n\n`),
          );
        }
      } catch (err: unknown) {
        controller.enqueue(
          encoder.encode(
            `data: ${JSON.stringify({
              event: "error",
              error: (err as Error).message || String(err),
            })}\n\n`,
          ),
        );
      } finally {
        clearInterval(heartbeat);
        controller.close();
      }
    },
  });

  return new Response(readable, {
    status: 200,
    headers: {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache, no-transform",
      Connection: "keep-alive",
    },
  });
};

export const prerender = false;
