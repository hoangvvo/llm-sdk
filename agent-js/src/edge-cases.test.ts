import { MockLanguageModel } from "@hoangvvo/llm-sdk/test";
import test, { suite, type TestContext } from "node:test";
import { Agent } from "./agent.ts";
import { AgentInvariantError } from "./errors.ts";
import { RunSession } from "./run.ts";
import type { AgentFunctionTool } from "./tool.ts";

suite("agent edge behavior", () => {
  test("rejects empty input without calling the model", async (t: TestContext) => {
    const model = new MockLanguageModel();
    const session = await RunSession.create({
      name: "test_agent",
      model,
      context: {},
    });

    await t.assert.rejects(
      () => session.run({ input: [] }),
      (error: unknown) => error instanceof AgentInvariantError,
    );
    t.assert.strictEqual(model.trackedGenerateInputs.length, 0);
  });

  test("rejects duplicate tool-call IDs before executing tools", async (t: TestContext) => {
    let executions = 0;
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: {
        content: [
          {
            type: "tool-call",
            tool_call_id: "duplicate",
            tool_name: "first",
            args: {},
          },
          {
            type: "tool-call",
            tool_call_id: "duplicate",
            tool_name: "second",
            args: {},
          },
        ],
      },
    });
    const makeTool = (name: string): AgentFunctionTool<object> => ({
      type: "function",
      name,
      description: name,
      parameters: { type: "object", properties: {} },
      execute: () => {
        executions++;
        return { content: [], is_error: false };
      },
    });
    const session = await RunSession.create({
      name: "test_agent",
      model,
      tools: [makeTool("first"), makeTool("second")],
      context: {},
    });

    await t.assert.rejects(
      () =>
        session.run({
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "Use tools" }],
            },
          ],
        }),
      /Duplicate tool call ID: duplicate/,
    );
    t.assert.strictEqual(executions, 0);
  });

  test("exposes the current turn and accumulated items to tools", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueGenerateResult(
      {
        response: {
          content: [
            {
              type: "tool-call",
              tool_call_id: "call_1",
              tool_name: "inspect_state",
              args: {},
            },
          ],
        },
      },
      { response: { content: [{ type: "text", text: "done" }] } },
    );
    const inspectTool: AgentFunctionTool<object> = {
      type: "function",
      name: "inspect_state",
      description: "Inspect run state",
      parameters: { type: "object", properties: {} },
      execute: (_args, _context, state) => {
        t.assert.strictEqual(state.currentTurn, 1);
        t.assert.deepStrictEqual(
          state.getItems().map((item) => item.type),
          ["message", "model"],
        );
        return {
          content: [{ type: "text", text: "inspected" }],
          is_error: false,
        };
      },
    };
    const session = await RunSession.create({
      name: "test_agent",
      model,
      tools: [inspectTool],
      context: {},
    });

    const response = await session.run({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Inspect" }],
        },
      ],
    });
    t.assert.deepStrictEqual(response.content, [
      { type: "text", text: "done" },
    ]);
  });

  test("turns invalid streamed delta sequences into invariant errors", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueStreamResult({
      partials: [
        { delta: { index: 0, part: { type: "text", text: "hello" } } },
        {
          delta: {
            index: 0,
            part: { type: "reasoning", text: "wrong type" },
          },
        },
      ],
    });
    const session = await RunSession.create({
      name: "test_agent",
      model,
      context: {},
    });

    await t.assert.rejects(async () => {
      for await (const event of session.runStream({
        input: [
          {
            type: "message",
            role: "user",
            content: [{ type: "text", text: "Stream" }],
          },
        ],
      })) {
        t.assert.ok(event);
      }
    }, /Type mismatch at index 0/);
  });

  test("closing a one-shot stream early still cleans up its toolkit", async (t: TestContext) => {
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
    await stream.return({ content: [], output: [] });
    t.assert.strictEqual(closeCalls, 1);
  });
});
