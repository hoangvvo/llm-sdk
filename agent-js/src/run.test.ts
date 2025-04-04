/* eslint-disable @typescript-eslint/no-floating-promises */
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/no-unsafe-member-access */
/* eslint-disable @typescript-eslint/no-unsafe-argument */
/* eslint-disable @typescript-eslint/no-unsafe-return */

import {
  LanguageModelError,
  type LanguageModelInput,
  type ModelResponse,
  type PartDelta,
  type PartialModelResponse,
} from "@hoangvvo/llm-sdk";
import assert from "node:assert";
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

function createMockLanguageModel(t: TestContext) {
  const responses: ModelResponse[] = [];
  const partialModelResponseSets: PartialModelResponse[][] = [];
  const errors: Error[] = [];
  const streamErrors: Error[] = [];

  return {
    modelId: "mock-model",
    provider: "mock",
    addResponses(...value: ModelResponse[]) {
      responses.push(...value);
      return this;
    },
    addPartialModelResponses(...value: PartialModelResponse[][]) {
      partialModelResponseSets.push(...value);
      return this;
    },
    addError(error: Error) {
      errors.push(error);
      return this;
    },
    addStreamError(error: Error) {
      streamErrors.push(error);
      return this;
    },
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    generate: t.mock.fn((_input: LanguageModelInput) => {
      const error = errors.shift();
      if (error) {
        return Promise.reject(error);
      }
      const response = responses.shift();
      assert(response, "no mock response");
      return Promise.resolve(response);
    }),
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    stream: t.mock.fn(async function* (_input: LanguageModelInput) {
      const error = streamErrors.shift();
      if (error) {
        throw error;
      }
      const partialResponses = partialModelResponseSets.shift();
      assert(partialResponses, "no mock partial response");
      for (const response of partialResponses) {
        yield Promise.resolve(response);
      }
    }),
  };
}

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

