import {
  runTestCase,
  TEST_CASE_NAMES,
  type RunTestCaseOptions,
} from "#test-common/cases";
import assert from "node:assert";
import test, { suite } from "node:test";
import type { JSONSchema, LanguageModelInput } from "../types.ts";
import { AnthropicModel } from "./anthropic.ts";

suite("AnthropicModel", () => {
  assert(process.env["ANTHROPIC_API_KEY"], "ANTHROPIC_API_KEY must be set");
  const model = new AnthropicModel({
    apiKey: process.env["ANTHROPIC_API_KEY"],
    modelId: "claude-sonnet-4-5",
  });

  function patchAnthropicStrictToolSchema(input: LanguageModelInput) {
    return {
      ...input,
      ...(input.tools && {
        tools: input.tools.map((tool) => ({
          ...tool,
          parameters: patchAnthropicToolSchema(tool.name, tool.parameters),
        })),
      }),
    };
  }

  function patchAnthropicToolSchema(name: string, parameters: JSONSchema) {
    if (name !== "get_weather") return parameters;
    const parametersProperties = parameters["properties"] as {
      preferred_unit?: JSONSchema;
    };

    // Temporary Anthropic test workaround: strict tools currently reject the
    // shared nullable-enum shape on get_weather.preferred_unit in practice.
    return {
      ...parameters,
      properties: {
        ...parametersProperties,
        preferred_unit: {
          ...parametersProperties.preferred_unit,
          type: "string",
        },
      },
    };
  }

  function withAnthropicCompat(
    options?: RunTestCaseOptions,
  ): RunTestCaseOptions | undefined {
    return {
      ...options,
      additionalInputs: (input) => {
        const patched = patchAnthropicStrictToolSchema(input);
        return options?.additionalInputs
          ? options.additionalInputs(patched)
          : patched;
      },
    };
  }

  const reasoningOptions: RunTestCaseOptions = {
    additionalInputs: (input) => ({
      ...input,
      reasoning: {
        enabled: true,
        budget_tokens: 3000,
      },
    }),
  };

  test(TEST_CASE_NAMES.GENERATE_TEXT, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.GENERATE_TEXT,
      withAnthropicCompat(),
    );
  });

  test(TEST_CASE_NAMES.STREAM_TEXT, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.STREAM_TEXT,
      withAnthropicCompat(),
    );
  });

  test(TEST_CASE_NAMES.GENERATE_WITH_SYSTEM_PROMPT, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.GENERATE_WITH_SYSTEM_PROMPT,
      withAnthropicCompat(),
    );
  });

  test(TEST_CASE_NAMES.GENERATE_TOOL_CALL, { timeout: 60 * 1000 }, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.GENERATE_TOOL_CALL,
      withAnthropicCompat(),
    );
  });

  test(TEST_CASE_NAMES.STREAM_TOOL_CALL, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.STREAM_TOOL_CALL,
      withAnthropicCompat(),
    );
  });

  test(TEST_CASE_NAMES.GENERATE_TEXT_FROM_TOOL_RESULT, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.GENERATE_TEXT_FROM_TOOL_RESULT,
      withAnthropicCompat(),
    );
  });

  test(TEST_CASE_NAMES.STREAM_TEXT_FROM_TOOL_RESULT, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.STREAM_TEXT_FROM_TOOL_RESULT,
      withAnthropicCompat(),
    );
  });

  test(TEST_CASE_NAMES.GENERATE_TEXT_FROM_IMAGE_TOOL_RESULT, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.GENERATE_TEXT_FROM_IMAGE_TOOL_RESULT,
      withAnthropicCompat(),
    );
  });

  test(TEST_CASE_NAMES.GENERATE_PARALLEL_TOOL_CALLS, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.GENERATE_PARALLEL_TOOL_CALLS,
      withAnthropicCompat(),
    );
  });

  test(TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS,
      withAnthropicCompat(),
    );
  });

  test(TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME,
      withAnthropicCompat(),
    );
  });

  test(TEST_CASE_NAMES.STRUCTURED_RESPONSE_FORMAT, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.STRUCTURED_RESPONSE_FORMAT,
      withAnthropicCompat(),
    );
  });

  test(TEST_CASE_NAMES.SOURCE_PART_INPUT, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.SOURCE_PART_INPUT,
      withAnthropicCompat(),
    );
  });

  test(
    TEST_CASE_NAMES.GENERATE_IMAGE,
    { skip: "model does not support image generation" },
    (t) => {
      return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_IMAGE);
    },
  );

  test(
    TEST_CASE_NAMES.STREAM_IMAGE,
    { skip: "model does not support image generation" },
    (t) => {
      return runTestCase(t, model, TEST_CASE_NAMES.STREAM_IMAGE);
    },
  );

  test(TEST_CASE_NAMES.GENERATE_IMAGE_INPUT, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.GENERATE_IMAGE_INPUT,
      withAnthropicCompat(),
    );
  });

  test(TEST_CASE_NAMES.STREAM_IMAGE_INPUT, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.STREAM_IMAGE_INPUT,
      withAnthropicCompat(),
    );
  });

  test(
    TEST_CASE_NAMES.GENERATE_AUDIO,
    { skip: "model does not support audio" },
    (t) => {
      return runTestCase(t, model, TEST_CASE_NAMES.GENERATE_AUDIO);
    },
  );

  test(
    TEST_CASE_NAMES.STREAM_AUDIO,
    { skip: "model does not support audio" },
    (t) => {
      return runTestCase(t, model, TEST_CASE_NAMES.STREAM_AUDIO);
    },
  );

  test(TEST_CASE_NAMES.GENERATE_REASONING, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.GENERATE_REASONING,
      withAnthropicCompat(reasoningOptions),
    );
  });

  test(TEST_CASE_NAMES.STREAM_REASONING, (t) => {
    return runTestCase(
      t,
      model,
      TEST_CASE_NAMES.STREAM_REASONING,
      withAnthropicCompat(reasoningOptions),
    );
  });
});
