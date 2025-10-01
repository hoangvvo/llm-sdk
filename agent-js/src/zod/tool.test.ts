import test, { type TestContext } from "node:test";
import z from "zod";
import { zodTool } from "./tool.ts";

test("zodTool returns AgentTool", (t: TestContext) => {
  const tool = zodTool({
    name: "echo",
    description: "Echo input",
    parameters: z.object({
      message: z.string(),
      option: z.number().optional(),
    }),
    execute: ({ message, option }) =>
      Promise.resolve({
        content: [{ type: "text", text: `${message}${String(option)}` }],
        is_error: false,
      }),
  });

  const expected = {
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
    // eslint-disable-next-line @typescript-eslint/unbound-method
    execute: tool.execute,
  };

  t.assert.deepStrictEqual(tool, expected);
});
