import assert from "node:assert/strict";
import test, { type TestContext } from "node:test";
import { assertReasoningPart } from "./assert.ts";

const context = { assert } as unknown as TestContext;

test("reasoning assertions require matching text even with a signature", () => {
  assert.throws(() =>
    assertReasoningPart(
      context,
      [{ type: "reasoning", text: "wrong", signature: "opaque" }],
      { type: "reasoning", text: /John/, signature: true },
    ),
  );
});

test("reasoning assertions can require signature presence", () => {
  assert.throws(() =>
    assertReasoningPart(context, [{ type: "reasoning", text: "John" }], {
      type: "reasoning",
      text: /John/,
      signature: true,
    }),
  );

  assert.doesNotThrow(() =>
    assertReasoningPart(
      context,
      [{ type: "reasoning", text: "John", signature: "opaque" }],
      { type: "reasoning", text: /John/, signature: true },
    ),
  );
});
