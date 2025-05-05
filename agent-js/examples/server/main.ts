import type { Agent } from "@hoangvvo/llm-agent";
import { type AgentRequest } from "@hoangvvo/llm-agent";
import http from "node:http";
import { getModel, getModelList } from "../get-model.ts";
import { availableTools, createAgent } from "./agent.ts";
import type { MyContext } from "./context.ts";

interface RunStreamBody {
  provider: string;
  model_id: string;
  input: AgentRequest<MyContext>;
  enabled_tools?: string[];
  disabled_instructions?: boolean;
  temperature?: number;
  top_p?: number;
  top_k?: number;
  frequency_penalty?: number;
  presence_penalty?: number;
}

async function runStreamHandler(
  req: http.IncomingMessage,
  res: http.ServerResponse,
) {
  let json: RunStreamBody;
  let agent: Agent<MyContext>;

  try {
    const body = await readBody(req);
    const apiKey = req.headers.authorization;

    json = JSON.parse(body) as RunStreamBody;

    const modelList = await getModelList();

    const { provider, model_id } = json;
    const modelInfo = modelList.find(
      (m) => m.provider === provider && m.model_id === model_id,
    );
    if (!modelInfo) {
      throw new Error(`Model not found: ${provider} - ${model_id}`);
    }

    const model = getModel(provider, model_id, modelInfo.metadata, apiKey);

    const enabledToolsParam = Array.isArray(json.enabled_tools)
      ? Array.from(
          new Set(
            json.enabled_tools.filter(
              (toolName): toolName is string => typeof toolName === "string",
            ),
          ),
        )
      : undefined;

    const disabledInstructions = json.disabled_instructions === true;
    const temperature = sanitizeNumber(json.temperature);
    const topP = sanitizeNumber(json.top_p);
    const topK = sanitizeInteger(json.top_k);
    const frequencyPenalty = sanitizeNumber(json.frequency_penalty);
    const presencePenalty = sanitizeNumber(json.presence_penalty);

    agent = createAgent(model, modelInfo, {
      enabledTools: enabledToolsParam,
      disabledInstructions,
      temperature,
      top_p: topP,
      top_k: topK,
      frequency_penalty: frequencyPenalty,
      presence_penalty: presencePenalty,
    });
  } catch (err) {
    res.writeHead(500, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: (err as Error).message }));
    return;
  }

  const { input } = json;

  const stream = agent.runStream(input);

  // Return an SSE stream
  res.writeHead(200, {
    "Content-Type": "text/event-stream",
    "Cache-Control": "no-cache",
    Connection: "keep-alive",
  });

  try {
    for await (const event of stream) {
      res.write(`data: ${JSON.stringify(event)}\n\n`);
    }
  } catch (err: unknown) {
    res.write(
      `data: ${JSON.stringify({ event: "error", error: (err as Error).message })}\n\n`,
    );
    res.statusCode = 500;
    res.end();
  } finally {
    res.end();
  }
}

async function listModelsHandler(res: http.ServerResponse) {
  try {
    const modelList = await getModelList();
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify(modelList));
  } catch (err) {
    res.writeHead(500, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: (err as Error).message }));
  }
}

function listToolsHandler(
  _req: http.IncomingMessage,
  res: http.ServerResponse,
) {
  try {
    const tools = availableTools.map((tool) => ({
      name: tool.name,
      description: tool.description,
    }));
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify(tools));
  } catch (err) {
    res.writeHead(500, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: (err as Error).message }));
  }
}

function sanitizeNumber(value: unknown): number | undefined {
  if (typeof value !== "number") {
    return undefined;
  }
  if (!Number.isFinite(value)) {
    return undefined;
  }
  return value;
}

function sanitizeInteger(value: unknown): number | undefined {
  if (typeof value !== "number") {
    return undefined;
  }
  if (!Number.isFinite(value)) {
    return undefined;
  }
  const int = Math.trunc(value);
  if (int < 0) {
    return undefined;
  }
  return int;
}

function readBody(req: http.IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    let data = "";
    req.on("data", (chunk) => {
      data += String(chunk);
    });
    req.on("end", () => {
      resolve(data);
    });
    req.on("error", (err) => {
      reject(err);
    });
  });
}

http
  .createServer((req, res) => {
    // CORS headers
    res.setHeader("Access-Control-Allow-Origin", "*");
    res.setHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS");
    res.setHeader(
      "Access-Control-Allow-Headers",
      "Content-Type, Authorization",
    );

    if (req.method === "OPTIONS") {
      res.writeHead(204);
      res.end();
      return;
    }

    if (req.url === "/run-stream") {
      if (req.method === "POST") {
        void runStreamHandler(req, res);
        return;
      }
    }

    if (req.url === "/models") {
      void listModelsHandler(res);
      return;
    }

    if (req.url === "/tools") {
      listToolsHandler(req, res);
      return;
    }

    res.end(`Welcome to llm-agent-js Server!
GitHub: https://github.com/hoangvvo/llm-sdk`);
  })
  .listen(4000);

console.log("Server listening on http://localhost:4000");
