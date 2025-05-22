import test, { type TestContext } from "node:test";

import type { AgentResponse } from "./types.ts";
import { getResponseText } from "./utils.ts";

void test("getResponseText concatenates text parts", (t: TestContext) => {
  const response: AgentResponse = {
    output: [],
    content: [
      { type: "text", text: "Hello" },
      { type: "image", image_data: "...", mime_type: "image/png" },
      { type: "text", text: "world" },
    ],
  };

  t.assert.strictEqual(getResponseText(response), "Hello world");
});

void test("getResponseText omits non-text and empty strings", (t: TestContext) => {
  const response: AgentResponse = {
    output: [],
    content: [
      { type: "tool-call", args: {}, tool_call_id: "1", tool_name: "tool" },
      { type: "text", text: "" },
    ],
  };

  t.assert.strictEqual(getResponseText(response), "");
});
