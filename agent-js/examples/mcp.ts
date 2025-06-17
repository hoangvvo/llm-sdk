import { Agent } from "@hoangvvo/llm-agent";
import { mcpToolkit } from "@hoangvvo/llm-agent/mcp";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { randomUUID } from "node:crypto";
import { once } from "node:events";
import { createServer, type IncomingMessage } from "node:http";
import { z } from "zod";

import { getModel } from "./get-model.ts";

// This example demonstrates:
// 1. Launching a minimal streamable HTTP MCP server using the official TypeScript SDK.
// 2. Registering that server through the MCP toolkit primitive.
// 3. Having the agent call the remote tool during a conversation.

const PORT = 39813;
const SERVER_URL = `http://127.0.0.1:${PORT}`;
const AUTH_TOKEN = "transit-hub-secret";

interface SessionContext {
  riderName: string;
  authorization: string;
}

async function main(): Promise<void> {
  const stopServer = await startStubMcpServer();
  try {
    const model = getModel("openai", "gpt-4o-mini");

    const agent = new Agent<SessionContext>({
      name: "Sage",
      model,
      instructions: [
        "You are Sage, the shuttle concierge for the Transit Hub.",
        "Lean on connected transit systems before guessing, and tailor advice to the rider's shift.",
        (context) =>
          `You are assisting ${context.riderName} with tonight's shuttle planning.`,
      ],
      // The MCP toolkit primitive resolves transport params per session. Here we pull the rider-specific
      // authorization token from context so each agent session connects with the correct credentials.
      toolkits: [
        mcpToolkit((context) => ({
          type: "streamable-http",
          url: SERVER_URL,
          authorization: context.authorization,
        })),
      ],
    });

    const session = await agent.createSession({
      riderName: "Avery",
      authorization: AUTH_TOKEN,
    });
    try {
      const turn = await session.run({
        input: [
          {
            type: "message",
            role: "user",
            content: [
              { type: "text", text: "What shuttles are running tonight?" },
            ],
          },
        ],
      });

      console.log("=== Agent Response ===");
      const replyText = turn.content
        .filter(
          (part): part is { type: "text"; text: string } =>
            part.type === "text",
        )
        .map((part) => part.text)
        .join("\n");
      console.log(replyText || JSON.stringify(turn.content, null, 2));
    } finally {
      await session.close();
    }
  } finally {
    await stopServer();
  }
}

await main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

function createShuttleServer(): McpServer {
  const server = new McpServer({
    name: "shuttle-scheduler",
    version: "1.0.0",
  });

  server.registerTool(
    "list_shuttles",
    {
      description: "List active shuttle routes for the selected shift",
      inputSchema: {
        shift: z
          .enum(["evening", "overnight"])
          .describe(
            "Which operating window to query. OpenAI requires `additionalProperties: false` and every property listed in `required`, so this schema keeps a single required field.",
          ),
      },
    },
    async ({ shift }) => ({
      content: [
        {
          type: "text",
          text:
            shift === "overnight"
              ? "Harbor Express and Dawn Flyer are staged for the overnight shift."
              : "Midnight Loop and Harbor Express are on duty tonight.",
        },
      ],
    }),
  );

  return server;
}

function isAuthorized(req: IncomingMessage): boolean {
  const header = req.headers.authorization;
  return typeof header === "string" && header === `Bearer ${AUTH_TOKEN}`;
}

async function startStubMcpServer(): Promise<() => Promise<void>> {
  const server = createShuttleServer();
  const transport = new StreamableHTTPServerTransport({
    sessionIdGenerator: () => randomUUID(),
    enableJsonResponse: true,
  });

  await server.connect(transport);

  const httpServer = createServer((req, res) => {
    if (req.url === "/status" && req.method === "GET") {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ status: "ok" }));
      return;
    }

    if (!isAuthorized(req)) {
      res.writeHead(401, { "Content-Type": "application/json" });
      res.end(
        JSON.stringify({
          error: "unauthorized",
          message: "Provide the shuttle access token.",
        }),
      );
      return;
    }

    if (req.url === "/" && req.method === "POST") {
      const chunks: Buffer[] = [];
      req.on("data", (chunk) => chunks.push(chunk as Buffer));
      req.on("end", async () => {
        const body = Buffer.concat(chunks);
        await transport.handleRequest(req, res, JSON.parse(body.toString()));
      });
      return;
    }

    res.writeHead(404);
    res.end();
  });

  httpServer.listen(PORT);
  await once(httpServer, "listening");

  return async () => {
    await new Promise<void>((resolve, reject) => {
      httpServer.close((err) => {
        if (err) reject(err);
        else resolve();
      });
    });
  };
}
