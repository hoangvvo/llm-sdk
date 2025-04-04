/* eslint-disable @typescript-eslint/no-floating-promises */

import type { LanguageModel, PartialModelResponse } from "@hoangvvo/llm-sdk";
import test, { suite, type TestContext } from "node:test";
import { Agent } from "./agent.ts";

function createMockLanguageModel(): LanguageModel {
  return {
    modelId: "mock-model",
    provider: "mock",
    generate: () =>
      Promise.resolve({ content: [{ type: "text", text: "Mock response" }] }),
    stream: async function* () {
      const event: PartialModelResponse = {
        delta: { index: 0, part: { type: "text", text: "Mock" } },
      };
      yield Promise.resolve(event);
    },
  };
}

suite("Agent#run", () => {
  test("creates session, runs, and finishes", async (t: TestContext) => {
    const model = createMockLanguageModel();
    const agent = new Agent({
      name: "test-agent",
      model,
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
  });
});

suite("Agent#runStream", () => {
  test("creates session, streams, and finishes", async (t: TestContext) => {
    const model = createMockLanguageModel();
    const agent = new Agent({
      name: "test-agent",
      model,
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
        type: "partial",
        delta: { index: 0, part: { type: "text", text: "Mock" } },
      },
      {
        type: "item",
        item: {
          type: "model",
          content: [{ type: "text", text: "Mock" }],
        },
      },
      {
        type: "response",
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
  });
});
