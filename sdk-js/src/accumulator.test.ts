import test, { suite, type TestContext } from "node:test";
import { StreamAccumulator } from "./accumulator.ts";
import { InvariantError } from "./errors.ts";

suite("StreamAccumulator", () => {
  test("reconstructs interleaved multipart streams and metadata", (t: TestContext) => {
    const accumulator = new StreamAccumulator();

    accumulator.addPartial({
      delta: { index: 2, part: { type: "reasoning", text: "think " } },
      usage: { input_tokens: 2, output_tokens: 1 },
      cost: 0.1,
    });
    accumulator.addPartial({
      delta: { index: 0, part: { type: "text", text: "Hel" } },
    });
    accumulator.addPartial({
      delta: {
        index: 1,
        part: {
          type: "tool-call",
          tool_call_id: "call_1",
          tool_name: "weather",
          args: '{"city":',
        },
      },
    });
    accumulator.addPartial({
      delta: { index: 0, part: { type: "text", text: "lo" } },
      usage: { input_tokens: 3, output_tokens: 4 },
      cost: 0.2,
    });
    accumulator.addPartial({
      delta: {
        index: 1,
        part: { type: "tool-call", args: '"Paris"}' },
      },
    });
    accumulator.addPartial({
      delta: {
        index: 2,
        part: { type: "reasoning", text: "done", signature: "sig" },
      },
    });

    t.assert.strictEqual(accumulator.size, 3);
    t.assert.strictEqual(accumulator.isEmpty, false);
    t.assert.deepStrictEqual(accumulator.computeResponse(), {
      content: [
        { type: "text", text: "Hello" },
        {
          type: "tool-call",
          tool_call_id: "call_1",
          tool_name: "weather",
          args: { city: "Paris" },
        },
        { type: "reasoning", text: "think done", signature: "sig" },
      ],
      usage: { input_tokens: 5, output_tokens: 5 },
      cost: 0.30000000000000004,
    });
  });

  test("rejects a different part type at an occupied index", (t: TestContext) => {
    const accumulator = new StreamAccumulator();
    accumulator.addPartial({
      delta: { index: 0, part: { type: "text", text: "hello" } },
    });

    t.assert.throws(() => {
      accumulator.addPartial({
        delta: {
          index: 0,
          part: { type: "reasoning", text: "wrong type" },
        },
      });
    }, /Type mismatch at index 0/);
  });

  test("rejects malformed and incomplete tool calls", (t: TestContext) => {
    const malformed = new StreamAccumulator();
    malformed.addPartial({
      delta: {
        index: 0,
        part: {
          type: "tool-call",
          tool_call_id: "call_1",
          tool_name: "weather",
          args: "{bad json",
        },
      },
    });
    t.assert.throws(
      () => malformed.computeResponse(),
      (error: unknown) => error instanceof InvariantError,
    );

    const incomplete = new StreamAccumulator();
    incomplete.addPartial({
      delta: {
        index: 0,
        part: { type: "tool-call", args: "{}" },
      },
    });
    t.assert.throws(
      () => incomplete.computeResponse(),
      /Missing required fields/,
    );
  });

  test("snapshots independently materializable parts", (t: TestContext) => {
    const accumulator = new StreamAccumulator();
    accumulator.addPartial({
      delta: { index: 0, part: { type: "text", text: "partial" } },
      usage: { input_tokens: 2, output_tokens: 3 },
      cost: 0.25,
    });
    accumulator.addPartial({
      delta: {
        index: 1,
        part: {
          type: "tool-call",
          tool_call_id: "call_1",
          tool_name: "weather",
          args: '{"city":"Paris"}',
        },
      },
    });
    accumulator.addPartial({
      delta: {
        index: 2,
        part: { type: "tool-call", args: "{incomplete" },
      },
    });
    accumulator.addPartial({
      delta: {
        index: 3,
        part: {
          type: "image",
          data: "aGVsbG8=",
          mime_type: "image/png",
        },
      },
    });
    accumulator.addPartial({
      delta: {
        index: 4,
        part: { type: "audio", data: "AAABAA==", format: "linear16" },
      },
    });

    t.assert.deepStrictEqual(accumulator.snapshot(), {
      content: [
        { type: "text", text: "partial" },
        {
          type: "tool-call",
          tool_call_id: "call_1",
          tool_name: "weather",
          args: { city: "Paris" },
        },
        { type: "image", data: "aGVsbG8=", mime_type: "image/png" },
        { type: "audio", data: "AAABAA==", format: "linear16" },
      ],
      usage: { input_tokens: 2, output_tokens: 3 },
      cost: 0.25,
    });
    t.assert.throws(() => accumulator.computeResponse());
  });
});
