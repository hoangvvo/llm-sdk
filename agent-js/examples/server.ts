import {
  Agent,
  type AgentRequest,
  type InstructionParam,
} from "@hoangvvo/llm-agent";
import { zodTool } from "@hoangvvo/llm-agent/zod";
import type { LanguageModel } from "@hoangvvo/llm-sdk";
import http from "node:http";
import z from "zod";
import { getModel, getModelList, type ModelInfo } from "./get-model.ts";

interface MyContext {
  name?: string;
  location?: string;
  units?: string;
  geo_api_key?: string;
  tomorrow_api_key?: string;
}

const instructions: InstructionParam<MyContext>[] = [
  `Answer in markdown format.
To access certain tools, the user may have to provide corresponding API keys in the context fields on the UI.`,
  (context) =>
    `The user name is ${context.name ?? "<not provided>"}. The user location is ${context.location ?? "<not provided>"}. The user prefers ${context.units ?? "<not provided>"} units.`,
  () => `The current date is ${new Date().toDateString()}.`,
];

const getCoordinatesTool = zodTool({
  name: "get_coordinates",
  description: "Get coordinates (latitude and longitude) from a location name",
  parameters: z.object({
    location: z.string().describe("The location name, e.g. Paris, France"),
  }),
  execute: async (input, context: MyContext) => {
    const { location } = input;

    const apiKey = context.geo_api_key ?? process.env["GEO_API_KEY"];

    if (apiKey === undefined) {
      return {
        content: [
          {
            type: "text",
            text: "API Key not provided. You can also provide the value on the UI with the Context field 'geo_api_key'. Get a free API key at https://geocode.maps.co/",
          },
        ],
        is_error: true,
      };
    }

    const response = await fetch(
      `https://geocode.maps.co/search?q=${encodeURIComponent(location)}&api_key=${apiKey}`,
    );

    if (!response.ok) {
      return {
        content: [
          {
            type: "text",
            text: `Error fetching coordinates: ${String(response.status)} ${response.statusText}`,
          },
        ],
        is_error: true,
      };
    }

    const items = (await response.json()) as {
      lat: string;
      lon: string;
    }[];

    if (!items[0]) {
      return {
        content: [
          {
            type: "text",
            text: `No coordinates found for location: ${location}`,
          },
        ],
        is_error: true,
      };
    }

    const { lat, lon } = items[0];

    return {
      content: [
        {
          type: "text",
          text: JSON.stringify({ latitude: lat, longitude: lon }),
        },
      ],
      is_error: false,
    };
  },
});

const getWeatherTool = zodTool({
  name: "get_weather",
  description: "Get current weather from latitude and longitude",
  parameters: z.object({
    latitude: z.string().describe("The latitude"),
    longitude: z.string().describe("The longitude"),
    units: z.enum(["metric", "imperial"]).describe("Units"),
    timesteps: z.enum(["current", "1h", "1d"]).describe("Timesteps"),
    startTime: z.string().describe("Start time in ISO format"),
  }),
  execute: async (input, context: MyContext) => {
    const { latitude, longitude, units, timesteps, startTime } = input;

    const apiKey = context.tomorrow_api_key ?? process.env["TOMORROW_API_KEY"];

    if (apiKey === undefined) {
      return {
        content: [
          {
            type: "text",
            text: "API Key not provided. You can also provide the value on the UI with the Context field 'tomorrow_api_key'. Get a free API key at https://tomorrow.io/",
          },
        ],
        is_error: true,
      };
    }

    const fields = ["temperature", "temperatureApparent", "humidity"].join(",");

    const response = await fetch(
      `https://api.tomorrow.io/v4/timelines?location=${latitude},${longitude}&fields=${fields}&timesteps=${timesteps}&units=${units}&startTime=${startTime}&apikey=${apiKey}`,
    );

    if (!response.ok) {
      return {
        content: [
          {
            type: "text",
            text: `Error fetching weather: ${String(response.status)} ${response.statusText}`,
          },
        ],
        is_error: true,
      };
    }

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const data = await response.json();

    return {
      content: [
        {
          type: "text",
          text: JSON.stringify(data),
        },
      ],
      is_error: false,
    };
  },
});

const availableTools = [getCoordinatesTool, getWeatherTool];

// Create a new instance of Agent for the request
// In your application, you may want to reuse the same instance of Agent throughout the lifetime of your app
function createAgent(
  model: LanguageModel,
  modelInfo: ModelInfo,
  options?: {
    enabledTools?: string[] | undefined;
    disabledInstructions?: boolean | undefined;
    temperature?: number | undefined;
    top_p?: number | undefined;
    top_k?: number | undefined;
    frequency_penalty?: number | undefined;
    presence_penalty?: number | undefined;
  },
): Agent<MyContext> {
  const enabledTools = options?.enabledTools;
  const toolNameSet = enabledTools ? new Set(enabledTools) : null;
  const tools =
    toolNameSet === null
      ? availableTools
      : availableTools.filter((tool) => toolNameSet.has(tool.name));
  const agentInstructions = options?.disabledInstructions ? [] : instructions;

  return new Agent<MyContext>({
    name: "MyAgent",
    instructions: agentInstructions,
    model,
    tools,
    max_turns: 5,
    ...(options?.temperature !== undefined && {
      temperature: options.temperature,
    }),
    ...(options?.top_p !== undefined && { top_p: options.top_p }),
    ...(options?.top_k !== undefined && { top_k: options.top_k }),
    ...(options?.frequency_penalty !== undefined && {
      frequency_penalty: options.frequency_penalty,
    }),
    ...(options?.presence_penalty !== undefined && {
      presence_penalty: options.presence_penalty,
    }),
    ...(modelInfo.modalities && { modalities: modelInfo.modalities }),
    ...(modelInfo.audio && { audio: modelInfo.audio }),
    ...(modelInfo.reasoning && { reasoning: modelInfo.reasoning }),
  });
}

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

async function listModelsHandler(
  req: http.IncomingMessage,
  res: http.ServerResponse,
) {
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
      void listModelsHandler(req, res);
      return;
    }

    if (req.url === "/tools") {
      listToolsHandler(req, res);
      return;
    }

    res.statusCode = 204;
    res.end();
  })
  .listen(4000);

console.log("Server listening on http://localhost:4000");
