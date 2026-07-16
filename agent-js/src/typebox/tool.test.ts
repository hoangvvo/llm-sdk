import test, { type TestContext } from "node:test";
import { Type } from "typebox";
import { RunState } from "../run.ts";
import type { AgentTool } from "../tool.ts";
import { typeboxTool } from "./tool.ts";

test("typeboxTool exposes its schema and executes typed arguments", async (t: TestContext) => {
  const context = { prefix: "typed:" };
  const signal = new AbortController().signal;
  let receivedContext: typeof context | undefined;
  let receivedSignal: AbortSignal | undefined;
  const tool = typeboxTool({
    name: "echo",
    description: "Echo input",
    parameters: Type.Object(
      { message: Type.String() },
      { additionalProperties: false },
    ),
    execute: ({ message }, ctx: typeof context, state) => {
      receivedContext = ctx;
      receivedSignal = state.signal;
      return Promise.resolve({
        content: [{ type: "text", text: `${ctx.prefix}${message}` }],
        is_error: false,
      });
    },
  });

  const expected: AgentTool<typeof context> = {
    type: "function",
    name: "echo",
    description: "Echo input",
    parameters: {
      type: "object",
      properties: {
        message: { type: "string" },
      },
      required: ["message"],
      additionalProperties: false,
    },

    execute: tool.execute,
  };

  t.assert.deepStrictEqual(tool, expected);
  const result = await tool.execute(
    { message: "hello" },
    context,
    new RunState([], 1, signal),
  );
  t.assert.strictEqual(receivedContext, context);
  t.assert.strictEqual(receivedSignal, signal);
  t.assert.deepStrictEqual(result, {
    content: [{ type: "text", text: "typed:hello" }],
    is_error: false,
  });
});
