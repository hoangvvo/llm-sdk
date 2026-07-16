import test, { type TestContext } from "node:test";

import { RunState } from "./run.ts";
import { tool, type AgentToolResult } from "./tool.ts";

void test("tool preserves its public contract and executes with context", async (t: TestContext) => {
  const context = { tenant: "north" };
  let received: unknown;
  const definition = {
    name: "echo",
    description: "Echoes input",
    parameters: { type: "object", properties: {} },
    execute: (
      args: { message: string },
      receivedContext: typeof context,
    ): AgentToolResult => {
      received = { args, context: receivedContext };
      return {
        content: [{ type: "text", text: args.message }],
        is_error: false,
      };
    },
  };

  const toolDefinition = tool(definition);

  t.assert.deepStrictEqual(toolDefinition, {
    type: "function",
    ...definition,
  });
  const result = await toolDefinition.execute(
    { message: "ok" },
    context,
    new RunState([], 1),
  );
  t.assert.deepStrictEqual(received, {
    args: { message: "ok" },
    context,
  });
  t.assert.deepStrictEqual(result, {
    content: [{ type: "text", text: "ok" }],
    is_error: false,
  });
});
