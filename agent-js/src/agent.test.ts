import { LanguageModelError } from "@hoangvvo/llm-sdk";
import { MockLanguageModel } from "@hoangvvo/llm-sdk/test";
import test, { suite, type TestContext } from "node:test";
import { Agent } from "./agent.ts";
import { AgentCleanupError, AgentLanguageModelError } from "./errors.ts";
import { tool } from "./tool.ts";
import type { Toolkit } from "./toolkit.ts";
import type { AgentStreamEvent } from "./types.ts";

function createCloseTrackingToolkit<TContext>(): {
  toolkit: Toolkit<TContext>;
  getCloseCalls: () => number;
} {
  let closeCalls = 0;
  return {
    toolkit: {
      createSession: () =>
        Promise.resolve({
          getSystemPrompt: () => undefined,
          getTools: () => [],
          close: () => {
            closeCalls++;
            return Promise.resolve();
          },
        }),
    },
    getCloseCalls: () => closeCalls,
  };
}

suite("Agent#run", () => {
  test("returns a cleanup error when session cleanup fails", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: { content: [{ type: "text", text: "done" }] },
    });
    const agent = new Agent({
      name: "test_agent",
      model,
      toolkits: [
        {
          createSession: () =>
            Promise.resolve({
              getSystemPrompt: () => undefined,
              getTools: () => [],
              close: () => Promise.reject(new Error("cleanup failed")),
            }),
        },
      ],
    });

    await t.assert.rejects(
      () =>
        agent.run({
          context: {},
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "Hello" }],
            },
          ],
        }),
      (error: unknown) => {
        t.assert.strictEqual(error instanceof AgentCleanupError, true);
        return true;
      },
    );
  });

  test("preserves the run error when cleanup also fails", async (t: TestContext) => {
    const model = new MockLanguageModel();
    const modelFailure = new LanguageModelError("generation failed");
    let closeCalls = 0;
    model.enqueueGenerateResult({
      error: modelFailure,
    });
    const agent = new Agent({
      name: "test_agent",
      model,
      toolkits: [
        {
          createSession: () =>
            Promise.resolve({
              getSystemPrompt: () => undefined,
              getTools: () => [],
              close: () => {
                closeCalls++;
                return Promise.reject(new Error("cleanup failed"));
              },
            }),
        },
      ],
    });

    await t.assert.rejects(
      () =>
        agent.run({
          context: {},
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "Hello" }],
            },
          ],
        }),
      (error: unknown) => {
        t.assert.strictEqual(error instanceof AgentLanguageModelError, true);
        t.assert.strictEqual(
          (error as AgentLanguageModelError).cause,
          modelFailure,
        );
        t.assert.deepStrictEqual((error as AgentLanguageModelError).snapshot, {
          output: [],
        });
        return true;
      },
    );
    t.assert.strictEqual(closeCalls, 1);
  });

  test("creates session, runs, and closes", async (t: TestContext) => {
    const closeTracker = createCloseTrackingToolkit<object>();
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: { content: [{ type: "text", text: "Mock response" }] },
    });
    const agent = new Agent({
      name: "test-agent",
      model,
      toolkits: [closeTracker.toolkit],
    });

    const response = await agent.run({
      context: {},
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Hello" }],
        },
      ],
    });

    t.assert.deepStrictEqual(response, {
      status: "completed",
      content: [{ type: "text", text: "Mock response" }],
      output: [
        {
          type: "model",
          content: [{ type: "text", text: "Mock response" }],
        },
      ],
    });
    t.assert.strictEqual(closeTracker.getCloseCalls(), 1);
  });

  test("closes the session when generation fails", async (t: TestContext) => {
    const closeTracker = createCloseTrackingToolkit<object>();
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      error: new LanguageModelError("generation failed"),
    });
    const agent = new Agent({
      name: "test-agent",
      model,
      toolkits: [closeTracker.toolkit],
    });

    await t.assert.rejects(() =>
      agent.run({
        context: {},
        input: [
          {
            type: "message",
            role: "user",
            content: [{ type: "text", text: "Hello" }],
          },
        ],
      }),
    );
    t.assert.strictEqual(closeTracker.getCloseCalls(), 1);
  });
});

