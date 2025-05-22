import type { Agent } from "@hoangvvo/llm-agent";
import { getModel, getModelList } from "../../agent-js/examples/get-model.ts";
import {
  availableTools,
  createAgent,
} from "../../agent-js/examples/server/agent.ts";
import type { MyContext } from "../../agent-js/examples/server/context.ts";
import type { RunStreamBody } from "../../agent-js/examples/server/types.ts";

const corHeaders = {
  "Access-Control-Allow-Origin": "https://llm-sdk.hoangvvo.com",
  "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
  "Access-Control-Allow-Headers": "Content-Type, Authorization",
  "Access-Control-Allow-Credentials": "true",
};

async function handleGetModels(): Promise<Response> {
  const modelList = await getModelList();

  return Response.json(modelList, { headers: corHeaders });
}

function handleGetTools(): Response {
  const tools = availableTools.map((tool) => ({
    name: tool.name,
    description: tool.description,
  }));

  return Response.json(tools, { headers: corHeaders });
}

async function handleRunStream(request: Request): Promise<Response> {
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
        {
          status: 404,
          headers: { "Content-Type": "application/json", ...corHeaders },
        },
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
        headers: { "Content-Type": "application/json", ...corHeaders },
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
      ...corHeaders,
    },
  });
}

export default {
  async fetch(request, env): Promise<Response> {
    // @ts-expect-error: shimming process.env
    globalThis.process = {
      env: env,
    };

    if (request.method === "OPTIONS") {
      return new Response(null, {
        status: 204,
        headers: corHeaders,
      });
    }

    const url = new URL(request.url);

    if (request.method === "GET" && url.pathname === "/api/models") {
      return handleGetModels();
    }

    if (request.method === "GET" && url.pathname === "/api/tools") {
      return handleGetTools();
    }

    if (request.method === "POST" && url.pathname === "/api/run-stream") {
      return handleRunStream(request);
    }

    return new Response("Not Found", { status: 404 });
  },
} satisfies ExportedHandler<Env>;
