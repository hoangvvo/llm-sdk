import test, { type TestContext } from "node:test";

import { tool, type AgentToolResult } from "./tool.ts";

void test("tool returns an agent function tool", (t: TestContext) => {
  const definition = {
    name: "echo",
    description: "Echoes input",
    parameters: { type: "object", properties: {} },
    execute: (): AgentToolResult => ({
      content: [{ type: "text", text: "ok" }],
      is_error: false,
    }),
  };

  const toolDefinition = tool(definition);

  t.assert.deepStrictEqual(toolDefinition, {
    type: "function",
    ...definition,
  });
});