suite("Agent#runStream", () => {
  test("emits a cleanup error instead of a response", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueStreamResult({
      partials: [{ delta: { index: 0, part: { type: "text", text: "done" } } }],
    });
    const agent = new Agent({
      name: "test_agent",
      model,
      toolkits: [
        {
          createSession: () =>
            Promise.resolve({
              getSystemPrompt: () => undefined,
              getTools: () => [],
              close: () => Promise.reject(new Error("cleanup failed")),
            }),
        },
      ],
    });
    const events: AgentStreamEvent[] = [];

    await t.assert.rejects(
      async () => {
        for await (const event of agent.runStream({
          context: {},
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "Hello" }],
            },
          ],
        })) {
          events.push(event);
        }
      },
      (error: unknown) => {
        t.assert.strictEqual(error instanceof AgentCleanupError, true);
        return true;
      },
    );

    t.assert.deepStrictEqual(
      events.map((event) => event.event),
      ["partial", "item"],
    );
  });

  test("closes the session when the caller stops streaming", async (t: TestContext) => {
    let closeCalls = 0;
    const model = new MockLanguageModel();
    model.enqueueStreamResult({
      partials: [
        { delta: { index: 0, part: { type: "text", text: "one" } } },
        { delta: { index: 0, part: { type: "text", text: "two" } } },
      ],
    });
    const agent = new Agent({
      name: "test_agent",
      model,
      toolkits: [
        {
          createSession: () =>
            Promise.resolve({
              getSystemPrompt: () => undefined,
              getTools: () => [],
              close: () => {
                closeCalls++;
                return Promise.resolve();
              },
            }),
        },
      ],
    });
    const stream = agent.runStream({
      context: {},
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Stream" }],
        },
      ],
    });

    await stream.next();
    await stream.return({ content: [], output: [], status: "cancelled" });
    t.assert.strictEqual(closeCalls, 1);
  });

  test("creates session, streams, and closes", async (t: TestContext) => {
    const closeTracker = createCloseTrackingToolkit<object>();
    const model = new MockLanguageModel();
    model.enqueueStreamResult({
      partials: [
        {
          delta: { index: 0, part: { type: "text", text: "Mock" } },
        },
      ],
    });
    const agent = new Agent({
      name: "test-agent",
      model,
      toolkits: [closeTracker.toolkit],
    });

    const generator = agent.runStream({
      context: {},
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Hello" }],
        },
      ],
    });

    const events = [];
    let current = await generator.next();
    while (!current.done) {
      events.push(current.value);
      current = await generator.next();
    }

    t.assert.deepStrictEqual(events, [
      {
        event: "partial",
        delta: { index: 0, part: { type: "text", text: "Mock" } },
      },
      {
        event: "item",
        index: 0,
        item: {
          type: "model",
          content: [{ type: "text", text: "Mock" }],
        },
      },
      {
        event: "response",
        status: "completed",
        content: [{ type: "text", text: "Mock" }],
        output: [
          {
            type: "model",
            content: [{ type: "text", text: "Mock" }],
          },
        ],
      },
    ]);
    t.assert.deepStrictEqual(current.value, {
      status: "completed",
      content: [{ type: "text", text: "Mock" }],
      output: [
        {
          type: "model",
          content: [{ type: "text", text: "Mock" }],
        },
      ],
    });
    t.assert.strictEqual(closeTracker.getCloseCalls(), 1);
  });

  test("closes the session when streaming fails", async (t: TestContext) => {
    const closeTracker = createCloseTrackingToolkit<object>();
    const model = new MockLanguageModel();
    model.enqueueStreamResult({
      error: new LanguageModelError("stream failed"),
    });
    const agent = new Agent({
      name: "test-agent",
      model,
      toolkits: [closeTracker.toolkit],
    });

    await t.assert.rejects(async () => {
      for await (const event of agent.runStream({
        context: {},
        input: [
          {
            type: "message",
            role: "user",
            content: [{ type: "text", text: "Hello" }],
          },
        ],
      })) {
        // No events are expected before the mocked stream fails.
        t.assert.fail(`unexpected event: ${JSON.stringify(event)}`);
      }
    });
    t.assert.strictEqual(closeTracker.getCloseCalls(), 1);
  });

  test("preserves the stream error when cleanup also fails", async (t: TestContext) => {
    const model = new MockLanguageModel();
    const modelFailure = new LanguageModelError("stream failed");
    let closeCalls = 0;
    model.enqueueStreamResult({
      error: modelFailure,
    });
    const agent = new Agent({
      name: "test_agent",
      model,
      toolkits: [
        {
          createSession: () =>
            Promise.resolve({
              getSystemPrompt: () => undefined,
              getTools: () => [],
              close: () => {
                closeCalls++;
                return Promise.reject(new Error("cleanup failed"));
              },
            }),
        },
      ],
    });

    await t.assert.rejects(
      async () => {
        for await (const event of agent.runStream({
          context: {},
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "Hello" }],
            },
          ],
        })) {
          t.assert.fail(`unexpected event: ${JSON.stringify(event)}`);
        }
      },
      (error: unknown) => {
        t.assert.strictEqual(error instanceof AgentLanguageModelError, true);
        t.assert.strictEqual(
          (error as AgentLanguageModelError).cause,
          modelFailure,
        );
        t.assert.deepStrictEqual((error as AgentLanguageModelError).snapshot, {
          output: [],
        });
        return true;
      },
    );
    t.assert.strictEqual(closeCalls, 1);
  });
});

