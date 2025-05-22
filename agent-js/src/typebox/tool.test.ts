/* eslint-disable @typescript-eslint/no-floating-promises */
import { Type } from "@sinclair/typebox";
import test, { type TestContext } from "node:test";
import type { AgentTool } from "../tool.ts";
import { typeboxTool } from "./tool.ts";

test("typeboxTool returns AgentTool", (t: TestContext) => {
  const tool = typeboxTool({
    name: "echo",
    description: "Echo input",
    parameters: Type.Object(
      { message: Type.String() },
      { additionalProperties: false },
    ),
    execute: ({ message }) =>
      Promise.resolve({
        content: [{ type: "text", text: message }],
        is_error: false,
      }),
  });

  const expected: AgentTool<object> = {
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

    // eslint-disable-next-line @typescript-eslint/unbound-method
    execute: tool.execute,
  };

  t.assert.deepStrictEqual(tool, expected);
});
