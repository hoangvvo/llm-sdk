/* eslint-disable @typescript-eslint/no-floating-promises */
import { randomUUID } from "node:crypto";
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from "node:http";
import type { AddressInfo } from "node:net";
import test, { suite, type TestContext } from "node:test";

import { Agent } from "../agent.ts";
import { convertMCPContentToParts, type MCPContent } from "./content.ts";
import { mcpToolkit } from "./toolkit.ts";

import { MockLanguageModel } from "@hoangvvo/llm-sdk/test";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { z } from "zod";
import type { AgentResponse } from "../types.ts";

const IMAGE_DATA = Buffer.from([0x00, 0x01, 0x02]).toString("base64");
const AUDIO_DATA = Buffer.from([0x03, 0x04]).toString("base64");

type ToolResponder = (params: { shift: "evening" | "overnight" }) => {
  content: (
    | { type: "text"; text: string }
    | { type: "image"; mimeType: string; data: string }
    | { type: "audio"; mimeType: string; data: string }
    | { type: "resource_link"; uri: string; name: string }
  )[];
};

async function readJsonBody(req: IncomingMessage): Promise<unknown> {
  const chunks: Buffer[] = [];

  for await (const chunk of req) {
    chunks.push(chunk as Buffer);
  }

  if (chunks.length === 0) {
    return undefined;
  }

  const body = Buffer.concat(chunks).toString();
  return body ? JSON.parse(body) : undefined;
}

async function startStubMcpServer(): Promise<{
  url: string;
  stop: () => Promise<void>;
  updateTool: (options: {
    name?: string;
    description?: string;
    responder?: ToolResponder;
  }) => void;
}> {
  const server = new McpServer({
    name: "stub-mcp",
    version: "1.0.0",
  });

  let responder: ToolResponder = ({ shift }) => ({
    content: [
      {
        type: "text",
        text: `Shuttle summary for ${shift} shift.`,
      },
      {
        type: "image",
        mimeType: "image/png",
        data: IMAGE_DATA,
      },
      {
        type: "audio",
        mimeType: "audio/mpeg",
        data: AUDIO_DATA,
      },
      {
        type: "resource_link",
        uri: "https://example.com/docs",
        name: "ignored",
      },
    ],
  });

  const registeredTool = server.registerTool(
    "list_shuttles",
    {
      description: "List active shuttle routes for a shift",
      inputSchema: {
        shift: z
          .enum(["evening", "overnight"])
          .describe("Which operating window to query."),
      },
    },
    ({ shift }: { shift: "evening" | "overnight" }) => responder({ shift }),
  );

  const transport = new StreamableHTTPServerTransport({
    sessionIdGenerator: () => randomUUID(),
    enableJsonResponse: true,
  });

  await server.connect(transport);

  const httpServer = createServer(
    // eslint-disable-next-line @typescript-eslint/no-misused-promises
    async (req: IncomingMessage, res: ServerResponse) => {
      try {
        const parsedBody = await readJsonBody(req);
        await transport.handleRequest(req, res, parsedBody);
      } catch (error) {
        res.statusCode = 500;
        res.setHeader("Content-Type", "application/json");
        res.end(
          JSON.stringify({
            error: error instanceof Error ? error.message : String(error),
          }),
        );
      }
    },
  );

  const listenPromise = new Promise<AddressInfo>((resolve, reject) => {
    httpServer.once("error", reject);
    httpServer.listen(0, "127.0.0.1", () => {
      httpServer.off("error", reject);
      resolve(httpServer.address() as AddressInfo);
    });
  });

  const address = await listenPromise;
  const baseUrl = `http://${address.address}:${String(address.port)}`;

  return {
    url: baseUrl,
    stop: async () => {
      await transport.close();
      await server.close();
      await new Promise<void>((resolve, reject) => {
        httpServer.close((err) => {
          if (err) reject(err);
          else resolve();
        });
      });
    },
    updateTool: ({ name, description, responder: nextResponder }) => {
      if (nextResponder) {
        responder = nextResponder;
      }
      registeredTool.update({
        ...(name ? { name } : {}),
        ...(description ? { description } : {}),
      });
    },
  };
}

