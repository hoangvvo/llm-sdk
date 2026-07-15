import { MockLanguageModel } from "@hoangvvo/llm-sdk/test";
import test, { suite, type TestContext } from "node:test";
import { AgentInitError } from "../errors.ts";
import { RunSession } from "../run.ts";
import { convertMCPContentToParts } from "./content.ts";
import { mcpToolkit } from "./toolkit.ts";

suite("MCP initialization", () => {
  test("resolves asynchronous transport parameters from session context", async (t: TestContext) => {
    const contexts: { endpoint: string }[] = [];
    const model = new MockLanguageModel();

    await t.assert.rejects(
      () =>
        RunSession.create({
          name: "test_agent",
          model,
          context: { endpoint: "not a valid URL" },
          toolkits: [
            mcpToolkit(async (context: { endpoint: string }) => {
              contexts.push(context);
              return Promise.resolve({
                type: "streamable-http",
                url: context.endpoint,
              });
            }),
          ],
        }),
      (error: unknown) => error instanceof AgentInitError,
    );
    t.assert.deepStrictEqual(contexts, [{ endpoint: "not a valid URL" }]);
  });

  test("wraps resolver failures as initialization errors", async (t: TestContext) => {
    const cause = new Error("credential lookup failed");
    const model = new MockLanguageModel();

    await t.assert.rejects(
      () =>
        RunSession.create({
          name: "test_agent",
          model,
          context: {},
          toolkits: [
            mcpToolkit(() => {
              throw cause;
            }),
          ],
        }),
      (error: unknown) => {
        t.assert.strictEqual(error instanceof AgentInitError, true);
        t.assert.strictEqual((error as AgentInitError).cause, cause);
        return true;
      },
    );
  });

  test("rejects MCP audio formats the agent cannot represent", (t: TestContext) => {
    t.assert.throws(
      () =>
        convertMCPContentToParts([
          { type: "audio", mimeType: "audio/unknown", data: "AAEC" },
        ]),
      /Unsupported audio format for mime type/,
    );
  });
});
