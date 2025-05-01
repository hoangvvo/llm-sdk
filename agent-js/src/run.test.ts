/* eslint-disable @typescript-eslint/no-floating-promises */
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/no-unsafe-member-access */
/* eslint-disable @typescript-eslint/no-unsafe-argument */
/* eslint-disable @typescript-eslint/no-unsafe-return */

import {
  LanguageModelError,
  MockLanguageModel,
  type PartDelta,
  type PartialModelResponse,
} from "@hoangvvo/llm-sdk";
import test, { suite, type TestContext } from "node:test";
import {
  AgentInvariantError,
  AgentLanguageModelError,
  AgentMaxTurnsExceededError,
  AgentToolExecutionError,
} from "./errors.ts";
import { RunSession, type RunState } from "./run.ts";
import type { AgentTool } from "./tool.ts";
import type { AgentResponse, AgentStreamEvent } from "./types.ts";

function createMockTool<TContext = any>(
  name: string,
  result: any,
  executeFn?: (args: any, ctx: TContext, state: RunState) => any,
): AgentTool<TContext> {
  return {
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
    });

    const response = await session.run({
      context: {},
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Hello!" }],
        },
      ],
    });

    t.assert.deepStrictEqual(response, {
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

    const tool = createMockTool("test_tool", null, toolExecute);

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
    });

    const response = await session.run({
      context: { testContext: true },
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Use the tool" }],
        },
      ],
    });

    const toolCallArguments = toolExecute.mock.calls.map((call) =>
      call.arguments.slice(0, 2),
    );
    t.assert.deepStrictEqual(toolCallArguments, [
      [{ param: "value" }, { testContext: true }],
    ]);

    t.assert.deepStrictEqual(response, {
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
          is_error: false,
        },
        {
          type: "model",
          content: [{ type: "text", text: "Final response" }],
        },
      ],
    });
  });

  test("executes multiple tool calls in parallel", async (t: TestContext) => {
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

    const tool1 = createMockTool("tool_1", null, tool1Execute);
    const tool2 = createMockTool("tool_2", null, tool2Execute);

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
    });

    const response = await session.run({
      context: {},
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
          is_error: false,
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
          is_error: false,
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

    const tool = createMockTool("calculator", null, toolExecute);

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
    });

    const response = await session.run({
      context: {},
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
          is_error: false,
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
          is_error: false,
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

    const tool = createMockTool("test_tool", null, toolExecute);

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
    });

    await t.assert.rejects(
      async () => {
        await session.run({
          context: {},
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
    });

    await t.assert.rejects(
      async () => {
        await session.run({
          context: {},
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

    const tool = createMockTool("failing_tool", null, toolExecute);

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
    });

    await t.assert.rejects(
      async () => {
        await session.run({
          context: {},
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

  test("handles tool returning error result", async (t: TestContext) => {
    const toolExecute = t.mock.fn<AgentTool<any>["execute"]>(() => ({
      content: [{ type: "text", text: "Error: Invalid parameters" }],
      is_error: true,
    }));

    const tool = createMockTool("test_tool", null, toolExecute);

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
    });

    const response = await session.run({
      context: {},
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Use the tool" }],
        },
      ],
    });

    t.assert.deepStrictEqual(
      toolExecute.mock.calls.map((call) => call.arguments[0]),
      [{ invalid: true }],
    );

    const expectedResponse: AgentResponse = {
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
          is_error: true,
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
    });

    await t.assert.rejects(
      async () => {
        await session.run({
          context: {},
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
    });

    await session.run({
      context: {},
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
    });

    await t.assert.rejects(
      async () => {
        await session.run({
          context: {},
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
    });

    await session.run({
      context: { userRole: "developer" },
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
});

suite("RunSession#runStream", () => {
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
    });

    const events: AgentStreamEvent[] = [];
    const generator = session.runStream({
      context: {},
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
        type: "model",
        content: [{ type: "text", text: "Hello!" }],
      },
    ]);
    t.assert.deepStrictEqual(events.find(isResponseEvent), {
      event: "response",
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

    const tool = createMockTool("test_tool", null, toolExecute);

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
    });

    const events = [];
    const generator = session.runStream({
      context: {},
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
        event: "item",
        type: "tool",
        tool_name: "test_tool",
        tool_call_id: "call_1",
        input: { a: 1, b: 2, operation: "add" },
        output: [{ type: "text", text: "Tool result" }],
        is_error: false,
      },
      {
        event: "item",
        type: "model",
        content: [{ type: "text", text: "FinalFinal response" }],
      },
    ];
    t.assert.deepStrictEqual(events.filter(isItemEvent), expectedItemEvents);

    const responseEvent = events.find(isResponseEvent);
    t.assert.deepStrictEqual(responseEvent, {
      event: "response",
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
          is_error: false,
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
    const toolExecute = t.mock.fn<AgentTool<any>["execute"]>(() => ({
      content: [{ type: "text", text: "Calculation done" }],
      is_error: false,
    }));

    const tool = createMockTool("calculator", null, toolExecute);

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
    });

    const events = [];
    const generator = session.runStream({
      context: {},
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
        event: "item",
        type: "tool",
        tool_name: "calculator",
        tool_call_id: "call_1",
        input: { a: 1, b: 2 },
        output: [{ type: "text", text: "Calculation done" }],
        is_error: false,
      },
      {
        event: "item",
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
        event: "item",
        type: "tool",
        tool_name: "calculator",
        tool_call_id: "call_2",
        input: { a: 3, b: 4 },
        output: [{ type: "text", text: "Calculation done" }],
        is_error: false,
      },
      {
        event: "item",
        type: "model",
        content: [{ type: "text", text: "All done" }],
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
          is_error: false,
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
          is_error: false,
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

    const tool = createMockTool("test_tool", null, toolExecute);

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
    });

    const generator = session.runStream({
      context: {},
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
    });

    const generator = session.runStream({
      context: {},
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

  test("throws error when session not initialized in streaming", async (t: TestContext) => {
    const model = new MockLanguageModel();

    const session = new RunSession({
      name: "test_agent",
      model,
      instructions: [],
      max_turns: 10,
      response_format: { type: "text" },
      tools: [],
    });

    const generator = session.runStream({
      context: {},
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
});

suite("RunSession lifecycle", () => {
  test("finish() cleans up session resources", async (t: TestContext) => {
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
    });

    await session.run({
      context: {},
      input: [
        {
          type: "message",
          role: "user",
          content: [{ type: "text", text: "Hello" }],
        },
      ],
    });

    await session.finish();

    await t.assert.rejects(
      async () => {
        await session.run({
          context: {},
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
