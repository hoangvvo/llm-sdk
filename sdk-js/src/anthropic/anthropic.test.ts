import { runTestGroup, SHARED_BEHAVIOR_TEST_GROUPS } from "#test-common/cases";
import { runTransportTestGroup } from "#test-common/transports";
import assert from "node:assert";
import { once } from "node:events";
import { createServer } from "node:http";
import test, { suite } from "node:test";
import Anthropic from "@anthropic-ai/sdk";
import { AnthropicModel } from "./anthropic.ts";

suite("AnthropicModel", () => {
  const apiKey = process.env["ANTHROPIC_API_KEY"];
  let model: AnthropicModel | undefined;
  function getModel() {
    assert(apiKey, "ANTHROPIC_API_KEY must be set");
    model ??= new AnthropicModel({ apiKey, modelId: "claude-sonnet-5" });
    return model;
  }

  const reasoningOptions = { profile: "anthropic_adaptive_reasoning" };

  for (const group of SHARED_BEHAVIOR_TEST_GROUPS) {
    test(group, { timeout: 120 * 1000 }, (t) => {
      return runTestGroup(t, getModel(), group);
    });
  }

  test("multimodal_tool_result", (t) =>
    runTestGroup(t, getModel(), "multimodal_tool_result"));
  test("web_search", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "web_search", {
      profile: "anthropic_web_search",
    }),
  );
  test("web_search_tool_mix", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "web_search_tool_mix"),
  );
  test("image_input", (t) => runTestGroup(t, getModel(), "image_input"));
  test("tool_search", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "tool_search"),
  );
  test("reasoning", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "reasoning", reasoningOptions),
  );
  test("reasoning_tool_use", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "reasoning_tool_use"),
  );
  test("anthropic_refusal", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "anthropic_refusal"),
  );
  test("anthropic_web_search_failure", { timeout: 120 * 1000 }, (t) =>
    runTestGroup(t, getModel(), "anthropic_web_search_failure"),
  );
  test("transport", (t) =>
    runTransportTestGroup(
      t,
      "anthropic_transport",
      (baseURL) =>
        new AnthropicModel({
          apiKey: "test-token",
          modelId: "test-model",
          baseURL,
        }),
    ));

  test("transport propagates cancellation without final usage", async (t) => {
    const server = createServer((_request, response) => {
      response.writeHead(200, { "content-type": "text/event-stream" });
      const events = [
        {
          type: "message_start",
          message: {
            id: "msg_1",
            type: "message",
            role: "assistant",
            model: "test-model",
            content: [],
            stop_reason: null,
            stop_sequence: null,
            usage: { input_tokens: 4, output_tokens: 0 },
          },
        },
        {
          type: "content_block_start",
          index: 0,
          content_block: { type: "text", text: "" },
        },
        {
          type: "content_block_delta",
          index: 0,
          delta: { type: "text_delta", text: "Partial" },
        },
      ];
      for (const event of events) {
        response.write(
          `event: ${event.type}\ndata: ${JSON.stringify(event)}\n\n`,
        );
      }
      // Keep the response open so cancellation interrupts the next read.
    });
    t.after(
      () =>
        new Promise<void>((resolve, reject) => {
          server.close((error) => {
            if (error) reject(error);
            else resolve();
          });
          server.closeAllConnections();
        }),
    );
    server.listen(0, "127.0.0.1");
    await once(server, "listening");
    const address = server.address();
    assert(address && typeof address !== "string");
    const model = new AnthropicModel({
      apiKey: "test-token",
      modelId: "test-model",
      baseURL: `http://127.0.0.1:${String(address.port)}`,
    });
    const controller = new AbortController();
    const stream = model.stream(
      {
        messages: [
          { role: "user", content: [{ type: "text", text: "Hello" }] },
        ],
      },
      { signal: controller.signal },
    );
    const first = await stream.next();
    assert(!first.done);
    assert.deepStrictEqual(first.value.delta?.part, { type: "text", text: "" });
    assert.strictEqual(first.value.usage, undefined);
    const partial = await stream.next();
    assert(!partial.done);
    assert.deepStrictEqual(partial.value.delta?.part, {
      type: "text",
      text: "Partial",
    });
    assert.strictEqual(partial.value.usage, undefined);
    const next = stream.next();
    controller.abort();
    await assert.rejects(next, Anthropic.APIUserAbortError);
    assert.strictEqual((await stream.next()).done, true);
  });
});