test("Agent forwards its complete public configuration to the model", async (t: TestContext) => {
  const model = new MockLanguageModel();
  model.enqueueGenerateResult({
    response: { content: [{ type: "text", text: "configured" }] },
  });
  const functionTool = tool<{ tenant: string }, { query: string }>({
    name: "lookup",
    description: "Look up a record",
    parameters: {
      type: "object",
      properties: { query: { type: "string" } },
      required: ["query"],
      additionalProperties: false,
    },
    execute: () => ({ content: [], is_error: false }),
  });
  const responseFormat = {
    type: "json" as const,
    name: "answer",
    description: "A configured answer",
    schema: {
      type: "object",
      properties: { answer: { type: "string" } },
      required: ["answer"],
      additionalProperties: false,
    },
  };
  const audio = { format: "mp3" as const, voice: "alloy", language: "en" };
  const reasoning = { enabled: true, budget_tokens: 256 };
  const agent = new Agent({
    name: "configured-agent",
    model,
    instructions: ["Static", ({ tenant }) => `Tenant: ${tenant}`],
    tools: [
      functionTool,
      { type: "web_search", allowed_domains: ["example.com"] },
    ],
    response_format: responseFormat,
    max_turns: 3,
    temperature: 0.2,
    top_p: 0.8,
    top_k: 12,
    presence_penalty: 0.1,
    frequency_penalty: 0.3,
    modalities: ["text", "audio"],
    audio,
    reasoning,
  });

  await agent.run({
    context: { tenant: "acme" },
    input: [
      {
        type: "message",
        role: "user",
        content: [{ type: "text", text: "Configure this" }],
      },
    ],
  });

  const [input] = model.trackedGenerateInputs;
  t.assert.ok(input);
  t.assert.deepStrictEqual(input, {
    messages: [
      {
        role: "user",
        content: [{ type: "text", text: "Configure this" }],
      },
    ],
    system_prompt: "Static\nTenant: acme",
    tools: [
      {
        type: "function",
        name: "lookup",
        description: "Look up a record",
        parameters: functionTool.parameters,
      },
      { type: "web_search", allowed_domains: ["example.com"] },
    ],
    response_format: responseFormat,
    temperature: 0.2,
    top_p: 0.8,
    top_k: 12,
    presence_penalty: 0.1,
    frequency_penalty: 0.3,
    modalities: ["text", "audio"],
    audio,
    reasoning,
  });
});
