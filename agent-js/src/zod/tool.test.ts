import test, { type TestContext } from "node:test";
import z from "zod";
import { RunState } from "../run.ts";
import { zodTool } from "./tool.ts";

test("zodTool exposes its strict schema and executes typed arguments", async (t: TestContext) => {
  const context = { suffix: "!" };
  const signal = new AbortController().signal;
  let receivedContext: typeof context | undefined;
  let receivedSignal: AbortSignal | undefined;
  const tool = zodTool({
    name: "echo",
    description: "Echo input",
    parameters: z.object({
      message: z.string(),
      option: z.number().optional(),
    }),
    execute: ({ message, option }, ctx: typeof context, state) => {
      receivedContext = ctx;
      receivedSignal = state.signal;
      return Promise.resolve({
        content: [
          {
            type: "text",
            text: `${message}:${String(option)}${ctx.suffix}`,
          },
        ],
        is_error: false,
      });
    },
  });

  const expected = {
    type: "function",
    name: "echo",
    description: "Echo input",
    parameters: {
      $schema: "https://json-schema.org/draft/2019-09/schema#",
      type: "object",
      properties: {
        message: { type: "string" },
        option: { type: ["number", "null"] },
      },
      // zod-to-json-schema adds all optional properties to required array
      // for OpenAI strict mode
      required: ["message", "option"],
      additionalProperties: false,
    },

    execute: tool.execute,
  };

  t.assert.deepStrictEqual(tool, expected);
  const result = await tool.execute(
    { message: "hello", option: 2 },
    context,
    new RunState([], 1, signal),
  );
  t.assert.strictEqual(receivedContext, context);
  t.assert.strictEqual(receivedSignal, signal);
  t.assert.deepStrictEqual(result, {
    content: [{ type: "text", text: "hello:2!" }],
    is_error: false,
  });
});
