import test, { type TestContext } from "node:test";

import { tool, type AgentToolResult } from "./tool.ts";

void test("tool returns the same definition object", (t: TestContext) => {
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

  t.assert.strictEqual(toolDefinition, definition);
});
