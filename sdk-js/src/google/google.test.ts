import { runTestCase, TEST_CASE_NAMES } from "#test-common/cases";
import assert from "node:assert";
import test, { suite } from "node:test";
import type { ContentDelta } from "../types.ts";
import { GoogleModel } from "./google.ts";

suite("GoogleModel", () => {
  assert(process.env["GOOGLE_API_KEY"], "GOOGLE_API_KEY must be set");
  const model = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-3.1-flash-lite",
  });

  const audioModel = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-3.1-flash-tts-preview",
  });

  const imageModel = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-3.1-flash-image",
  });

  const multimodalToolModel = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-3.1-pro-preview",
  });

  const thinkingModel = new GoogleModel({
    apiKey: process.env["GOOGLE_API_KEY"],
    modelId: "gemini-3.1-pro-preview",
  });

  test(TEST_CASE_NAMES.GENERATE_TEXT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_TEXT);
  });

  test(TEST_CASE_NAMES.STREAM_TEXT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_TEXT);
  });

  test(TEST_CASE_NAMES.GENERATE_WITH_SYSTEM_PROMPT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_WITH_SYSTEM_PROMPT);
  });

  test(TEST_CASE_NAMES.GENERATE_TOOL_CALL, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_TOOL_CALL);
  });

  test(TEST_CASE_NAMES.STREAM_TOOL_CALL, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_TOOL_CALL);
  });

  test(TEST_CASE_NAMES.GENERATE_TEXT_FROM_TOOL_RESULT, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.GENERATE_TEXT_FROM_TOOL_RESULT,
    );
  });

  test(TEST_CASE_NAMES.STREAM_TEXT_FROM_TOOL_RESULT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_TEXT_FROM_TOOL_RESULT);
  });

  test(TEST_CASE_NAMES.GENERATE_TEXT_FROM_IMAGE_TOOL_RESULT, (t) => {
    return runTestCase(
      t,
      multimodalToolModel,
      TEST_CASE_NAMES.GENERATE_TEXT_FROM_IMAGE_TOOL_RESULT,
    );
  });

  test(TEST_CASE_NAMES.GENERATE_PARALLEL_TOOL_CALLS, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_PARALLEL_TOOL_CALLS);
  });

  test(TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS);
  });

  test(TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME,
    );
  });

  test(TEST_CASE_NAMES.STRUCTURED_RESPONSE_FORMAT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.STRUCTURED_RESPONSE_FORMAT);
  });

  test(TEST_CASE_NAMES.SOURCE_PART_INPUT, (t) => {
    return runTestCase(t, model, TEST_CASE_NAMES.SOURCE_PART_INPUT);
  });

  const googleWebSearchOptions = {
    additionalInputs: (input: Parameters<typeof model.generate>[0]) => ({
      ...input,
      tools: input.tools?.map((tool) =>
        tool.type === "web_search" ? { type: "web_search" as const } : tool,
      ),
    }),
  };

  test(TEST_CASE_NAMES.GENERATE_WEB_SEARCH, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.GENERATE_WEB_SEARCH,
      googleWebSearchOptions,
    );
  });

  test(TEST_CASE_NAMES.STREAM_WEB_SEARCH, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.STREAM_WEB_SEARCH,
      googleWebSearchOptions,
    );
  });

  test("maps citations using provider part indexes before filtering", async (t) => {
    t.mock.method(globalThis, "fetch", () =>
      Promise.resolve(
        new Response(
          JSON.stringify({
            candidates: [
              {
                content: {
                  parts: [{}, { text: "first" }, { text: "second" }],
                },
                groundingMetadata: {
                  groundingChunks: [
                    {
                      web: {
                        uri: "https://example.com",
                        title: "Example",
                      },
                    },
                  ],
                  groundingSupports: [
                    {
                      segment: { partIndex: 2, text: "second" },
                      groundingChunkIndices: [0, 0],
                    },
                  ],
                },
              },
            ],
          }),
          { headers: { "content-type": "application/json" } },
        ),
      ),
    );

    const response = await new GoogleModel({
      apiKey: "test",
      modelId: "test",
    }).generate({
      messages: [{ role: "user", content: [{ type: "text", text: "test" }] }],
    });

    assert.deepStrictEqual(response.content, [
      { type: "text", text: "first" },
      {
        type: "text",
        text: "second",
        citations: [
          {
            source: "https://example.com",
            title: "Example",
            cited_text: "second",
          },
          {
            source: "https://example.com",
            title: "Example",
            cited_text: "second",
          },
        ],
      },
    ]);
  });

  test("streams citations on the explicitly mapped text part", async (t) => {
    const streamEvents = [
      {
        candidates: [
          {
            content: {
              parts: [{}, { text: "first" }, { text: "second" }],
            },
          },
        ],
      },
      {
        candidates: [
          {
            content: { parts: [{}, {}, {}] },
            groundingMetadata: {
              groundingChunks: [
                {
                  web: {
                    uri: "https://example.com",
                    title: "Example",
                  },
                },
              ],
              groundingSupports: [
                {
                  segment: { partIndex: 2, text: "second" },
                  groundingChunkIndices: [0, 0],
                },
              ],
            },
          },
        ],
      },
    ];
    t.mock.method(globalThis, "fetch", () =>
      Promise.resolve(
        new Response(
          streamEvents
            .map((event) => `data: ${JSON.stringify(event)}\n\n`)
            .join(""),
          { headers: { "content-type": "text/event-stream" } },
        ),
      ),
    );

    const deltas: ContentDelta[] = [];
    const stream = new GoogleModel({
      apiKey: "test",
      modelId: "test",
    }).stream({
      messages: [{ role: "user", content: [{ type: "text", text: "test" }] }],
    });
    for await (const event of stream) {
      if (event.delta) deltas.push(event.delta);
    }

    assert.deepStrictEqual(
      deltas.map((delta) => delta.index),
      [0, 1, 1, 1],
    );
    assert.deepStrictEqual(
      deltas.slice(2).map((delta) => delta.part),
      [
        {
          type: "text",
          text: "",
          citation: {
            type: "citation",
            source: "https://example.com",
            title: "Example",
            cited_text: "second",
          },
        },
        {
          type: "text",
          text: "",
          citation: {
            type: "citation",
            source: "https://example.com",
            title: "Example",
            cited_text: "second",
          },
        },
      ],
    );
  });

  test(TEST_CASE_NAMES.GENERATE_IMAGE, { timeout: 60 * 1000 }, (t) => {
    return runTestCase(t, imageModel, TEST_CASE_NAMES.GENERATE_IMAGE);
  });

  test(TEST_CASE_NAMES.STREAM_IMAGE, { timeout: 60 * 1000 }, (t) => {
    return runTestCase(t, imageModel, TEST_CASE_NAMES.STREAM_IMAGE);
  });

  test(TEST_CASE_NAMES.GENERATE_IMAGE_INPUT, { timeout: 60 * 1000 }, (t) => {
    return runTestCase(t, imageModel, TEST_CASE_NAMES.GENERATE_IMAGE_INPUT);
  });

  test(TEST_CASE_NAMES.STREAM_IMAGE_INPUT, { timeout: 60 * 1000 }, (t) => {
    return runTestCase(t, imageModel, TEST_CASE_NAMES.STREAM_IMAGE_INPUT);
  });

  test(TEST_CASE_NAMES.GENERATE_AUDIO, (t) => {
    return runTestCase(t, audioModel, TEST_CASE_NAMES.GENERATE_AUDIO, {
      additionalInputs: (input) => ({
        ...input,
        modalities: ["audio"],
        audio: {
          voice: "Zephyr",
        },
      }),
      customOutputContent: (content) =>
        content.map((part) => {
          if (part.type === "audio") {
            return { ...part, id: false, transcript: undefined };
          }
          return part;
        }),
    });
  });

  test(TEST_CASE_NAMES.STREAM_AUDIO, (t) => {
    return runTestCase(t, audioModel, TEST_CASE_NAMES.STREAM_AUDIO, {
      additionalInputs: (input) => ({
        ...input,
        modalities: ["audio"],
        audio: {
          voice: "Zephyr",
        },
      }),
      customOutputContent: (content) =>
        content.map((part) => {
          if (part.type === "audio") {
            return { ...part, id: false, transcript: undefined };
          }
          return part;
        }),
    });
  });

  test(TEST_CASE_NAMES.GENERATE_REASONING, (t) => {
    return runTestCase(t, thinkingModel, TEST_CASE_NAMES.GENERATE_REASONING);
  });

  test(TEST_CASE_NAMES.STREAM_REASONING, (t) => {
    return runTestCase(t, thinkingModel, TEST_CASE_NAMES.STREAM_REASONING);
  });
});