suite("MCP toolkit", () => {
  test("agent hydrates MCP tools over streamable HTTP", async (t: TestContext) => {
    const stub = await startStubMcpServer();

    try {
      const model = new MockLanguageModel();
      model.enqueueGenerateResult({
        response: {
          content: [
            {
              type: "tool-call",
              tool_name: "list_shuttles",
              tool_call_id: "call_1",
              args: { shift: "evening" },
            },
          ],
        },
      });
      model.enqueueGenerateResult({
        response: {
          content: [{ type: "text", text: "Ready to roll." }],
        },
      });

      const agent = new Agent({
        name: "mcp-test",
        model,
        instructions: [],
        toolkits: [
          mcpToolkit(() => ({
            type: "streamable-http",
            url: stub.url,
          })),
        ],
      });

      const session = await agent.createSession({});

      try {
        const response = await session.run({
          input: [
            {
              type: "message",
              role: "user",
              content: [
                {
                  type: "text",
                  text: "What's running tonight?",
                },
              ],
            },
          ],
        });

        const expected: AgentResponse = {
          content: [{ type: "text", text: "Ready to roll." }],
          output: [
            {
              type: "model",
              content: [
                {
                  type: "tool-call",
                  tool_name: "list_shuttles",
                  tool_call_id: "call_1",
                  args: { shift: "evening" },
                },
              ],
            },
            {
              type: "tool",
              tool_name: "list_shuttles",
              tool_call_id: "call_1",
              input: { shift: "evening" },
              output: [
                { type: "text", text: "Shuttle summary for evening shift." },
                {
                  type: "image",
                  mime_type: "image/png",
                  data: IMAGE_DATA,
                },
                {
                  type: "audio",
                  data: AUDIO_DATA,
                  format: "mp3",
                },
              ],
              is_error: false,
            },
            {
              type: "model",
              content: [{ type: "text", text: "Ready to roll." }],
            },
          ],
        };
        t.assert.deepStrictEqual(response, expected);
      } finally {
        await session.close();
      }
    } finally {
      await stub.stop();
    }
  });

  test("agent refreshes tools on MCP list change", async (t: TestContext) => {
    const stub = await startStubMcpServer();

    try {
      const model = new MockLanguageModel();
      model.enqueueGenerateResult({
        response: {
          content: [
            {
              type: "tool-call",
              tool_name: "list_shuttles",
              tool_call_id: "call_1",
              args: { shift: "evening" },
            },
          ],
        },
      });
      model.enqueueGenerateResult({
        response: {
          content: [{ type: "text", text: "Ready to roll." }],
        },
      });
      model.enqueueGenerateResult({
        response: {
          content: [
            {
              type: "tool-call",
              tool_name: "list_shuttles_v2",
              tool_call_id: "call_2",
              args: { shift: "evening" },
            },
          ],
        },
      });
      model.enqueueGenerateResult({
        response: {
          content: [{ type: "text", text: "Routes synced." }],
        },
      });

      const agent = new Agent({
        name: "mcp-test",
        model,
        instructions: [],
        toolkits: [
          mcpToolkit(() => ({
            type: "streamable-http",
            url: stub.url,
          })),
        ],
      });

      const session = await agent.createSession({});

      try {
        const firstResponse = await session.run({
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "What's running tonight?" }],
            },
          ],
        });

        const expectedFirst: AgentResponse = {
          content: [{ type: "text", text: "Ready to roll." }],
          output: [
            {
              type: "model",
              content: [
                {
                  type: "tool-call",
                  tool_name: "list_shuttles",
                  tool_call_id: "call_1",
                  args: { shift: "evening" },
                },
              ],
            },
            {
              type: "tool",
              tool_name: "list_shuttles",
              tool_call_id: "call_1",
              input: { shift: "evening" },
              output: [
                { type: "text", text: "Shuttle summary for evening shift." },
                {
                  type: "image",
                  mime_type: "image/png",
                  data: IMAGE_DATA,
                },
                {
                  type: "audio",
                  data: AUDIO_DATA,
                  format: "mp3",
                },
              ],
              is_error: false,
            },
            {
              type: "model",
              content: [{ type: "text", text: "Ready to roll." }],
            },
          ],
        };
        t.assert.deepStrictEqual(firstResponse, expectedFirst);

        stub.updateTool({
          name: "list_shuttles_v2",
          description: "List active shuttle routes with live updates",
          responder: ({ shift }) => ({
            content: [
              {
                type: "text",
                text: `Updated shuttle roster for ${shift} shift.`,
              },
            ],
          }),
        });

        await new Promise((resolve) => setTimeout(resolve, 20));

        const secondResponse = await session.run({
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "How about now?" }],
            },
          ],
        });

        const expectedSecond: AgentResponse = {
          content: [{ type: "text", text: "Routes synced." }],
          output: [
            {
              type: "model",
              content: [
                {
                  type: "tool-call",
                  tool_name: "list_shuttles_v2",
                  tool_call_id: "call_2",
                  args: { shift: "evening" },
                },
              ],
            },
            {
              type: "tool",
              tool_name: "list_shuttles_v2",
              tool_call_id: "call_2",
              input: { shift: "evening" },
              output: [
                {
                  type: "text",
                  text: "Updated shuttle roster for evening shift.",
                },
              ],
              is_error: false,
            },
            {
              type: "model",
              content: [{ type: "text", text: "Routes synced." }],
            },
          ],
        };
        t.assert.deepStrictEqual(secondResponse, expectedSecond);
      } finally {
        await session.close();
      }
    } finally {
      await stub.stop();
    }
  });

  test("convertMCPContentToParts maps supported content", (t: TestContext) => {
    const contents: MCPContent[] = [
      { type: "text", text: "sample" },
      { type: "image", mimeType: "image/png", data: IMAGE_DATA },
      { type: "audio", mimeType: "audio/mpeg", data: AUDIO_DATA },
      {
        // @ts-expect-error Testing that unsupported types are skipped.
        type: "resource_link",
        uri: "https://example.com",
        name: "ignored",
      },
    ];

    const parts = convertMCPContentToParts(contents);

    t.assert.deepStrictEqual(parts, [
      { type: "text", text: "sample" },
      {
        type: "image",
        mime_type: "image/png",
        data: IMAGE_DATA,
      },
      {
        type: "audio",
        data: AUDIO_DATA,
        format: "mp3",
      },
    ]);
  });
});
