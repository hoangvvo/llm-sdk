import { LanguageModelError } from "@hoangvvo/llm-sdk";
import { MockLanguageModel } from "@hoangvvo/llm-sdk/test";
import test, { suite, type TestContext } from "node:test";
import { Agent } from "./agent.ts";
import { AgentInitError, AgentLanguageModelError } from "./errors.ts";
import { RunSession } from "./run.ts";

suite("agent lifecycle failures", () => {
  test("closes toolkit sessions that initialized before another toolkit failed", async (t: TestContext) => {
    let closeCalls = 0;
    const initFailure = new Error("later toolkit failed");
    const cleanupFailure = new Error("cleanup failed");
    const model = new MockLanguageModel();

    await t.assert.rejects(
      () =>
        RunSession.create({
          name: "test_agent",
          model,
          context: {},
          toolkits: [
            {
              createSession: () =>
                Promise.resolve({
                  getSystemPrompt: () => undefined,
                  getTools: () => [],
                  close: () => {
                    closeCalls++;
                    throw cleanupFailure;
                  },
                }),
            },
            {
              createSession: () =>
                Promise.resolve({
                  getSystemPrompt: () => undefined,
                  getTools: () => [],
                  close: () => {
                    closeCalls++;
                    return Promise.resolve();
                  },
                }),
            },
            { createSession: () => Promise.reject(initFailure) },
          ],
        }),
      (error: unknown) => {
        t.assert.strictEqual(error instanceof AgentInitError, true);
        t.assert.strictEqual((error as AgentInitError).cause, initFailure);
        return true;
      },
    );
    t.assert.strictEqual(closeCalls, 2);
  });

  test("attempts every toolkit close and reports cleanup failure", async (t: TestContext) => {
    const cleanupFailure = new Error("cleanup failed");
    let successfulCloseCalls = 0;
    const model = new MockLanguageModel();
    const session = await RunSession.create({
      name: "test_agent",
      model,
      context: {},
      toolkits: [
        {
          createSession: () =>
            Promise.resolve({
              getSystemPrompt: () => undefined,
              getTools: () => [],
              close: () => {
                throw cleanupFailure;
              },
            }),
        },
        {
          createSession: () =>
            Promise.resolve({
              getSystemPrompt: () => undefined,
              getTools: () => [],
              close: () => {
                successfulCloseCalls++;
                return Promise.resolve();
              },
            }),
        },
      ],
    });

    await t.assert.rejects(() => session.close(), cleanupFailure);
    t.assert.strictEqual(successfulCloseCalls, 1);
  });

  test("preserves the model error when cleanup also fails", async (t: TestContext) => {
    const modelFailure = new LanguageModelError("generation failed");
    const cleanupFailure = new Error("cleanup failed");
    const model = new MockLanguageModel();
    model.enqueueGenerateResult({ error: modelFailure });
    const agent = new Agent({
      name: "test_agent",
      model,
      toolkits: [
        {
          createSession: () =>
            Promise.resolve({
              getSystemPrompt: () => undefined,
              getTools: () => [],
              close: () => Promise.reject(cleanupFailure),
            }),
        },
      ],
    });

    await t.assert.rejects(
      () =>
        agent.run({
          context: {},
          input: [
            {
              type: "message",
              role: "user",
              content: [{ type: "text", text: "Hello" }],
            },
          ],
        }),
      (error: unknown) => {
        t.assert.strictEqual(error instanceof AgentLanguageModelError, true);
        t.assert.strictEqual(
          (error as AgentLanguageModelError).cause,
          modelFailure,
        );
        return true;
      },
    );
  });
});
