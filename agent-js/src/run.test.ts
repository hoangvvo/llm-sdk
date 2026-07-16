/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/no-unsafe-member-access */
/* eslint-disable @typescript-eslint/no-unsafe-argument */
/* eslint-disable @typescript-eslint/no-unsafe-return */

import {
  LanguageModelError,
  type PartDelta,
  type PartialModelResponse,
} from "@hoangvvo/llm-sdk";
import { MockLanguageModel } from "@hoangvvo/llm-sdk/test";
import test, { suite, type TestContext } from "node:test";
import { setTimeout } from "node:timers/promises";
import {
  AgentCleanupError,
  AgentInitError,
  AgentInvariantError,
  AgentLanguageModelError,
  AgentMaxTurnsExceededError,
  AgentToolExecutionError,
} from "./errors.ts";
import { RunSession, type RunState } from "./run.ts";
import type { AgentFunctionTool, AgentTool } from "./tool.ts";
import type { Toolkit, ToolkitSession } from "./toolkit.ts";
import type { AgentResponse, AgentStreamEvent } from "./types.ts";

function createMockTool<TContext = any>(
  name: string,
  result: any,
  executeFn?: (args: any, ctx: TContext, state: RunState) => any,
): AgentTool<TContext> {
  return {
    type: "function",
    name,
    description: `Mock tool ${name}`,
    parameters: { type: "object", properties: {} },
    execute: executeFn ?? (() => result),
  };
}

function createPartialResponse(part: PartDelta): PartialModelResponse {
  return {
    delta: {
      index: 0,
      part,
    },
  };
}

function createMixedSnapshotPartials(): PartialModelResponse[] {
  return [
    {
      delta: { index: 0, part: { type: "text", text: "partial text" } },
    },
    {
      delta: {
        index: 1,
        part: {
          type: "tool-call",
          tool_call_id: "call_1",
          tool_name: "weather",
          args: '{"city":"Paris"}',
        },
      },
    },
    {
      delta: {
        index: 2,
        part: { type: "tool-call", args: "{incomplete" },
      },
    },
  ];
}

function createMixedSnapshotModelItem() {
  return {
    type: "model" as const,
    content: [
      { type: "text" as const, text: "partial text" },
      {
        type: "tool-call" as const,
        tool_call_id: "call_1",
        tool_name: "weather",
        args: { city: "Paris" },
      },
    ],
  };
}

function createMixedSnapshotCancelledToolItem() {
  return {
    type: "tool" as const,
    tool_call_id: "call_1",
    tool_name: "weather",
    input: { city: "Paris" },
    output: [],
    status: "cancelled" as const,
  };
}

function isPartialEvent(
  event: AgentStreamEvent,
): event is Extract<AgentStreamEvent, { event: "partial" }> {
  return event.event === "partial";
}

function isItemEvent(
  event: AgentStreamEvent,
): event is Extract<AgentStreamEvent, { event: "item" }> {
  return event.event === "item";
}

function isResponseEvent(
  event: AgentStreamEvent,
): event is Extract<AgentStreamEvent, { event: "response" }> {
  return event.event === "response";
}

