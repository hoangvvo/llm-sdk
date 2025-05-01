/* eslint-disable @typescript-eslint/no-floating-promises */
import test, { suite, type TestContext } from "node:test";
import { MockLanguageModel } from "./testing.ts";
import type { ModelResponse } from "./types.ts";
// import { MockLanguageModel, type ModelResponse } from "@hoangvvo/llm-sdk";

suite("MockLanguageModel", () => {
  test("tracks generate inputs and returns mocked responses", async (t: TestContext) => {
    const model = new MockLanguageModel();

    model.enqueueGenerateResult(
      // First mocked response
      {
        response: {
          content: [{ type: "text", text: "Hello, world!" }],
        },
      },
      // Second mocked response is an error
      {
        error: new Error("Generate error"),
      },
      // Third mocked response
      {
        response: {
          content: [{ type: "text", text: "Goodbye, world!" }],
        },
      },
    );

    // First call should return the first mocked response
    const res1 = await model.generate({
      messages: [{ role: "user", content: [{ type: "text", text: "Hi" }] }],
    });
    const expected1: ModelResponse = {
      content: [{ type: "text", text: "Hello, world!" }],
    };
    t.assert.deepStrictEqual(res1, expected1);
    t.assert.deepStrictEqual(model.trackedGenerateInputs[0], {
      messages: [{ role: "user", content: [{ type: "text", text: "Hi" }] }],
    });

    // Second call should throw an error
    await t.assert.rejects(
      model.generate({
        messages: [
          { role: "user", content: [{ type: "text", text: "Error" }] },
        ],
      }),
      { message: "Generate error" },
    );
    t.assert.deepStrictEqual(model.trackedGenerateInputs[1], {
      messages: [{ role: "user", content: [{ type: "text", text: "Error" }] }],
    });

    // Third call should return the last mocked response
    const res3 = await model.generate({
      messages: [
        { role: "user", content: [{ type: "text", text: "Goodbye" }] },
      ],
    });
    const expected3: ModelResponse = {
      content: [{ type: "text", text: "Goodbye, world!" }],
    };
    t.assert.deepStrictEqual(res3, expected3);
    t.assert.deepStrictEqual(model.trackedGenerateInputs[2], {
      messages: [
        { role: "user", content: [{ type: "text", text: "Goodbye" }] },
      ],
    });

    // Reset tracked inputs
    model.reset();
    t.assert.deepStrictEqual(model.trackedGenerateInputs, []);

    model.enqueueGenerateResult({
      response: {
        content: [{ type: "text", text: "After reset" }],
      },
    });

    // Restore the mock to its initial state
    model.restore();
    t.assert.deepStrictEqual(model.trackedGenerateInputs, []);
    await t.assert.rejects(() => {
      // No mocked results should be available after restore
      return model.generate({
        messages: [{ role: "user", content: [{ type: "text", text: "Hi" }] }],
      });
    }, /No mocked generate results available/);
  });

  test("tracks stream inputs and yields mocked partials", async (t: TestContext) => {
    const model = new MockLanguageModel();

    model.enqueueStreamResult(
      // First mocked stream response
      {
        partials: [
          { delta: { index: 0, part: { type: "text", text: "Hello" } } },
          { delta: { index: 0, part: { type: "text", text: ", " } } },
          { delta: { index: 0, part: { type: "text", text: "world!" } } },
        ],
      },
      // Second mocked stream response is an error
      {
        error: new Error("Stream error"),
      },
      // Third mocked stream response
      {
        partials: [
          { delta: { index: 0, part: { type: "text", text: "Goodbye" } } },
          { delta: { index: 0, part: { type: "text", text: ", " } } },
          { delta: { index: 0, part: { type: "text", text: "world!" } } },
        ],
      },
    );

    // First stream call should yield the first set of partials
    const partials1 = [];
    for await (const partial of model.stream({
      messages: [{ role: "user", content: [{ type: "text", text: "Hi" }] }],
    })) {
      partials1.push(partial);
    }
    const expectedPartials1 = [
      { delta: { index: 0, part: { type: "text", text: "Hello" } } },
      { delta: { index: 0, part: { type: "text", text: ", " } } },
      { delta: { index: 0, part: { type: "text", text: "world!" } } },
    ];
    t.assert.deepStrictEqual(partials1, expectedPartials1);
    t.assert.deepStrictEqual(model.trackedStreamInputs[0], {
      messages: [{ role: "user", content: [{ type: "text", text: "Hi" }] }],
    });

    // Second stream call should throw an error
    await t.assert.rejects(
      async () => {
        const partials2 = [];
        for await (const partial of model.stream({
          messages: [
            { role: "user", content: [{ type: "text", text: "Error" }] },
          ],
        })) {
          partials2.push(partial);
        }
        return partials2;
      },
      { message: "Stream error" },
    );
    t.assert.deepStrictEqual(model.trackedStreamInputs[1], {
      messages: [{ role: "user", content: [{ type: "text", text: "Error" }] }],
    });

    // Third stream call should yield the last set of partials
    const partials3 = [];
    for await (const partial of model.stream({
      messages: [
        { role: "user", content: [{ type: "text", text: "Goodbye" }] },
      ],
    })) {
      partials3.push(partial);
    }
    const expectedPartials3 = [
      { delta: { index: 0, part: { type: "text", text: "Goodbye" } } },
      { delta: { index: 0, part: { type: "text", text: ", " } } },
      { delta: { index: 0, part: { type: "text", text: "world!" } } },
    ];
    t.assert.deepStrictEqual(partials3, expectedPartials3);
    t.assert.deepStrictEqual(model.trackedStreamInputs[2], {
      messages: [
        { role: "user", content: [{ type: "text", text: "Goodbye" }] },
      ],
    });

    // Reset tracked inputs
    model.reset();
    t.assert.deepStrictEqual(model.trackedStreamInputs, []);

    model.enqueueStreamResult({
      partials: [
        { delta: { index: 0, part: { type: "text", text: "After reset" } } },
      ],
    });

    // Restore the mock to its initial state
    model.restore();
    t.assert.deepStrictEqual(model.trackedStreamInputs, []);
    await t.assert.rejects(async () => {
      // No mocked results should be available after restore
      const stream = model.stream({
        messages: [{ role: "user", content: [{ type: "text", text: "Hi" }] }],
      });
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      for await (const _partial of stream) {
        // noop
      }
    }, /No mocked stream results available/);
  });
});
