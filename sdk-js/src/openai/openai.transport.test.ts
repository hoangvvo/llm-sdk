import assert from "node:assert/strict";
import { once } from "node:events";
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from "node:http";
import test, { suite, type TestContext } from "node:test";
import { OpenAIModel } from "./openai.ts";

type Handler = (
  request: IncomingMessage,
  response: ServerResponse,
) => void | Promise<void>;

async function startServer(t: TestContext, handler: Handler) {
  const server = createServer((request, response) => {
    void Promise.resolve(handler(request, response)).catch((error: unknown) => {
      response.destroy(error as Error);
    });
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(async () => {
    await new Promise<void>((resolve) => {
      server.close(() => {
        resolve();
      });
    });
  });
  const address = server.address();
  assert(address && typeof address !== "string");
  return `http://127.0.0.1:${String(address.port)}/v1`;
}

async function readJSON(request: IncomingMessage): Promise<unknown> {
  const chunks: Uint8Array[] = [];
  for await (const chunk of request as AsyncIterable<Uint8Array>) {
    chunks.push(chunk);
  }
  return JSON.parse(Buffer.concat(chunks).toString("utf8"));
}

const input = {
  system_prompt: "Be exact",
  messages: [
    {
      role: "user" as const,
      content: [{ type: "text" as const, text: "Hello" }],
    },
  ],
  max_tokens: 17,
  temperature: 0.2,
  top_p: 0.8,
};

suite("OpenAI recorded transport", () => {
  test("sends one neutral output for an empty tool result", async (t) => {
    let requestBody: unknown;
    const baseURL = await startServer(t, async (request, response) => {
      requestBody = await readJSON(request);
      response.setHeader("content-type", "application/json");
      response.end(JSON.stringify({ output: [] }));
    });
    const model = new OpenAIModel({
      modelId: "recorded-model",
      apiKey: "test-token",
      baseURL,
    });

    await model.generate({
      messages: [
        {
          role: "assistant",
          content: [
            {
              type: "tool-call",
              tool_call_id: "call_1",
              tool_name: "wait",
              args: {},
            },
          ],
        },
        {
          role: "tool",
          content: [
            {
              type: "tool-result",
              tool_call_id: "call_1",
              tool_name: "wait",
              content: [],
              status: "cancelled",
            },
          ],
        },
      ],
    });

    const body = requestBody as { input: Record<string, unknown>[] };
    assert.deepStrictEqual(
      body.input.filter((item) => item["type"] === "function_call_output"),
      [
        {
          type: "function_call_output",
          call_id: "call_1",
          output: "",
        },
      ],
    );
  });

  test("sends the exact generate request and maps the recorded response", async (t) => {
    let requestBody: unknown;
    let authorization: string | undefined;
    let requestURL: string | undefined;
    const baseURL = await startServer(t, async (request, response) => {
      requestURL = request.url;
      authorization = request.headers.authorization;
      requestBody = await readJSON(request);
      response.setHeader("content-type", "application/json");
      response.end(
        JSON.stringify({
          output: [
            {
              type: "message",
              id: "msg_1",
              role: "assistant",
              status: "completed",
              content: [
                {
                  type: "output_text",
                  text: "Recorded response",
                  annotations: [],
                  logprobs: [],
                },
              ],
            },
          ],
          usage: {
            input_tokens: 4,
            output_tokens: 2,
            total_tokens: 6,
            input_tokens_details: { cached_tokens: 1 },
            output_tokens_details: { reasoning_tokens: 0 },
          },
        }),
      );
    });
    const model = new OpenAIModel({
      modelId: "recorded-model",
      apiKey: "test-token",
      baseURL,
    });

    const result = await model.generate(input);

    assert.equal(requestURL, "/v1/responses");
    assert.equal(authorization, "Bearer test-token");
    assert.deepEqual(requestBody, {
      model: "recorded-model",
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "input_text", text: "Hello" }],
        },
      ],
      instructions: "Be exact",
      max_output_tokens: 17,
      store: false,
      stream: false,
      temperature: 0.2,
      top_p: 0.8,
    });
    assert.deepEqual(result, {
      content: [{ type: "text", text: "Recorded response" }],
      usage: { input_tokens: 4, output_tokens: 2 },
    });
  });

  test("parses fragmented SSE, split tool arguments, and usage-only chunks", async (t) => {
    const baseURL = await startServer(t, async (request, response) => {
      const body = (await readJSON(request)) as { stream?: boolean };
      assert.equal(body.stream, true);
      response.setHeader("content-type", "text/event-stream");
      const first = JSON.stringify({
        type: "response.output_item.added",
        output_index: 0,
        sequence_number: 0,
        item: {
          type: "function_call",
          id: "fc_1",
          call_id: "call_1",
          name: "lookup",
          arguments: "",
          status: "in_progress",
        },
      });
      response.write(`data: ${first.slice(0, 31)}`);
      response.write(`${first.slice(31)}\n\n`);
      response.write(
        `data: ${JSON.stringify({
          type: "response.function_call_arguments.delta",
          item_id: "fc_1",
          output_index: 0,
          sequence_number: 1,
          delta: '{"city":',
        })}\n\n`,
      );
      response.write(
        `data: ${JSON.stringify({
          type: "response.function_call_arguments.delta",
          item_id: "fc_1",
          output_index: 0,
          sequence_number: 2,
          delta: '"Hanoi"}',
        })}\n\n`,
      );
      response.write(
        `data: ${JSON.stringify({
          type: "response.completed",
          sequence_number: 3,
          response: {
            usage: {
              input_tokens: 7,
              output_tokens: 3,
              total_tokens: 10,
              input_tokens_details: { cached_tokens: 0 },
              output_tokens_details: { reasoning_tokens: 0 },
            },
          },
        })}\n\n`,
      );
      response.end("data: [DONE]\n\n");
    });
    const model = new OpenAIModel({
      modelId: "recorded-model",
      apiKey: "test-token",
      baseURL,
    });

    const partials = [];
    for await (const partial of model.stream(input)) partials.push(partial);

    assert.deepEqual(partials, [
      {
        delta: {
          index: 0,
          part: {
            type: "tool-call",
            id: "fc_1",
            tool_call_id: "call_1",
            tool_name: "lookup",
            args: "",
          },
        },
      },
      {
        delta: {
          index: 0,
          part: { type: "tool-call", args: '{"city":' },
        },
      },
      {
        delta: {
          index: 0,
          part: { type: "tool-call", args: '"Hanoi"}' },
        },
      },
      { usage: { input_tokens: 7, output_tokens: 3 } },
    ]);
  });

  test("surfaces recorded HTTP and malformed-stream failures", async (t) => {
    const baseURL = await startServer(t, async (request, response) => {
      const body = (await readJSON(request)) as { stream?: boolean };
      if (!body.stream) {
        response.statusCode = 429;
        response.setHeader("content-type", "application/json");
        response.end('{"error":{"message":"rate limited"}}');
        return;
      }
      response.setHeader("content-type", "text/event-stream");
      response.end('data: {"type":\n\n');
    });
    const model = new OpenAIModel({
      modelId: "recorded-model",
      apiKey: "test-token",
      baseURL,
    });

    await assert.rejects(
      () => model.generate(input),
      (error: unknown) => {
        assert.equal((error as { status?: number }).status, 429);
        return true;
      },
    );
    await assert.rejects(async () => {
      for await (const partial of model.stream(input)) {
        assert.ok(partial);
      }
    }, /JSON/);
  });
});