suite("RunSession#run", () => {
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

  test("returns cancelled without calling the model when already aborted", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: { content: [{ type: "text", text: "ignored" }] },
    });
    const session = await RunSession.create({
      name: "test_agent",
      model,
      context: {},
    });
    const controller = new AbortController();
    controller.abort();

    const response = await session.run(
      {
        input: [
          {
            type: "message",
            role: "user",
            content: [{ type: "text", text: "Hello" }],
          },
        ],
      },
      { signal: controller.signal },
    );

    t.assert.deepStrictEqual(response, {
      content: [],
      output: [],
      status: "cancelled",
    });
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

  test("records cancelled tool results for the next run", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: {
        content: [
          {
            type: "tool-call",
            tool_call_id: "call_1",
            tool_name: "wait",
            args: {},
          },
          {
            type: "tool-call",
            tool_call_id: "call_2",
            tool_name: "wait",
            args: {},
          },
        ],
      },
    });
    let started: (() => void) | undefined;
    const toolStarted = new Promise<void>((resolve) => {
      started = resolve;
    });
    const waitTool: AgentFunctionTool<object> = {
      type: "function",
      name: "wait",
      description: "wait",
      parameters: { type: "object", properties: {} },
      execute: async (_args, _context, state) => {
        started?.();
        await setTimeout(60_000, undefined, { signal: state.signal });
        return { content: [], is_error: false };
      },
    };
    const session = await RunSession.create({
      name: "test_agent",
      model,
      tools: [waitTool],
      context: {},
    });
    const initial = {
      type: "message" as const,
      role: "user" as const,
      content: [{ type: "text" as const, text: "Wait" }],
    };
    const controller = new AbortController();
    const run = session.run(
      { input: [initial] },
      { signal: controller.signal },
    );
    await toolStarted;
    controller.abort();
    const response = await run;

    t.assert.strictEqual(response.status, "cancelled");
    t.assert.strictEqual(response.output.length, 3);
    for (const item of response.output.slice(1)) {
      if (item.type !== "tool") t.assert.fail("Expected a tool item");
      t.assert.strictEqual(item.status, "cancelled");
      t.assert.deepStrictEqual(item.output, []);
    }

    model.enqueueGenerateResult({
      response: { content: [{ type: "text", text: "continued" }] },
    });
    const nextSession = await RunSession.create({
      name: "test_agent",
      model,
      context: {},
    });
    await nextSession.run({
      input: [
        initial,
        ...response.output,
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Continue" }],
        },
      ],
    });

    const toolMessage = model.trackedGenerateInputs[1]?.messages[2];
    if (toolMessage?.role !== "tool") t.assert.fail("Expected a tool message");
    t.assert.strictEqual(toolMessage.content.length, 2);
    for (const part of toolMessage.content) {
      if (part.type !== "tool-result") t.assert.fail("Expected a tool result");
      t.assert.strictEqual(part.status, "cancelled");
      t.assert.deepStrictEqual(part.content, []);
    }
  });

  test("does not start later tools after a non-cooperative tool finishes", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: {
        content: [
          {
            type: "tool-call",
            tool_call_id: "call_1",
            tool_name: "first",
            args: {},
          },
          {
            type: "tool-call",
            tool_call_id: "call_2",
            tool_name: "second",
            args: {},
          },
        ],
      },
    });

    let markStarted: (() => void) | undefined;
    const started = new Promise<void>((resolve) => {
      markStarted = resolve;
    });
    let releaseFirst: (() => void) | undefined;
    const released = new Promise<void>((resolve) => {
      releaseFirst = resolve;
    });
    const first: AgentFunctionTool<object> = {
      type: "function",
      name: "first",
      description: "first",
      parameters: { type: "object", properties: {} },
      execute: async () => {
        markStarted?.();
        await released;
        return {
          content: [{ type: "text", text: "first finished" }],
          is_error: false,
        };
      },
    };
    const secondExecute = t.mock.fn(() => ({
      content: [{ type: "text" as const, text: "second finished" }],
      is_error: false,
    }));
    const second: AgentFunctionTool<object> = {
      type: "function",
      name: "second",
      description: "second",
      parameters: { type: "object", properties: {} },
      execute: secondExecute,
    };
    const session = await RunSession.create({
      name: "test_agent",
      model,
      tools: [first, second],
      context: {},
    });
    const controller = new AbortController();
    const run = session.run(
      {
        input: [
          {
            type: "message",
            role: "user",
            content: [{ type: "text", text: "Run both tools" }],
          },
        ],
      },
      { signal: controller.signal },
    );

    await started;
    controller.abort();
    releaseFirst?.();
    const response = await run;

    t.assert.strictEqual(response.status, "cancelled");
    t.assert.strictEqual(response.output.length, 3);
    const firstItem = response.output[1];
    const secondItem = response.output[2];
    if (firstItem?.type !== "tool" || secondItem?.type !== "tool") {
      t.assert.fail("Expected completed and cancelled tool items");
    }
    t.assert.strictEqual(firstItem.status, "completed");
    t.assert.strictEqual(secondItem.status, "cancelled");
    t.assert.strictEqual(secondExecute.mock.callCount(), 0);
  });

  test("returns a response when there is no tool call", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: { content: [{ type: "text", text: "Hi!" }] },
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [],
      context: {},
    });

    const response = await session.run({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Hello!" }],
        },
      ],
    });

    t.assert.deepStrictEqual(response, {
      status: "completed",
      content: [{ type: "text", text: "Hi!" }],
      output: [
        {
          type: "model",
          content: [
            {
              type: "text",
              text: "Hi!",
            },
          ],
        },
      ],
    });
  });

  test("executes a single tool call and returns response", async (t: TestContext) => {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const toolExecute = t.mock.fn((_args, _context) => ({
      content: [{ type: "text", text: "Tool result" }],
      is_error: false,
    }));

    const tool = createMockTool<{ testContext: boolean }>(
      "test_tool",
      null,
      toolExecute,
    );

    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: {
        content: [
          {
            type: "tool-call",
            tool_name: "test_tool",
            tool_call_id: "call_1",
            args: { param: "value" },
          },
        ],
        usage: {
          input_tokens: 1000,
          output_tokens: 50,
        },
        cost: 0.0015,
      },
    });
    model.enqueueGenerateResult({
      response: {
        content: [{ type: "text", text: "Final response" }],
      },
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [tool],
      context: { testContext: true },
    });

    const response = await session.run({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Use the tool" }],
        },
      ],
    });

    // The bound context should reach tool executions even though input items are supplied per run.
    const toolCallArguments = toolExecute.mock.calls.map((call) =>
      call.arguments.slice(0, 2),
    );
    t.assert.deepStrictEqual(toolCallArguments, [
      [{ param: "value" }, { testContext: true }],
    ]);

    t.assert.deepStrictEqual(response, {
      status: "completed",
      content: [{ type: "text", text: "Final response" }],
      output: [
        {
          type: "model",
          content: [
            {
              type: "tool-call",
              tool_name: "test_tool",
              tool_call_id: "call_1",
              args: { param: "value" },
            },
          ],
          usage: {
            input_tokens: 1000,
            output_tokens: 50,
          },
          cost: 0.0015,
        },
        {
          type: "tool",
          tool_name: "test_tool",
          tool_call_id: "call_1",
          input: { param: "value" },
          output: [{ type: "text", text: "Tool result" }],
          status: "completed",
        },
        {
          type: "model",
          content: [{ type: "text", text: "Final response" }],
        },
      ],
    });
  });

  test("returns existing assistant response without generating a new model output", async (t: TestContext) => {
    const model = new MockLanguageModel();

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [],
      context: {},
    });

    const response = await session.run({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "What did I say?" }],
        },
        {
          type: "message",
          role: "assistant",
          content: [{ type: "text", text: "Cached answer" }],
        },
      ],
    });

    t.assert.deepStrictEqual(response, {
      status: "completed",
      content: [{ type: "text", text: "Cached answer" }],
      output: [],
    });
  });

  test("executes multiple tool calls from one model response", async (t: TestContext) => {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const tool1Execute = t.mock.fn((_args) => ({
      content: [{ type: "text", text: "Tool 1 result" }],
      is_error: false,
    }));
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const tool2Execute = t.mock.fn((_args) => ({
      content: [{ type: "text", text: "Tool 2 result" }],
      is_error: false,
    }));

    const tool1 = createMockTool<object>("tool_1", null, tool1Execute);
    const tool2 = createMockTool<object>("tool_2", null, tool2Execute);

    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: {
        content: [
          {
            type: "tool-call",
            tool_name: "tool_1",
            tool_call_id: "call_1",
            args: { param: "value1" },
          },
          {
            type: "tool-call",
            tool_name: "tool_2",
            tool_call_id: "call_2",
            args: { param: "value2" },
          },
        ],
        usage: {
          input_tokens: 2000,
          output_tokens: 100,
        },
      },
    });
    model.enqueueGenerateResult({
      response: {
        content: [{ type: "text", text: "Processed both tools" }],
        usage: {
          input_tokens: 50,
          output_tokens: 10,
        },
        cost: 0.0003,
      },
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [tool1, tool2],
      context: {},
    });

    const response = await session.run({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Use both tools" }],
        },
      ],
    });

    t.assert.deepStrictEqual(
      tool1Execute.mock.calls.map((call) => call.arguments[0]),
      [{ param: "value1" }],
    );
    t.assert.deepStrictEqual(
      tool2Execute.mock.calls.map((call) => call.arguments[0]),
      [{ param: "value2" }],
    );

    const expectedResponse: AgentResponse = {
      status: "completed",
      content: [{ type: "text", text: "Processed both tools" }],
      output: [
        {
          type: "model",
          content: [
            {
              type: "tool-call",
              tool_name: "tool_1",
              tool_call_id: "call_1",
              args: { param: "value1" },
            },
            {
              type: "tool-call",
              tool_name: "tool_2",
              tool_call_id: "call_2",
              args: { param: "value2" },
            },
          ],
          usage: {
            input_tokens: 2000,
            output_tokens: 100,
          },
        },
        {
          type: "tool",
          tool_call_id: "call_1",
          tool_name: "tool_1",
          input: { param: "value1" },
          output: [
            {
              type: "text",
              text: "Tool 1 result",
            },
          ],
          status: "completed",
        },
        {
          type: "tool",
          tool_call_id: "call_2",
          tool_name: "tool_2",
          input: { param: "value2" },
          output: [
            {
              type: "text",
              text: "Tool 2 result",
            },
          ],
          status: "completed",
        },
        {
          type: "model",
          content: [
            {
              type: "text",
              text: "Processed both tools",
            },
          ],
          usage: {
            input_tokens: 50,
            output_tokens: 10,
          },
          cost: 0.0003,
        },
      ],
    };

    t.assert.deepStrictEqual(response, expectedResponse);
  });

  test("handles multiple turns with tool calls", async (t: TestContext) => {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const toolExecute = t.mock.fn((_args) => ({
      content: [{ type: "text", text: "Calculation result" }],
      is_error: false,
    }));

    const tool = createMockTool<object>("calculator", null, toolExecute);

    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: {
        content: [
          {
            type: "tool-call",
            tool_name: "calculator",
            tool_call_id: "call_1",
            args: { operation: "add", a: 1, b: 2 },
          },
        ],
      },
    });
    model.enqueueGenerateResult({
      response: {
        content: [
          {
            type: "tool-call",
            tool_name: "calculator",
            tool_call_id: "call_2",
            args: { operation: "multiply", a: 3, b: 4 },
          },
        ],
      },
    });
    model.enqueueGenerateResult({
      response: {
        content: [{ type: "text", text: "All calculations done" }],
      },
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [tool],
      context: {},
    });

    const response = await session.run({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Calculate some numbers" }],
        },
      ],
    });

    t.assert.deepStrictEqual(
      toolExecute.mock.calls.map((call) => call.arguments[0]),
      [
        { operation: "add", a: 1, b: 2 },
        { operation: "multiply", a: 3, b: 4 },
      ],
    );

    const expectedResponse: AgentResponse = {
      status: "completed",
      content: [{ type: "text", text: "All calculations done" }],
      output: [
        {
          type: "model",
          content: [
            {
              type: "tool-call",
              tool_name: "calculator",
              tool_call_id: "call_1",
              args: { operation: "add", a: 1, b: 2 },
            },
          ],
        },
        {
          type: "tool",
          tool_name: "calculator",
          tool_call_id: "call_1",
          input: { operation: "add", a: 1, b: 2 },
          output: [{ type: "text", text: "Calculation result" }],
          status: "completed",
        },
        {
          type: "model",
          content: [
            {
              type: "tool-call",
              tool_name: "calculator",
              tool_call_id: "call_2",
              args: { operation: "multiply", a: 3, b: 4 },
            },
          ],
        },
        {
          type: "tool",
          tool_name: "calculator",
          tool_call_id: "call_2",
          input: { operation: "multiply", a: 3, b: 4 },
          output: [{ type: "text", text: "Calculation result" }],
          status: "completed",
        },
        {
          type: "model",
          content: [{ type: "text", text: "All calculations done" }],
        },
      ],
    };

    t.assert.deepStrictEqual(response, expectedResponse);
  });

  test("throws AgentMaxTurnsExceededError when max turns exceeded", async (t: TestContext) => {
    const toolExecute = t.mock.fn(() => ({
      content: [{ type: "text", text: "Tool result" }],
      is_error: false,
    }));

    const tool = createMockTool<object>("test_tool", null, toolExecute);

    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: {
        content: [
          {
            type: "tool-call",
            tool_name: "test_tool",
            tool_call_id: "call_1",
            args: {},
          },
        ],
      },
    });
    model.enqueueGenerateResult({
      response: {
        content: [
          {
            type: "tool-call",
            tool_name: "test_tool",
            tool_call_id: "call_2",
            args: {},
          },
        ],
      },
    });
    model.enqueueGenerateResult({
      response: {
        content: [
          {
            type: "tool-call",
            tool_name: "test_tool",
            tool_call_id: "call_3",
            args: {},
          },
        ],
      },
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 2,
      response_format: { type: "text" },
      tools: [tool],
      context: {},
    });

    await t.assert.rejects(
      async () => {
        await session.run({
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "Keep using tools" }],
            },
          ],
        });
      },
      (err: any) => {
        t.assert.strictEqual(err instanceof AgentMaxTurnsExceededError, true);
        t.assert.match(err.message, /maximum number of turns.*2.*exceeded/);
        return true;
      },
    );
  });

  test("throws AgentInvariantError when tool not found", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: {
        content: [
          {
            type: "tool-call",
            tool_name: "non_existent_tool",
            tool_call_id: "call_1",
            args: {},
          },
        ],
      },
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [],
      context: {},
    });

    await t.assert.rejects(
      async () => {
        await session.run({
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "Use a tool" }],
            },
          ],
        });
      },
      (err: any) => {
        t.assert.strictEqual(err instanceof AgentInvariantError, true);
        t.assert.match(err.message, /Tool non_existent_tool not found/);
        return true;
      },
    );
  });

  test("throws AgentToolExecutionError when tool execution fails", async (t: TestContext) => {
    const toolExecute = t.mock.fn(() => {
      throw new Error("Tool execution failed");
    });

    const tool = createMockTool<object>("failing_tool", null, toolExecute);

    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: {
        content: [
          {
            type: "tool-call",
            tool_name: "failing_tool",
            tool_call_id: "call_1",
            args: {},
          },
        ],
      },
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [tool],
      context: {},
    });

    await t.assert.rejects(
      async () => {
        await session.run({
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "Use the tool" }],
            },
          ],
        });
      },
      (err: any) => {
        t.assert.strictEqual(err instanceof AgentToolExecutionError, true);
        t.assert.match(err.message, /Tool execution failed/);
        return true;
      },
    );
  });

  test("resumes tool processing from tool message with partial results", async (t: TestContext) => {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const toolExecute = t.mock.fn((_: object) => ({
      content: [{ type: "text", text: "call_2 result" }],
      is_error: false,
    }));

    const tool = createMockTool<object>("resume_tool", null, toolExecute);

    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: {
        content: [{ type: "text", text: "Final reply" }],
      },
    });

    const session = await RunSession.create({
      name: "resumable",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [tool],
      context: {},
    });

    const response = await session.run({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Continue" }],
        },
        {
          type: "model",
          content: [
            {
              type: "tool-call",
              tool_name: "resume_tool",
              tool_call_id: "call_1",
              args: { step: 1 },
            },
            {
              type: "tool-call",
              tool_name: "resume_tool",
              tool_call_id: "call_2",
              args: { step: 2 },
            },
          ],
        },
        {
          type: "message",
          role: "tool",
          content: [
            {
              type: "tool-result",
              tool_name: "resume_tool",
              tool_call_id: "call_1",
              content: [{ type: "text", text: "already done" }],
              status: "completed",
            },
          ],
        },
      ],
    });

    t.assert.strictEqual(toolExecute.mock.calls.length, 1);
    t.assert.deepStrictEqual(toolExecute.mock.calls[0]?.arguments[0], {
      step: 2,
    });

    t.assert.deepStrictEqual(response, {
      status: "completed",
      content: [{ type: "text", text: "Final reply" }],
      output: [
        {
          type: "tool",
          tool_name: "resume_tool",
          tool_call_id: "call_2",
          input: { step: 2 },
          output: [{ type: "text", text: "call_2 result" }],
          status: "completed",
        },
        {
          type: "model",
          content: [{ type: "text", text: "Final reply" }],
        },
      ],
    });
  });

  test("resumes tool processing when trailing items are individual tool entries", async (t: TestContext) => {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const toolExecute = t.mock.fn((_args: object) => ({
      content: [{ type: "text", text: "call_2 via item" }],
      is_error: false,
    }));

    const tool = createMockTool<object>("resume_tool", null, toolExecute);

    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: {
        content: [{ type: "text", text: "Final reply from items" }],
      },
    });

    const session = await RunSession.create({
      name: "resumable_tool_items",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [tool],
      context: {},
    });

    const response = await session.run({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Continue" }],
        },
        {
          type: "model",
          content: [
            {
              type: "tool-call",
              tool_name: "resume_tool",
              tool_call_id: "call_1",
              args: { stage: 1 },
            },
            {
              type: "tool-call",
              tool_name: "resume_tool",
              tool_call_id: "call_2",
              args: { stage: 2 },
            },
          ],
        },
        {
          type: "tool",
          tool_name: "resume_tool",
          tool_call_id: "call_1",
          input: { stage: 1 },
          output: [{ type: "text", text: "already done" }],
          status: "completed",
        },
      ],
    });

    t.assert.strictEqual(toolExecute.mock.calls.length, 1);
    t.assert.deepStrictEqual(toolExecute.mock.calls[0]?.arguments[0], {
      stage: 2,
    });

    t.assert.deepStrictEqual(response, {
      status: "completed",
      content: [{ type: "text", text: "Final reply from items" }],
      output: [
        {
          type: "tool",
          tool_name: "resume_tool",
          tool_call_id: "call_2",
          input: { stage: 2 },
          output: [{ type: "text", text: "call_2 via item" }],
          status: "completed",
        },
        {
          type: "model",
          content: [{ type: "text", text: "Final reply from items" }],
        },
      ],
    });
  });

  test("throws AgentInvariantError when tool results lack preceding assistant content", async (t: TestContext) => {
    const tool = createMockTool<object>("resume_tool", null, () => ({
      content: [{ type: "text", text: "unused" }],
      is_error: false,
    }));

    const model = new MockLanguageModel();

    const session = await RunSession.create({
      name: "resumable_error",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [tool],
      context: {},
    });

    await t.assert.rejects(
      async () => {
        await session.run({
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "Resume" }],
            },
            {
              type: "message",
              role: "tool",
              content: [
                {
                  type: "tool-result",
                  tool_name: "resume_tool",
                  tool_call_id: "call_1",
                  content: [{ type: "text", text: "orphan" }],
                  status: "completed",
                },
              ],
            },
          ],
        });
      },
      (err: any) => {
        t.assert.strictEqual(err instanceof AgentInvariantError, true);
        t.assert.match(
          err.message,
          /Expected a model item or assistant message before tool results/,
        );
        return true;
      },
    );
  });

  test("handles tool returning error result", async (t: TestContext) => {
    const toolExecute = t.mock.fn((args: Record<string, unknown>) => {
      t.assert.deepStrictEqual(args, { invalid: true });
      return {
        content: [{ type: "text" as const, text: "Error: Invalid parameters" }],
        is_error: true,
      };
    });

    const tool = createMockTool<object>("test_tool", null, toolExecute);

    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: {
        content: [
          {
            type: "tool-call",
            tool_name: "test_tool",
            tool_call_id: "call_1",
            args: { invalid: true },
          },
        ],
      },
    });
    model.enqueueGenerateResult({
      response: {
        content: [{ type: "text", text: "Handled the error" }],
      },
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [tool],
      context: {},
    });

    const response = await session.run({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Use the tool" }],
        },
      ],
    });

    t.assert.strictEqual(toolExecute.mock.callCount(), 1);

    const expectedResponse: AgentResponse = {
      status: "completed",
      content: [{ type: "text", text: "Handled the error" }],
      output: [
        {
          type: "model",
          content: [
            {
              type: "tool-call",
              tool_name: "test_tool",
              tool_call_id: "call_1",
              args: { invalid: true },
            },
          ],
        },
        {
          type: "tool",
          tool_name: "test_tool",
          tool_call_id: "call_1",
          input: { invalid: true },
          output: [{ type: "text", text: "Error: Invalid parameters" }],
          status: "failed",
        },
        {
          type: "model",
          content: [{ type: "text", text: "Handled the error" }],
        },
      ],
    };

    t.assert.deepStrictEqual(response, expectedResponse);
  });

  test("throws error when session not initialized", async (t: TestContext) => {
    const model = new MockLanguageModel();

    const session = new RunSession({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [],
      context: {},
    });

    await t.assert.rejects(
      async () => {
        await session.run({
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "Hello" }],
            },
          ],
        });
      },
      (err: any) => {
        t.assert.match(err.message, /RunSession not initialized/);
        return true;
      },
    );
  });

  test("passes sampling parameters to model", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: { content: [{ type: "text", text: "Response" }] },
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [],
      temperature: 0.7,
      top_p: 0.9,
      top_k: 40,
      presence_penalty: 0.1,
      frequency_penalty: 0.2,
      context: {},
    });

    await session.run({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Hello" }],
        },
      ],
    });

    const samplingInputs = model.trackedGenerateInputs.map((input) => ({
      temperature: input.temperature,
      top_p: input.top_p,
      top_k: input.top_k,
      presence_penalty: input.presence_penalty,
      frequency_penalty: input.frequency_penalty,
    }));
    t.assert.deepStrictEqual(samplingInputs, [
      {
        temperature: 0.7,
        top_p: 0.9,
        top_k: 40,
        presence_penalty: 0.1,
        frequency_penalty: 0.2,
      },
    ]);
  });

  test("passes provider-hosted tools to the model", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: { content: [{ type: "text", text: "Search complete" }] },
    });
    const webSearchTool = {
      type: "web_search" as const,
      allowed_domains: ["example.com"],
    };

    const session = await RunSession.create({
      name: "test_agent",
      model,
      tools: [webSearchTool],
      context: {},
    });

    await session.run({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Find an example" }],
        },
      ],
    });

    t.assert.deepStrictEqual(model.trackedGenerateInputs[0]?.tools, [
      webSearchTool,
    ]);
  });

  test("throws LanguageModelError when non-streaming generation fails", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      error: new LanguageModelError("API quota exceeded"),
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [],
      context: {},
    });

    await t.assert.rejects(
      async () => {
        await session.run({
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "Hello" }],
            },
          ],
        });
      },
      (err: any) => {
        t.assert.strictEqual(err instanceof AgentLanguageModelError, true);
        t.assert.match(err.message, /API quota exceeded/);
        return true;
      },
    );
  });

  test("includes string and dynamic function instructions in system prompt", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: { content: [{ type: "text", text: "Response" }] },
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [
        "You are a helpful assistant.",
        (ctx: { userRole: string }) => `The user is a ${ctx.userRole}.`,
        "Always be polite.",
      ],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [],
      context: { userRole: "developer" },
    });

    await session.run({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Hello" }],
        },
      ],
    });

    const systemPrompts = model.trackedGenerateInputs.map(
      (input) => input.system_prompt,
    );
    t.assert.deepStrictEqual(systemPrompts, [
      "You are a helpful assistant.\nThe user is a developer.\nAlways be polite.",
    ]);
  });

  test("merges toolkit prompts and tools with run context", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueGenerateResult(
      {
        response: {
          content: [
            {
              type: "tool-call",
              tool_name: "lookup-order",
              tool_call_id: "call-1",
              args: { orderId: "123" },
            },
          ],
        },
      },
      {
        response: {
          content: [{ type: "text", text: "Order ready" }],
        },
      },
    );

    interface Context {
      customer: string;
    }
    const context: Context = { customer: "Ada" };
    const createdContexts: Context[] = [];
    let closeCalls = 0;
    const executed: {
      ctx: Context;
      args: Record<string, unknown>;
      turn: number;
    }[] = [];

    const dynamicTool: AgentTool<Context, { orderId: string }> = {
      type: "function",
      name: "lookup-order",
      description: "Lookup an order by ID",
      parameters: {
        type: "object",
        properties: {
          orderId: { type: "string" },
        },
        required: ["orderId"],
        additionalProperties: false,
      },
      execute(args, ctx, state) {
        executed.push({ ctx, args, turn: state.currentTurn });
        return Promise.resolve({
          content: [
            {
              type: "text",
              text: `Order ${args.orderId} ready for ${ctx.customer}`,
            },
          ],
          is_error: false,
        });
      },
    };

    const toolkitSession: ToolkitSession<Context> = {
      getSystemPrompt() {
        return "Toolkit prompt";
      },
      getTools() {
        return [dynamicTool];
      },
      close() {
        closeCalls += 1;
        return Promise.resolve();
      },
    };

    const toolkit: Toolkit<Context> = {
      createSession(ctx) {
        createdContexts.push(ctx);
        return Promise.resolve(toolkitSession);
      },
    };

    const session = await RunSession.create({
      name: "toolkit-agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [],
      toolkits: [toolkit],
      context,
    });

    const response = await session.run({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Status?" }],
        },
      ],
    });

    t.assert.deepStrictEqual(createdContexts, [context]);
    t.assert.deepStrictEqual(executed, [
      {
        ctx: context,
        args: { orderId: "123" },
        turn: 1,
      },
    ]);

    t.assert.strictEqual(model.trackedGenerateInputs.length, 2);
    for (const input of model.trackedGenerateInputs) {
      t.assert.strictEqual(input.system_prompt, "Toolkit prompt");
      t.assert.deepStrictEqual(input.tools, [
        {
          type: "function",
          name: "lookup-order",
          description: "Lookup an order by ID",
          parameters: dynamicTool.parameters,
        },
      ]);
    }
    t.assert.deepStrictEqual(response, {
      status: "completed",
      content: [{ type: "text", text: "Order ready" }],
      output: [
        {
          type: "model",
          content: [
            {
              type: "tool-call",
              tool_name: "lookup-order",
              tool_call_id: "call-1",
              args: { orderId: "123" },
            },
          ],
        },
        {
          type: "tool",
          tool_name: "lookup-order",
          tool_call_id: "call-1",
          input: { orderId: "123" },
          output: [
            {
              type: "text",
              text: "Order 123 ready for Ada",
            },
          ],
          status: "completed",
        },
        {
          type: "model",
          content: [{ type: "text", text: "Order ready" }],
        },
      ],
    });

    await session.close();
    t.assert.strictEqual(closeCalls, 1);
  });
});

