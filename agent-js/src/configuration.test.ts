import { MockLanguageModel } from "@hoangvvo/llm-sdk/test";
import test, { type TestContext } from "node:test";
import { Agent } from "./agent.ts";
import { tool } from "./tool.ts";

test("Agent forwards its complete public configuration to the model", async (t: TestContext) => {
  const model = new MockLanguageModel();
  model.enqueueGenerateResult({
    response: { content: [{ type: "text", text: "configured" }] },
  });
  const functionTool = tool<{ tenant: string }, { query: string }>({
    name: "lookup",
    description: "Look up a record",
    parameters: {
      type: "object",
      properties: { query: { type: "string" } },
      required: ["query"],
      additionalProperties: false,
    },
    execute: () => ({ content: [], is_error: false }),
  });
  const responseFormat = {
    type: "json" as const,
    name: "answer",
    description: "A configured answer",
    schema: {
      type: "object",
      properties: { answer: { type: "string" } },
      required: ["answer"],
      additionalProperties: false,
    },
  };
  const audio = { format: "mp3" as const, voice: "alloy", language: "en" };
  const reasoning = { enabled: true, budget_tokens: 256 };
  const agent = new Agent({
    name: "configured-agent",
    model,
    instructions: ["Static", ({ tenant }) => `Tenant: ${tenant}`],
    tools: [
      functionTool,
      { type: "web_search", allowed_domains: ["example.com"] },
    ],
    response_format: responseFormat,
    max_turns: 3,
    temperature: 0.2,
    top_p: 0.8,
    top_k: 12,
    presence_penalty: 0.1,
    frequency_penalty: 0.3,
    modalities: ["text", "audio"],
    audio,
    reasoning,
  });

  await agent.run({
    context: { tenant: "acme" },
    input: [
      {
        type: "message",
        role: "user",
        content: [{ type: "text", text: "Configure this" }],
      },
    ],
  });

  const [input] = model.trackedGenerateInputs;
  t.assert.ok(input);
  t.assert.deepStrictEqual(input, {
    messages: [
      {
        role: "user",
        content: [{ type: "text", text: "Configure this" }],
      },
    ],
    system_prompt: "Static\nTenant: acme",
    tools: [
      {
        type: "function",
        name: "lookup",
        description: "Look up a record",
        parameters: functionTool.parameters,
      },
      { type: "web_search", allowed_domains: ["example.com"] },
    ],
    response_format: responseFormat,
    temperature: 0.2,
    top_p: 0.8,
    top_k: 12,
    presence_penalty: 0.1,
    frequency_penalty: 0.3,
    modalities: ["text", "audio"],
    audio,
    reasoning,
  });
});
