import { LanguageModelError } from "@hoangvvo/llm-sdk";
import { MockLanguageModel } from "@hoangvvo/llm-sdk/test";
import test, { suite, type TestContext } from "node:test";
import { Agent } from "./agent.ts";
import type { Toolkit } from "./toolkit.ts";

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
});