suite("RunSession#runStream", () => {
  test("returns cancelled without streaming when already aborted", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueStreamResult({
      partials: [createPartialResponse({ type: "text", text: "ignored" })],
    });
    const session = await RunSession.create({
      name: "test_agent",
      model,
      context: {},
    });
    const controller = new AbortController();
    controller.abort();
    const events: AgentStreamEvent[] = [];

    for await (const event of session.runStream(
      {
        input: [
          {
            type: "message",
            role: "user",
            content: [{ type: "text", text: "Hello" }],
          },
        ],
      },
      { signal: controller.signal },
    )) {
      events.push(event);
    }

    t.assert.deepStrictEqual(events, [
      { event: "response", content: [], output: [], status: "cancelled" },
    ]);
    t.assert.strictEqual(model.trackedStreamInputs.length, 0);
  });

  test("turns invalid delta sequences into invariant errors", async (t: TestContext) => {
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

  test("streams response when there is no tool call", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueStreamResult({
      partials: [
        createPartialResponse({ type: "text", text: "Hel" }),
        createPartialResponse({ type: "text", text: "lo" }),
        createPartialResponse({ type: "text", text: "!" }),
      ],
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [],
      context: {},
    });

    const events: AgentStreamEvent[] = [];
    const generator = session.runStream({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Hi" }],
        },
      ],
    });

    for await (const event of generator) {
      events.push(event);
    }

    t.assert.deepStrictEqual(
      events.map((event) => event.event),
      ["partial", "partial", "partial", "item", "response"],
    );

    const partialEvents = events.filter(isPartialEvent);
    t.assert.deepStrictEqual(
      partialEvents.map((event) => event.delta),
      [
        { index: 0, part: { type: "text", text: "Hel" } },
        { index: 0, part: { type: "text", text: "lo" } },
        { index: 0, part: { type: "text", text: "!" } },
      ],
    );

    t.assert.deepStrictEqual(events.filter(isItemEvent), [
      {
        event: "item",
        index: 0,
        item: {
          type: "model",
          content: [{ type: "text", text: "Hello!" }],
        },
      },
    ]);
    t.assert.deepStrictEqual(events.find(isResponseEvent), {
      event: "response",
      status: "completed",
      content: [{ type: "text", text: "Hello!" }],
      output: [
        {
          type: "model",
          content: [{ type: "text", text: "Hello!" }],
        },
      ],
    });
  });

  test("streams tool call execution and response", async (t: TestContext) => {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const toolExecute = t.mock.fn((_args) => ({
      content: [{ type: "text", text: "Tool result" }],
      is_error: false,
    }));

    const tool = createMockTool<object>("test_tool", null, toolExecute);

    const model = new MockLanguageModel();
    model.enqueueStreamResult({
      partials: [
        createPartialResponse({
          type: "tool-call",
          tool_name: "test_tool",
          tool_call_id: "call_1",
          args: JSON.stringify({ a: 1, b: 2, operation: "add" }),
        }),
      ],
    });
    model.enqueueStreamResult({
      partials: [
        createPartialResponse({ type: "text", text: "Final" }),
        createPartialResponse({ type: "text", text: "Final response" }),
      ],
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [tool],
      context: {},
    });

    const events = [];
    const generator = session.runStream({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Use tool" }],
        },
      ],
    });

    for await (const event of generator) {
      events.push(event);
    }

    t.assert.deepStrictEqual(
      events.map((event) => event.event),
      ["partial", "item", "item", "partial", "partial", "item", "response"],
    );

    const partialEvents = events.filter(isPartialEvent);
    t.assert.deepStrictEqual(
      partialEvents.map((event) => event.delta),
      [
        {
          index: 0,
          part: {
            type: "tool-call",
            tool_name: "test_tool",
            tool_call_id: "call_1",
            args: JSON.stringify({ a: 1, b: 2, operation: "add" }),
          },
        },
        { index: 0, part: { type: "text", text: "Final" } },
        { index: 0, part: { type: "text", text: "Final response" } },
      ],
    );

    const expectedItemEvents = [
      {
        event: "item",
        index: 0,
        item: {
          type: "model",
          content: [
            {
              type: "tool-call",
              tool_name: "test_tool",
              tool_call_id: "call_1",
              args: { a: 1, b: 2, operation: "add" },
            },
          ],
        },
      },
      {
        event: "item",
        index: 1,
        item: {
          type: "tool",
          tool_name: "test_tool",
          tool_call_id: "call_1",
          input: { a: 1, b: 2, operation: "add" },
          output: [{ type: "text", text: "Tool result" }],
          status: "completed",
        },
      },
      {
        event: "item",
        index: 2,
        item: {
          type: "model",
          content: [{ type: "text", text: "FinalFinal response" }],
        },
      },
    ];

    t.assert.deepStrictEqual(events.filter(isItemEvent), expectedItemEvents);

    const responseEvent = events.find(isResponseEvent);
    t.assert.deepStrictEqual(responseEvent, {
      event: "response",
      status: "completed",
      content: [{ type: "text", text: "FinalFinal response" }],
      output: [
        {
          type: "model",
          content: [
            {
              type: "tool-call",
              tool_name: "test_tool",
              tool_call_id: "call_1",
              args: { a: 1, b: 2, operation: "add" },
            },
          ],
        },
        {
          type: "tool",
          tool_name: "test_tool",
          tool_call_id: "call_1",
          input: { a: 1, b: 2, operation: "add" },
          output: [{ type: "text", text: "Tool result" }],
          status: "completed",
        },
        {
          type: "model",
          content: [{ type: "text", text: "FinalFinal response" }],
        },
      ],
    });

    t.assert.deepStrictEqual(
      toolExecute.mock.calls.map((call) => call.arguments[0]),
      [{ a: 1, b: 2, operation: "add" }],
    );
  });

  test("handles multiple turns in streaming mode", async (t: TestContext) => {
    const toolExecute = t.mock.fn((args: Record<string, unknown>) => {
      t.assert.ok("a" in args && "b" in args);
      return {
        content: [{ type: "text" as const, text: "Calculation done" }],
        is_error: false,
      };
    });

    const tool = createMockTool<object>("calculator", null, toolExecute);

    const model = new MockLanguageModel();
    model.enqueueStreamResult({
      partials: [
        createPartialResponse({
          type: "tool-call",
          tool_name: "calculator",
          tool_call_id: "call_1",
          args: JSON.stringify({ a: 1, b: 2 }),
        }),
      ],
    });
    model.enqueueStreamResult({
      partials: [
        createPartialResponse({
          type: "tool-call",
          tool_name: "calculator",
          tool_call_id: "call_2",
          args: JSON.stringify({ a: 3, b: 4 }),
        }),
      ],
    });
    model.enqueueStreamResult({
      partials: [createPartialResponse({ type: "text", text: "All done" })],
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [tool],
      context: {},
    });

    const events = [];
    const generator = session.runStream({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Calculate" }],
        },
      ],
    });

    for await (const event of generator) {
      events.push(event);
    }

    t.assert.deepStrictEqual(
      events.map((event) => event.event),
      [
        "partial",
        "item",
        "item",
        "partial",
        "item",
        "item",
        "partial",
        "item",
        "response",
      ],
    );

    const expectedItemEvents = [
      {
        event: "item",
        index: 0,
        item: {
          type: "model",
          content: [
            {
              type: "tool-call",
              tool_name: "calculator",
              tool_call_id: "call_1",
              args: { a: 1, b: 2 },
            },
          ],
        },
      },
      {
        event: "item",
        index: 1,
        item: {
          type: "tool",
          tool_name: "calculator",
          tool_call_id: "call_1",
          input: { a: 1, b: 2 },
          output: [{ type: "text", text: "Calculation done" }],
          status: "completed",
        },
      },
      {
        event: "item",
        index: 2,
        item: {
          type: "model",
          content: [
            {
              type: "tool-call",
              tool_name: "calculator",
              tool_call_id: "call_2",
              args: { a: 3, b: 4 },
            },
          ],
        },
      },
      {
        event: "item",
        index: 3,
        item: {
          type: "tool",
          tool_name: "calculator",
          tool_call_id: "call_2",
          input: { a: 3, b: 4 },
          output: [{ type: "text", text: "Calculation done" }],
          status: "completed",
        },
      },
      {
        event: "item",
        index: 4,
        item: {
          type: "model",
          content: [{ type: "text", text: "All done" }],
        },
      },
    ];
    t.assert.deepStrictEqual(events.filter(isItemEvent), expectedItemEvents);

    t.assert.deepStrictEqual(
      toolExecute.mock.calls.map((call) => call.arguments[0]),
      [
        { a: 1, b: 2 },
        { a: 3, b: 4 },
      ],
    );

    const responseEvent = events.at(-1);
    t.assert.deepStrictEqual(responseEvent, {
      event: "response",
      status: "completed",
      content: [{ type: "text", text: "All done" }],
      output: [
        {
          type: "model",
          content: [
            {
              type: "tool-call",
              tool_name: "calculator",
              tool_call_id: "call_1",
              args: { a: 1, b: 2 },
            },
          ],
        },
        {
          type: "tool",
          tool_name: "calculator",
          tool_call_id: "call_1",
          input: { a: 1, b: 2 },
          output: [{ type: "text", text: "Calculation done" }],
          status: "completed",
        },
        {
          type: "model",
          content: [
            {
              type: "tool-call",
              tool_name: "calculator",
              tool_call_id: "call_2",
              args: { a: 3, b: 4 },
            },
          ],
        },
        {
          type: "tool",
          tool_name: "calculator",
          tool_call_id: "call_2",
          input: { a: 3, b: 4 },
          output: [{ type: "text", text: "Calculation done" }],
          status: "completed",
        },
        {
          type: "model",
          content: [{ type: "text", text: "All done" }],
        },
      ],
    });
  });

  test("throws AgentMaxTurnsExceededError in streaming mode", async (t: TestContext) => {
    const toolExecute = t.mock.fn(() => ({
      content: [{ type: "text", text: "Tool result" }],
      is_error: false,
    }));

    const tool = createMockTool<object>("test_tool", null, toolExecute);

    const model = new MockLanguageModel();
    model.enqueueStreamResult({
      partials: [
        createPartialResponse({
          type: "tool-call",
          tool_name: "test_tool",
          tool_call_id: "call_1",
          args: "{}",
        }),
      ],
    });
    model.enqueueStreamResult({
      partials: [
        createPartialResponse({
          type: "tool-call",
          tool_name: "test_tool",
          tool_call_id: "call_2",
          args: "{}",
        }),
      ],
    });
    model.enqueueStreamResult({
      partials: [
        createPartialResponse({
          type: "tool-call",
          tool_name: "test_tool",
          tool_call_id: "call_3",
          args: "{}",
        }),
      ],
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 2,
      response_format: { type: "text" },
      tools: [tool],
      context: {},
    });

    const generator = session.runStream({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Keep using tools" }],
        },
      ],
    });

    await t.assert.rejects(
      async () => {
        // eslint-disable-next-line @typescript-eslint/no-unused-vars
        for await (const _event of generator) {
          // consume events
        }
      },
      (err: any) => {
        t.assert.strictEqual(err instanceof AgentMaxTurnsExceededError, true);
        return true;
      },
    );
  });

  test("throws AgentLanguageModelError when streaming fails", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueStreamResult({
      error: new LanguageModelError("Rate limit exceeded"),
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [],
      context: {},
    });

    const generator = session.runStream({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Hello" }],
        },
      ],
    });

    await t.assert.rejects(
      async () => {
        // eslint-disable-next-line @typescript-eslint/no-unused-vars
        for await (const _event of generator) {
          // consume events
        }
      },
      (err: any) => {
        t.assert.strictEqual(err instanceof AgentLanguageModelError, true);
        t.assert.match(
          err.message,
          /Language model error.*Rate limit exceeded/,
        );
        return true;
      },
    );
  });

  test("commits materializable partial content before streaming errors", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueStreamResult({
      partials: createMixedSnapshotPartials(),
      error: new LanguageModelError("stream failed"),
    });
    const session = await RunSession.create({
      name: "test_agent",
      model,
      context: {},
    });
    const events: AgentStreamEvent[] = [];

    await t.assert.rejects(
      async () => {
        for await (const event of session.runStream({
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
        t.assert.strictEqual(error instanceof AgentLanguageModelError, true);
        if (!(error instanceof AgentLanguageModelError)) return false;
        t.assert.deepStrictEqual(error.snapshot, {
          output: [createMixedSnapshotModelItem()],
        });
        return true;
      },
    );

    t.assert.strictEqual(events.length, 4);
    t.assert.strictEqual(events.slice(0, 3).every(isPartialEvent), true);
    t.assert.deepStrictEqual(events[3], {
      event: "item",
      index: 0,
      item: createMixedSnapshotModelItem(),
    });
  });

  test("records cancelled results for materialized streamed tool calls", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueStreamResult({ partials: createMixedSnapshotPartials() });
    const session = await RunSession.create({
      name: "test_agent",
      model,
      context: {},
    });
    const controller = new AbortController();
    const generator = session.runStream(
      {
        input: [
          {
            type: "message",
            role: "user",
            content: [{ type: "text", text: "Hello" }],
          },
        ],
      },
      { signal: controller.signal },
    );

    for (let index = 0; index < 3; index++) {
      const current = await generator.next();
      t.assert.strictEqual(current.done, false);
      t.assert.strictEqual(isPartialEvent(current.value), true);
    }
    controller.abort();

    const item = await generator.next();
    t.assert.strictEqual(item.done, false);
    if (!isItemEvent(item.value)) {
      t.assert.fail("Expected a model item event");
    }
    t.assert.deepStrictEqual(item.value, {
      event: "item",
      index: 0,
      item: createMixedSnapshotModelItem(),
    });

    const toolItem = await generator.next();
    t.assert.strictEqual(toolItem.done, false);
    if (!isItemEvent(toolItem.value)) {
      t.assert.fail("Expected a cancelled tool item event");
    }
    t.assert.deepStrictEqual(toolItem.value, {
      event: "item",
      index: 1,
      item: createMixedSnapshotCancelledToolItem(),
    });

    const terminal = await generator.next();
    t.assert.strictEqual(terminal.done, false);
    if (!isResponseEvent(terminal.value)) {
      t.assert.fail("Expected a terminal response event");
    }
    t.assert.deepStrictEqual(terminal.value, {
      event: "response",
      status: "cancelled",
      output: [
        createMixedSnapshotModelItem(),
        createMixedSnapshotCancelledToolItem(),
      ],
      content: [],
    });
    const completed = await generator.next();
    t.assert.strictEqual(completed.done, true);
    t.assert.deepStrictEqual(completed.value, {
      status: "cancelled",
      output: terminal.value.output,
      content: terminal.value.content,
    });
  });

  test("throws error when session not initialized in streaming", async (t: TestContext) => {
    const model = new MockLanguageModel();

    const session = new RunSession({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [],
      context: {},
    });

    const generator = session.runStream({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Hello" }],
        },
      ],
    });

    await t.assert.rejects(
      async () => {
        // eslint-disable-next-line @typescript-eslint/no-unused-vars
        for await (const _event of generator) {
          // consume events
        }
      },
      (err: any) => {
        t.assert.match(err.message, /RunSession not initialized/);
        return true;
      },
    );
  });

  test("merges toolkit prompts and tools in streaming runs", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueStreamResult({
      partials: [
        {
          delta: { index: 0, part: { type: "text", text: "Done" } },
        },
      ],
    });

    interface Context {
      customer: string;
    }
    const context: Context = { customer: "Ben" };
    const createdContexts: Context[] = [];
    let closeCalls = 0;

    const dynamicTool: AgentTool<Context> = {
      type: "function",
      name: "noop",
      description: "No operation",
      parameters: {
        type: "object",
        properties: {},
        additionalProperties: false,
      },
      execute() {
        return Promise.resolve({
          content: [],
          is_error: false,
        });
      },
    };

    const toolkitSession: ToolkitSession<Context> = {
      getSystemPrompt() {
        return "Streaming toolkit prompt";
      },
      getTools() {
        return [dynamicTool];
      },
      close() {
        closeCalls += 1;
        return Promise.resolve();
      },
    };

    const toolkit: Toolkit<Context> = {
      createSession(ctx) {
        createdContexts.push(ctx);
        return Promise.resolve(toolkitSession);
      },
    };

    const session = await RunSession.create({
      name: "toolkit-stream-agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [],
      toolkits: [toolkit],
      context,
    });

    const events: AgentStreamEvent[] = [];
    const generator = session.runStream({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Hello" }],
        },
      ],
    });

    let result = await generator.next();
    while (!result.done) {
      events.push(result.value);
      result = await generator.next();
    }

    t.assert.deepStrictEqual(events, [
      {
        event: "partial",
        delta: { index: 0, part: { type: "text", text: "Done" } },
      },
      {
        event: "item",
        index: 0,
        item: {
          type: "model",
          content: [{ type: "text", text: "Done" }],
        },
      },
      {
        event: "response",
        status: "completed",
        content: [{ type: "text", text: "Done" }],
        output: [
          {
            type: "model",
            content: [{ type: "text", text: "Done" }],
          },
        ],
      },
    ]);
    t.assert.deepStrictEqual(result.value, {
      status: "completed",
      content: [{ type: "text", text: "Done" }],
      output: [
        {
          type: "model",
          content: [{ type: "text", text: "Done" }],
        },
      ],
    });

    t.assert.deepStrictEqual(createdContexts, [context]);
    t.assert.deepStrictEqual(
      model.trackedStreamInputs.map((input) => input.system_prompt),
      ["Streaming toolkit prompt"],
    );
    t.assert.deepStrictEqual(
      model.trackedStreamInputs.map((input) => input.tools),
      [
        [
          {
            type: "function",
            name: "noop",
            description: "No operation",
            parameters: dynamicTool.parameters,
          },
        ],
      ],
    );
    t.assert.deepStrictEqual(model.trackedStreamInputs[0]?.tools, [
      {
        type: "function",
        name: "noop",
        description: "No operation",
        parameters: dynamicTool.parameters,
      },
    ]);

    await session.close();
    t.assert.strictEqual(closeCalls, 1);
  });
});