suite("RunSession#run", () => {
  test("returns a response when there is no tool call", async (t: TestContext) => {
    const model = createMockLanguageModel(t).addResponses({
      content: [{ type: "text", text: "Hi!" }],
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

    const model = createMockLanguageModel(t).addResponses(
      {
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
        content: [{ type: "text", text: "Final response" }],
      },
    );

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

    t.assert.strictEqual(toolExecute.mock.calls.length, 1);
    const firstCall = toolExecute.mock.calls[0];
    t.assert.ok(firstCall);
    t.assert.deepStrictEqual(firstCall.arguments[0], {
      param: "value",
    });
    t.assert.deepStrictEqual(firstCall.arguments[1], {
      testContext: true,
    });

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

    const model = createMockLanguageModel(t).addResponses(
      {
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
        content: [{ type: "text", text: "Processed both tools" }],
        usage: {
          input_tokens: 50,
          output_tokens: 10,
        },
        cost: 0.0003,
      },
    );

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

    t.assert.deepStrictEqual(tool1Execute.mock.calls[0]?.arguments[0], {
      param: "value1",
    });
    t.assert.deepStrictEqual(tool2Execute.mock.calls[0]?.arguments[0], {
      param: "value2",
    });

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

    t.assert.deepStrictEqual(response.content, [
      { type: "text", text: "Processed both tools" },
    ]);
    t.assert.strictEqual(response.output.length, 4);
  });

  test("handles multiple turns with tool calls", async (t: TestContext) => {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const toolExecute = t.mock.fn((_args) => ({
      content: [{ type: "text", text: "Calculation result" }],
      is_error: false,
    }));

    const tool = createMockTool("calculator", null, toolExecute);

    const model = createMockLanguageModel(t).addResponses(
      {
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
        content: [{ type: "text", text: "All calculations done" }],
      },
    );

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

    t.assert.deepStrictEqual(toolExecute.mock.calls[0]?.arguments[0], {
      operation: "add",
      a: 1,
      b: 2,
    });
    t.assert.deepStrictEqual(response.content, [
      { type: "text", text: "All calculations done" },
    ]);
    t.assert.strictEqual(response.output.length, 5);
  });

  test("throws AgentMaxTurnsExceededError when max turns exceeded", async (t: TestContext) => {
    const toolExecute = t.mock.fn(() => ({
      content: [{ type: "text", text: "Tool result" }],
      is_error: false,
    }));

    const tool = createMockTool("test_tool", null, toolExecute);

    const model = createMockLanguageModel(t).addResponses(
      {
        content: [
          {
            type: "tool-call",
            tool_name: "test_tool",
            tool_call_id: "call_1",
            args: {},
          },
        ],
      },
      {
        content: [
          {
            type: "tool-call",
            tool_name: "test_tool",
            tool_call_id: "call_2",
            args: {},
          },
        ],
      },
      {
        content: [
          {
            type: "tool-call",
            tool_name: "test_tool",
            tool_call_id: "call_3",
            args: {},
          },
        ],
      },
    );

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
    const model = createMockLanguageModel(t).addResponses({
      content: [
        {
          type: "tool-call",
          tool_name: "non_existent_tool",
          tool_call_id: "call_1",
          args: {},
        },
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

    const model = createMockLanguageModel(t).addResponses({
      content: [
        {
          type: "tool-call",
          tool_name: "failing_tool",
          tool_call_id: "call_1",
          args: {},
        },
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
    const toolExecute = t.mock.fn(() => ({
      content: [{ type: "text", text: "Error: Invalid parameters" }],
      is_error: true,
    }));

    const tool = createMockTool("test_tool", null, toolExecute);

    const model = createMockLanguageModel(t).addResponses(
      {
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
        content: [{ type: "text", text: "Handled the error" }],
      },
    );

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

    const toolItem = response.output[1];
    t.assert.ok(toolItem);
    t.assert.strictEqual(toolItem.type, "tool");
    t.assert.strictEqual(toolItem.is_error, true);
    t.assert.deepStrictEqual(toolItem.output, [
      { type: "text", text: "Error: Invalid parameters" },
    ]);
    t.assert.deepStrictEqual(response.content, [
      { type: "text", text: "Handled the error" },
    ]);
  });

  test("throws error when session not initialized", async (t: TestContext) => {
    const model = createMockLanguageModel(t);

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
    const model = createMockLanguageModel(t).addResponses({
      content: [{ type: "text", text: "Response" }],
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

    const generateCalls = model.generate.mock.calls;
    t.assert.strictEqual(generateCalls.length, 1);
    const generateCall = generateCalls[0];
    t.assert.ok(generateCall);
    t.assert.strictEqual(generateCall.arguments[0].temperature, 0.7);
    t.assert.strictEqual(generateCall.arguments[0].top_p, 0.9);
    t.assert.strictEqual(generateCall.arguments[0].top_k, 40);
    t.assert.strictEqual(generateCall.arguments[0].presence_penalty, 0.1);
    t.assert.strictEqual(generateCall.arguments[0].frequency_penalty, 0.2);
  });

  test("throws LanguageModelError when non-streaming generation fails", async (t: TestContext) => {
    const model = createMockLanguageModel(t).addError(
      new LanguageModelError("API quota exceeded"),
    );

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
    const model = createMockLanguageModel(t).addResponses({
      content: [{ type: "text", text: "Response" }],
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

    const generateCalls = model.generate.mock.calls;
    t.assert.strictEqual(generateCalls.length, 1);
    const generateCall = generateCalls[0];
    t.assert.ok(generateCall);
    t.assert.strictEqual(
      generateCall.arguments[0].system_prompt,
      "You are a helpful assistant.\nThe user is a developer.\nAlways be polite.",
    );
  });
});

suite("RunSession#runStream", () => {
  test("streams response when there is no tool call", async (t: TestContext) => {
    const model = createMockLanguageModel(t).addPartialModelResponses([
      createPartialResponse({ type: "text", text: "Hel" }),
      createPartialResponse({ type: "text", text: "lo" }),
      createPartialResponse({ type: "text", text: "!" }),
    ]);

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

    t.assert.strictEqual(events.length, 5);
    const partialEvents = events.filter((e) => e.event === "partial");
    t.assert.strictEqual(partialEvents.length, 3);

    t.assert.deepStrictEqual(
      partialEvents.map((e) => e.delta),
      [
        {
          index: 0,
          part: { type: "text", text: "Hel" },
        },
        {
          index: 0,
          part: { type: "text", text: "lo" },
        },
        {
          index: 0,
          part: { type: "text", text: "!" },
        },
      ],
    );

    const itemEvent = events[3];
    t.assert.ok(itemEvent);
    t.assert.strictEqual(itemEvent.event, "item");
    t.assert.strictEqual(itemEvent.type, "model");

    const responseEvent = events[4];
    t.assert.ok(responseEvent);
    t.assert.strictEqual(responseEvent.event, "response");
    t.assert.deepStrictEqual(responseEvent.content, [
      { type: "text", text: "Hello!" },
    ]);
  });

  test("streams tool call execution and response", async (t: TestContext) => {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const toolExecute = t.mock.fn((_args) => ({
      content: [{ type: "text", text: "Tool result" }],
      is_error: false,
    }));

    const tool = createMockTool("test_tool", null, toolExecute);

    const model = createMockLanguageModel(t)
      .addPartialModelResponses([
        createPartialResponse({
          type: "tool-call",
          tool_name: "test_tool",
          tool_call_id: "call_1",
          args: JSON.stringify({ a: 1, b: 2, operation: "add" }),
        }),
      ])
      .addPartialModelResponses([
        createPartialResponse({ type: "text", text: "Final" }),
        createPartialResponse({ type: "text", text: "Final response" }),
      ]);

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

    const partialEvents = events.filter((e) => e.event === "partial");
    const itemEvents = events.filter((e) => e.event === "item");
    const responseEvents = events.filter((e) => e.event === "response");

    t.assert.strictEqual(partialEvents.length, 3);
    t.assert.strictEqual(itemEvents.length, 3);
    t.assert.strictEqual(responseEvents.length, 1);

    t.assert.deepStrictEqual(
      itemEvents.map((e) => e.type),
      ["model", "tool", "model"],
    );

    t.assert.deepStrictEqual(toolExecute.mock.calls[0]?.arguments[0], {
      operation: "add",
      a: 1,
      b: 2,
    });
  });

  test("handles multiple turns in streaming mode", async (t: TestContext) => {
    const toolExecute = t.mock.fn(() => ({
      content: [{ type: "text", text: "Calculation done" }],
      is_error: false,
    }));

    const tool = createMockTool("calculator", null, toolExecute);

    const model = createMockLanguageModel(t)
      .addPartialModelResponses([
        createPartialResponse({
          type: "tool-call",
          tool_name: "calculator",
          tool_call_id: "call_1",
          args: JSON.stringify({ a: 1, b: 2 }),
        }),
      ])
      .addPartialModelResponses([
        createPartialResponse({
          type: "tool-call",
          tool_name: "calculator",
          tool_call_id: "call_2",
          args: JSON.stringify({ a: 3, b: 4 }),
        }),
      ])
      .addPartialModelResponses([
        createPartialResponse({ type: "text", text: "All done" }),
      ]);

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

    const itemEvents = events.filter((e) => e.event === "item");
    t.assert.strictEqual(itemEvents.length, 5);
    t.assert.strictEqual(toolExecute.mock.calls.length, 2);

    const responseEvent = events.find((e) => e.event === "response");
    t.assert.ok(responseEvent);
    t.assert.deepStrictEqual(responseEvent.content, [
      { type: "text", text: "All done" },
    ]);
  });

  test("throws AgentMaxTurnsExceededError in streaming mode", async (t: TestContext) => {
    const toolExecute = t.mock.fn(() => ({
      content: [{ type: "text", text: "Tool result" }],
      is_error: false,
    }));

    const tool = createMockTool("test_tool", null, toolExecute);

    const model = createMockLanguageModel(t)
      .addPartialModelResponses([
        createPartialResponse({
          type: "tool-call",
          tool_name: "test_tool",
          tool_call_id: "call_1",
          args: "{}",
        }),
      ])
      .addPartialModelResponses([
        createPartialResponse({
          type: "tool-call",
          tool_name: "test_tool",
          tool_call_id: "call_2",
          args: "{}",
        }),
      ])
      .addPartialModelResponses([
        createPartialResponse({
          type: "tool-call",
          tool_name: "test_tool",
          tool_call_id: "call_3",
          args: "{}",
        }),
      ]);

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
    const model = createMockLanguageModel(t).addStreamError(
      new LanguageModelError("Rate limit exceeded"),
    );

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
    const model = createMockLanguageModel(t);

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
    const model = createMockLanguageModel(t).addResponses({
      content: [{ type: "text", text: "Response" }],
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