suite("RunSession initialization and cleanup", () => {
  test("closes initialized toolkit sessions when a later toolkit fails", async (t: TestContext) => {
    let closeCalls = 0;
    const initFailure = new Error("later toolkit failed");
    const cleanupFailure = new Error("cleanup failed");
    const model = new MockLanguageModel();

    await t.assert.rejects(
      () =>
        RunSession.create({
          name: "test_agent",
          model,
          context: {},
          toolkits: [
            {
              createSession: () =>
                Promise.resolve({
                  getSystemPrompt: () => undefined,
                  getTools: () => [],
                  close: () => {
                    closeCalls++;
                    throw cleanupFailure;
                  },
                }),
            },
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
            { createSession: () => Promise.reject(initFailure) },
          ],
        }),
      (error: unknown) => {
        t.assert.strictEqual(error instanceof AgentInitError, true);
        t.assert.strictEqual((error as AgentInitError).cause, initFailure);
        return true;
      },
    );
    t.assert.strictEqual(closeCalls, 2);
  });

  test("attempts every toolkit close and reports cleanup failure", async (t: TestContext) => {
    const cleanupFailure = new Error("cleanup failed");
    let successfulCloseCalls = 0;
    const model = new MockLanguageModel();
    const session = await RunSession.create({
      name: "test_agent",
      model,
      context: {},
      toolkits: [
        {
          createSession: () =>
            Promise.resolve({
              getSystemPrompt: () => undefined,
              getTools: () => [],
              close: () => {
                throw cleanupFailure;
              },
            }),
        },
        {
          createSession: () =>
            Promise.resolve({
              getSystemPrompt: () => undefined,
              getTools: () => [],
              close: () => {
                successfulCloseCalls++;
                return Promise.resolve();
              },
            }),
        },
      ],
    });

    await t.assert.rejects(
      () => session.close(),
      (error: unknown) => {
        t.assert.strictEqual(error instanceof AgentCleanupError, true);
        t.assert.strictEqual(
          (error as AgentCleanupError).cause,
          cleanupFailure,
        );
        return true;
      },
    );
    t.assert.strictEqual(successfulCloseCalls, 1);
  });

  test("reports instruction resolution failures as initialization errors", async (t: TestContext) => {
    const model = new MockLanguageModel();
    const cause = new Error("could not load tenant instructions");

    await t.assert.rejects(
      () =>
        RunSession.create({
          name: "test_agent",
          model,
          instructions: [() => Promise.reject(cause)],
          context: {},
        }),
      (err: unknown) => {
        t.assert.strictEqual(err instanceof AgentInitError, true);
        t.assert.strictEqual((err as AgentInitError).cause, cause);
        return true;
      },
    );
  });

  test("close() cleans up session resources", async (t: TestContext) => {
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({
      response: { content: [{ type: "text", text: "Response" }] },
    });

    const session = await RunSession.create({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [],
      context: {},
    });

    await session.run({
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Hello" }],
        },
      ],
    });

    await session.close();

    await t.assert.rejects(
      async () => {
        await session.run({
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "Hello again" }],
            },
          ],
        });
      },
      (err: any) => {
        t.assert.match(err.message, /RunSession not initialized/);
        return true;
      },
    );
  });
});
