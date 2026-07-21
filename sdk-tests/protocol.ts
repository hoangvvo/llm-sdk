import { readFileSync } from "node:fs";

/* eslint-disable @typescript-eslint/no-explicit-any */

type DataObject = any;

interface TestData {
  tools: any[];
  test_cases: any[];
  profiles?: Record<string, any>;
}

export interface StageContext {
  stages: Array<{
    assistant: unknown[];
    tool_calls: unknown[];
  }>;
}

export interface PreparedStage {
  method: "generate" | "stream";
  input: unknown;
  stage_count: number;
}

export interface PreparedTransportStage extends PreparedStage {
  transport: {
    request: unknown;
    response: unknown;
  };
}

interface StageOptions {
  test_case: string;
  stage: number;
  context?: StageContext;
  profile?: string;
}

interface ValidateOutputOptions {
  test_case: string;
  stage: number;
  content: any[];
  response?: any;
  stream?: any;
  profile?: string;
}

interface ValidateErrorOptions {
  test_case: string;
  stage: number;
  error: { kind: string; message: string };
  profile?: string;
}

type TestCaseNames = Readonly<{
  GENERATE_TEXT: "generate_text";
  STREAM_TEXT: "stream_text";
  GENERATE_WITH_SYSTEM_PROMPT: "generate_with_system_prompt";
  GENERATE_TOOL_CALL: "generate_tool_call";
  STREAM_TOOL_CALL: "stream_tool_call";
  GENERATE_TEXT_FROM_TOOL_RESULT: "generate_text_from_tool_result";
  STREAM_TEXT_FROM_TOOL_RESULT: "stream_text_from_tool_result";
  GENERATE_TEXT_FROM_IMAGE_TOOL_RESULT: "generate_text_from_image_tool_result";
  GENERATE_PARALLEL_TOOL_CALLS: "generate_parallel_tool_calls";
  STREAM_PARALLEL_TOOL_CALLS: "stream_parallel_tool_calls";
  STREAM_PARALLEL_TOOL_CALLS_OF_SAME_NAME: "stream_parallel_tool_calls_of_same_name";
  STRUCTURED_RESPONSE_FORMAT: "structured_response_format";
  SOURCE_PART_INPUT: "source_part_input";
  GENERATE_AUDIO: "generate_audio";
  STREAM_AUDIO: "stream_audio";
  GENERATE_IMAGE: "generate_image";
  STREAM_IMAGE: "stream_image";
  GENERATE_IMAGE_INPUT: "generate_image_input";
  STREAM_IMAGE_INPUT: "stream_image_input";
  GENERATE_WEB_SEARCH: "generate_web_search";
  STREAM_WEB_SEARCH: "stream_web_search";
  ANTHROPIC_GENERATE_WEB_SEARCH_FAILURE: "anthropic_generate_web_search_failure";
  ANTHROPIC_STREAM_WEB_SEARCH_FAILURE: "anthropic_stream_web_search_failure";
  GENERATE_REASONING: "generate_reasoning";
  STREAM_REASONING: "stream_reasoning";
  MIXED_CONTENT_CONVERSATION: "mixed_content_conversation";
  MULTI_STEP_TOOL_WORKFLOW: "multi_step_tool_workflow";
  STREAM_NESTED_TOOL_ARGUMENTS: "stream_nested_tool_arguments";
  STREAM_NESTED_STRUCTURED_RESPONSE: "stream_nested_structured_response";
  GENERATE_EXPLICIT_TEXT_OPTIONS: "generate_explicit_text_options";
  CONSECUTIVE_USER_MESSAGES: "consecutive_user_messages";
  ASSISTANT_MESSAGE_HISTORY: "assistant_message_history";
  AUTO_TOOL_CHOICE_TEXT_RESPONSE: "auto_tool_choice_text_response";
  CANCELLED_TOOL_RESULT: "cancelled_tool_result";
  PARALLEL_TOOL_RESULTS: "parallel_tool_results";
  SEQUENTIAL_TOOL_CHAIN: "sequential_tool_chain";
  STRUCTURED_DATA_EXTRACTION: "structured_data_extraction";
  MULTIPLE_SOURCE_DOCUMENTS: "multiple_source_documents";
  STREAM_UNICODE_TEXT: "stream_unicode_text";
  MULTI_PART_TOOL_RESULT: "multi_part_tool_result";
  REASONING_TOOL_CONTINUATION: "reasoning_tool_continuation";
  ANTHROPIC_GENERATE_REFUSAL: "anthropic_generate_refusal";
  ANTHROPIC_STREAM_REFUSAL: "anthropic_stream_refusal";
  OPENAI_GENERATE: "openai_generate";
  OPENAI_STREAM: "openai_stream";
  OPENAI_HTTP_ERROR: "openai_http_error";
  OPENAI_CANCELLED_RESULT: "openai_cancelled_result";
  OPENAI_MALFORMED_STREAM: "openai_malformed_stream";
  ANTHROPIC_GENERATE: "anthropic_generate";
  ANTHROPIC_STREAM: "anthropic_stream";
  ANTHROPIC_HTTP_ERROR: "anthropic_http_error";
  ANTHROPIC_MALFORMED_STREAM: "anthropic_malformed_stream";
  GOOGLE_GENERATE: "google_generate";
  GOOGLE_STREAM: "google_stream";
  GOOGLE_HTTP_ERROR: "google_http_error";
  GOOGLE_MALFORMED_STREAM: "google_malformed_stream";
}>;

const TEST_DATA = JSON.parse(
  readFileSync(new URL("./tests.json", import.meta.url), "utf8"),
) as TestData;
const TRANSPORT_DATA = JSON.parse(
  readFileSync(new URL("./transports.json", import.meta.url), "utf8"),
) as TestData;
const ALL_TEST_CASES = [...TEST_DATA.test_cases, ...TRANSPORT_DATA.test_cases];

const PART_TYPES = new Set([
  "text",
  "tool_call",
  "web_search_call",
  "web_search_result",
  "audio",
  "image",
  "reasoning",
  "json",
]);

function fail(message: string): never {
  throw new Error(message);
}

function isObject(value: unknown): value is DataObject {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function clone<T>(value: T): T {
  return structuredClone(value);
}

function validateTestData(): void {
  if (
    !Array.isArray(TEST_DATA.tools) ||
    !Array.isArray(TEST_DATA.test_cases) ||
    !Array.isArray(TRANSPORT_DATA.test_cases)
  ) {
    fail("sdk-tests/tests.json must contain tools and test_cases arrays");
  }

  const names = new Set<string>();
  for (const testCase of ALL_TEST_CASES) {
    if (typeof testCase.name !== "string" || names.has(testCase.name)) {
      fail(`Invalid or duplicate test case name: ${String(testCase.name)}`);
    }
    names.add(testCase.name);
    if (!Array.isArray(testCase.stages) || testCase.stages.length === 0) {
      fail(`Test case "${testCase.name}" must contain at least one stage`);
    }
    if (
      testCase.groups !== undefined &&
      (!Array.isArray(testCase.groups) ||
        testCase.groups.some((group: any) => typeof group !== "string"))
    ) {
      fail(`Test case "${testCase.name}" has invalid groups`);
    }
    for (const [index, stage] of testCase.stages.entries()) {
      if (stage.type !== "generate" && stage.type !== "stream") {
        fail(`Invalid method for ${testCase.name} stage ${index}`);
      }
      const expectsOutput = Array.isArray(stage.expect?.content);
      const expectsError = isObject(stage.expect?.error);
      if (
        !isObject(stage.input) ||
        !isObject(stage.expect) ||
        expectsOutput === expectsError
      ) {
        fail(
          `Invalid input or expectation for ${testCase.name} stage ${index}`,
        );
      }
      if (
        expectsError &&
        (typeof stage.expect.error.kind !== "string" ||
          (stage.expect.error.message !== undefined &&
            typeof stage.expect.error.message !== "string"))
      ) {
        fail(`Invalid error expectation for ${testCase.name} stage ${index}`);
      }
      if (expectsError && index !== testCase.stages.length - 1) {
        fail(`Error expectation must be the final stage for ${testCase.name}`);
      }
      for (const assertion of stage.expect.content ?? []) {
        if (!PART_TYPES.has(assertion.type)) {
          fail(
            `Unsupported assertion type "${String(assertion.type)}" in ${testCase.name} stage ${index}`,
          );
        }
      }
      if (stage.expect.stream !== undefined && stage.type !== "stream") {
        fail(
          `Stream expectations require a stream stage for ${testCase.name} stage ${index}`,
        );
      }
      if (
        stage.transport !== undefined &&
        (!isObject(stage.transport.request) ||
          !isObject(stage.transport.response))
      ) {
        fail(`Invalid transport fixture for ${testCase.name} stage ${index}`);
      }
    }
  }

  for (const [name, profile] of Object.entries(TEST_DATA.profiles ?? {})) {
    if (
      !Array.isArray(profile.applies_to) ||
      profile.applies_to.some((testCaseName: any) => !names.has(testCaseName))
    ) {
      fail(`Profile "${name}" has invalid applies_to entries`);
    }
  }
}

validateTestData();

const TEST_CASES = new Map(
  ALL_TEST_CASES.map((testCase) => [testCase.name, testCase]),
);
const TOOLS = new Map(
  TEST_DATA.tools.map((tool) => [
    tool.type === "function" ? tool.name : tool.type,
    tool,
  ]),
);

export const TEST_CASE_NAMES = Object.freeze(
  Object.fromEntries(
    ALL_TEST_CASES.map((testCase) => [
      testCase.name.toUpperCase(),
      testCase.name,
    ]),
  ),
) as TestCaseNames;

function getTestCase(name: string): any {
  const testCase = TEST_CASES.get(name);
  if (!testCase) fail(`Test case "${name}" not found`);
  return testCase;
}

function getProfile(
  name: string | null | undefined,
  testCaseName: string,
): any {
  if (name === undefined || name === null) return undefined;
  const profile = TEST_DATA.profiles?.[name];
  if (!profile) fail(`Test profile "${name}" not found`);
  if (!profile.applies_to.includes(testCaseName)) {
    fail(`Test profile "${name}" does not apply to "${testCaseName}"`);
  }
  return profile;
}

function deepMerge(base: any, patch: any): any {
  if (!isObject(base) || !isObject(patch)) return clone(patch);
  const result = clone(base);
  for (const [key, value] of Object.entries(patch)) {
    result[key] =
      isObject(result[key]) && isObject(value)
        ? deepMerge(result[key], value)
        : clone(value);
  }
  return result;
}

function omitPath(value: any, segments: string[]): void {
  if (!isObject(value) && !Array.isArray(value)) return;
  const [segment, ...rest] = segments;
  if (segment === undefined) return;
  if (segment === "*") {
    for (const child of Array.isArray(value) ? value : Object.values(value)) {
      omitPath(child, rest);
    }
    return;
  }
  if (rest.length === 0) {
    delete value[segment];
    return;
  }
  omitPath(value[segment], rest);
}

function resolvePath(path: string, root: any): any {
  let current = root;
  for (const segment of path.split(".")) {
    const key = Array.isArray(current) ? Number(segment) : segment;
    if (current === null || typeof current !== "object" || !(key in current)) {
      fail(`Invalid stage ref path "${path}"`);
    }
    current = current[key];
  }
  return clone(current);
}

function readPath(path: string, root: any): any {
  let current = root;
  for (const segment of path.split(".")) {
    const key = Array.isArray(current) ? Number(segment) : segment;
    if (current === null || typeof current !== "object" || !(key in current)) {
      return undefined;
    }
    current = current[key];
  }
  return current;
}

function resolveRefs(value: any, context: StageContext): any {
  if (Array.isArray(value)) {
    return value.map((child) => resolveRefs(child, context));
  }
  if (isObject(value)) {
    if (typeof value.$ref === "string") {
      let resolved = resolvePath(value.$ref, context);
      if (value.$where !== undefined) {
        if (!Array.isArray(resolved) || !isObject(value.$where)) {
          fail(`Invalid filtered stage ref "${value.$ref}"`);
        }
        resolved = resolved.find(
          (candidate) =>
            isObject(candidate) &&
            Object.entries(value.$where).every(([key, expected]) =>
              Object.is(readPath(key, candidate), expected),
            ),
        );
        if (resolved === undefined) {
          fail(`No stage ref match for "${value.$ref}"`);
        }
      }
      if (value.$path !== undefined) {
        if (typeof value.$path !== "string") {
          fail(`Invalid relative stage ref path for "${value.$ref}"`);
        }
        resolved = resolvePath(value.$path, resolved);
      }
      return clone(resolved);
    }
    return Object.fromEntries(
      Object.entries(value).map(([key, child]) => [
        key,
        resolveRefs(child, context),
      ]),
    );
  }
  return value;
}

function resolveTools(input: any, names: string[] | undefined): any {
  if (names === undefined) return input;
  const tools = names.map((name) => {
    const tool = TOOLS.get(name);
    if (!tool) fail(`Tool "${name}" not found in test data`);
    return clone(tool);
  });
  return { ...input, tools };
}

export function getTestCaseInfo(testCaseName: string): {
  name: string;
  stage_count: number;
} {
  const testCase = getTestCase(testCaseName);
  return { name: testCase.name, stage_count: testCase.stages.length };
}

export function getTestCasesByGroup(group: string): string[] {
  if (typeof group !== "string" || group.length === 0) {
    fail("Test case group must be a non-empty string");
  }
  const testCases = ALL_TEST_CASES.filter((testCase) =>
    testCase.groups?.includes(group),
  ).map((testCase) => testCase.name);
  if (testCases.length === 0) fail(`Test case group "${group}" not found`);
  return testCases;
}

export function prepareStage({
  test_case: testCaseName,
  stage: stageIndex,
  context = { stages: [] },
  profile: profileName,
}: StageOptions): PreparedStage {
  const testCase = getTestCase(testCaseName);
  const stage = testCase.stages[stageIndex];
  if (!stage)
    fail(`Stage ${stageIndex} not found in test case "${testCaseName}"`);
  const profile = getProfile(profileName, testCaseName);

  let input = resolveTools(clone(stage.input), stage.input_tools);
  if (profile?.input_merge) input = deepMerge(input, profile.input_merge);
  for (const path of profile?.input_omit ?? []) {
    omitPath(input, path.split("."));
  }
  input = resolveRefs(input, context);

  return {
    method: stage.type,
    input,
    stage_count: testCase.stages.length,
  };
}

export function prepareTransportStage(
  options: StageOptions,
): PreparedTransportStage {
  const prepared = prepareStage(options);
  const testCase = getTestCase(options.test_case);
  const stage = testCase.stages[options.stage];
  if (!isObject(stage?.transport)) {
    fail(
      `Test case "${options.test_case}" stage ${String(options.stage)} has no transport fixture`,
    );
  }
  return { ...prepared, transport: clone(stage.transport) };
}

function regexMatches(pattern: string, value: unknown): boolean {
  if (typeof value !== "string") return false;
  try {
    return new RegExp(pattern, "s").test(value);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    fail(`Invalid test pattern ${JSON.stringify(pattern)}: ${message}`);
  }
}

function valueMatches(expected: any, actual: any): boolean {
  if (Object.is(expected, actual)) return true;
  if (
    isObject(expected) &&
    Object.keys(expected).length === 1 &&
    typeof expected.$regex === "string"
  ) {
    return regexMatches(expected.$regex, actual);
  }
  if (Array.isArray(expected) && Array.isArray(actual)) {
    return (
      expected.length === actual.length &&
      expected.every((value, index) => valueMatches(value, actual[index]))
    );
  }
  if (isObject(expected) && isObject(actual)) {
    const expectedKeys = Object.keys(expected);
    const actualKeys = Object.keys(actual);
    return (
      expectedKeys.length === actualKeys.length &&
      expectedKeys.every(
        (key) => key in actual && valueMatches(expected[key], actual[key]),
      )
    );
  }
  return false;
}

function citationMatches(assertion: any, citation: any): boolean {
  return (
    isObject(citation) &&
    (assertion.source === undefined ||
      regexMatches(assertion.source, citation.source)) &&
    (assertion.title === undefined ||
      regexMatches(assertion.title, citation.title)) &&
    (assertion.cited_text === undefined ||
      regexMatches(assertion.cited_text, citation.cited_text)) &&
    (assertion.any_of === undefined ||
      assertion.any_of.some((option: any) => citationMatches(option, citation)))
  );
}

function parseJsonText(text: unknown): any {
  if (typeof text !== "string") return undefined;
  try {
    return JSON.parse(text);
  } catch {
    return undefined;
  }
}

function assertionMatches(assertion: any, part: any): boolean {
  switch (assertion.type) {
    case "text":
      return (
        part.type === "text" &&
        (assertion.text === undefined ||
          regexMatches(assertion.text, part.text)) &&
        (assertion.contains_all === undefined ||
          assertion.contains_all.every((value: any) =>
            part.text.includes(value),
          )) &&
        (assertion.citation === undefined ||
          part.citations?.some((citation: any) =>
            citationMatches(assertion.citation, citation),
          ))
      );
    case "tool_call":
      return (
        (part.type === "tool-call" || part.type === "tool_call") &&
        (part.call?.type ?? "function") === "function" &&
        (part.call?.name ?? part.tool_name) === assertion.tool_name &&
        (assertion.tool_call_id === false ||
          (typeof part.tool_call_id === "string" &&
            part.tool_call_id.length > 0)) &&
        (assertion.args === undefined ||
          valueMatches(assertion.args, part.call?.args ?? part.args))
      );
    case "web_search_call":
      return (
        (part.type === "tool-call" || part.type === "tool_call") &&
        part.call?.type === "web_search" &&
        typeof part.tool_call_id === "string" &&
        part.tool_call_id.length > 0 &&
        (assertion.status === undefined ||
          part.call.status === assertion.status) &&
        (assertion.action !== true || isObject(part.call.action))
      );
    case "web_search_result":
      return (
        (part.type === "tool-result" || part.type === "tool_result") &&
        part.result?.type === "web_search" &&
        Array.isArray(part.result.sources) &&
        (assertion.status === undefined || part.status === assertion.status) &&
        (assertion.error_code === undefined ||
          regexMatches(assertion.error_code, part.result.error_code)) &&
        (assertion.source === undefined ||
          part.result.sources.some((source: any) =>
            regexMatches(assertion.source, source.url),
          )) &&
        (assertion.source_signature !== true ||
          part.result.sources.some(
            (source: any) =>
              typeof source.signature === "string" &&
              source.signature.length > 0,
          ))
      );
    case "audio":
      return (
        part.type === "audio" &&
        typeof part.data === "string" &&
        part.data.length > 0 &&
        typeof part.format === "string" &&
        part.format.length > 0 &&
        (!assertion.id ||
          (typeof part.id === "string" && part.id.length > 0)) &&
        (assertion.transcript === undefined ||
          regexMatches(assertion.transcript, part.transcript))
      );
    case "image":
      return (
        part.type === "image" &&
        typeof part.data === "string" &&
        part.data.length > 0 &&
        typeof part.mime_type === "string" &&
        /^image\//.test(part.mime_type) &&
        (!assertion.id || (typeof part.id === "string" && part.id.length > 0))
      );
    case "reasoning":
      return (
        part.type === "reasoning" &&
        regexMatches(assertion.text, part.text) &&
        (!assertion.signature ||
          (typeof part.signature === "string" && part.signature.length > 0))
      );
    case "json":
      return (
        part.type === "text" &&
        valueMatches(assertion.value, parseJsonText(part.text))
      );
    default:
      return false;
  }
}

function findDistinctMatches(
  assertions: any[],
  content: any[],
  index = 0,
  used = new Set<number>(),
): boolean {
  if (index === assertions.length) return true;
  for (const [partIndex, part] of content.entries()) {
    if (!used.has(partIndex) && assertionMatches(assertions[index], part)) {
      used.add(partIndex);
      if (findDistinctMatches(assertions, content, index + 1, used))
        return true;
      used.delete(partIndex);
    }
  }
  return false;
}

function formatValidationFailure(
  testCaseName: string,
  stageIndex: number,
  expected: any,
  content: any[],
): string {
  return [
    `Output validation failed for "${testCaseName}" stage ${stageIndex}.`,
    "Expected each assertion to match a distinct content part:",
    JSON.stringify(expected, null, 2),
    "Received:",
    JSON.stringify(content, null, 2),
  ].join("\n");
}

function validateUsage(
  testCaseName: string,
  stageIndex: number,
  usage: any,
): void {
  if (!isObject(usage)) {
    fail(
      `Output validation failed for "${testCaseName}" stage ${stageIndex}: expected usage metadata.`,
    );
  }
  for (const key of ["input_tokens", "output_tokens"]) {
    if (!Number.isInteger(usage[key]) || usage[key] < 0) {
      fail(
        `Output validation failed for "${testCaseName}" stage ${stageIndex}: usage.${key} must be a non-negative integer.`,
      );
    }
  }
  if (usage.input_tokens === 0 && usage.output_tokens === 0) {
    fail(
      `Output validation failed for "${testCaseName}" stage ${stageIndex}: usage must contain at least one token.`,
    );
  }
  for (const detailsKey of ["input_tokens_details", "output_tokens_details"]) {
    const details = usage[detailsKey];
    if (details === undefined) continue;
    if (
      !isObject(details) ||
      (Object.values(details) as any[]).some(
        (value) => !Number.isInteger(value) || value < 0,
      )
    ) {
      fail(
        `Output validation failed for "${testCaseName}" stage ${stageIndex}: usage.${detailsKey} values must be non-negative integers.`,
      );
    }
  }
}

function validateResponseMetadata({
  testCaseName,
  stageIndex,
  expected,
  response,
  stream,
}: any): void {
  if (response !== undefined) {
    if (!isObject(response) || !Array.isArray(response.content)) {
      fail(
        `Output validation failed for "${testCaseName}" stage ${stageIndex}: response must contain a content array.`,
      );
    }
    if (
      response.cost !== undefined &&
      (typeof response.cost !== "number" ||
        !Number.isFinite(response.cost) ||
        response.cost < 0)
    ) {
      fail(
        `Output validation failed for "${testCaseName}" stage ${stageIndex}: cost must be a non-negative finite number.`,
      );
    }
  }

  if (expected.usage === true) {
    validateUsage(testCaseName, stageIndex, response?.usage);
  } else if (
    isObject(expected.usage) &&
    !valueContains(expected.usage, response?.usage)
  ) {
    fail(
      `Output validation failed for "${testCaseName}" stage ${stageIndex}: expected usage ${JSON.stringify(expected.usage)}, received ${JSON.stringify(response?.usage)}.`,
    );
  }

  if (expected.stream !== undefined) {
    if (!isObject(stream)) {
      fail(
        `Output validation failed for "${testCaseName}" stage ${stageIndex}: expected stream metrics.`,
      );
    }
    for (const [metric, minimum] of Object.entries(expected.stream) as Array<
      [string, number]
    >) {
      if (
        !Number.isInteger(minimum) ||
        minimum < 0 ||
        !Number.isInteger(stream[metric]) ||
        stream[metric] < minimum
      ) {
        fail(
          `Output validation failed for "${testCaseName}" stage ${stageIndex}: stream.${metric} must be at least ${String(minimum)}, received ${String(stream[metric])}.`,
        );
      }
    }
  }
}

export function validateOutput({
  test_case: testCaseName,
  stage: stageIndex,
  content,
  response,
  stream,
  profile: profileName,
}: ValidateOutputOptions): { ok: true } {
  const testCase = getTestCase(testCaseName);
  const stage = testCase.stages[stageIndex];
  if (!stage)
    fail(`Stage ${stageIndex} not found in test case "${testCaseName}"`);
  if (!Array.isArray(content)) fail("Model output content must be an array");
  const profile = getProfile(profileName, testCaseName);
  const expected = clone(profile?.expect ?? stage.expect);
  for (const rule of profile?.expect_omit ?? []) {
    if (rule.method === undefined || rule.method === stage.type) {
      omitPath(expected, rule.path.split("."));
    }
  }
  if (expected.error !== undefined) {
    fail(
      `Output validation failed for "${testCaseName}" stage ${stageIndex}: expected ${expected.error.kind} error, received model output.`,
    );
  }
  validateResponseMetadata({
    testCaseName,
    stageIndex,
    expected,
    response,
    stream,
  });
  const aggregateAssertions = expected.content.filter(
    (assertion: any) =>
      assertion.type === "text" && assertion.aggregate === true,
  );
  const partAssertions = expected.content.filter(
    (assertion: any) => !aggregateAssertions.includes(assertion),
  );
  const aggregateText = content
    .filter((part) => part.type === "text")
    .map((part) => part.text)
    .join("\n");

  if (
    aggregateAssertions.some(
      (assertion: any) =>
        !assertionMatches(assertion, { type: "text", text: aggregateText }),
    ) ||
    !findDistinctMatches(partAssertions, content)
  ) {
    fail(formatValidationFailure(testCaseName, stageIndex, expected, content));
  }

  const expectedToolCalls = expected.content.filter(
    (part: any) => part.type === "tool_call" || part.type === "web_search_call",
  ).length;
  const actualToolCalls = content.filter(
    (part) => part.type === "tool-call" || part.type === "tool_call",
  );
  if (
    expected.allow_extra_tool_calls !== true &&
    actualToolCalls.length !== expectedToolCalls
  ) {
    fail(
      `Output validation failed for "${testCaseName}" stage ${stageIndex}: expected exactly ${expectedToolCalls} tool call(s), received ${actualToolCalls.length}.`,
    );
  }

  const ids = actualToolCalls.map((part) => part.tool_call_id);
  if (new Set(ids).size !== ids.length) {
    fail(
      `Output validation failed for "${testCaseName}" stage ${stageIndex}: tool_call_id values must be unique.`,
    );
  }

  return { ok: true };
}

export function validateError({
  test_case: testCaseName,
  stage: stageIndex,
  error,
  profile: profileName,
}: ValidateErrorOptions): { ok: true } {
  const testCase = getTestCase(testCaseName);
  const stage = testCase.stages[stageIndex];
  if (!stage)
    fail(`Stage ${stageIndex} not found in test case "${testCaseName}"`);
  const profile = getProfile(profileName, testCaseName);
  const expected = clone(profile?.expect ?? stage.expect);
  if (!isObject(expected.error)) {
    fail(
      `Unexpected model error for "${testCaseName}" stage ${stageIndex}: ${String(error?.kind)}: ${String(error?.message)}`,
    );
  }
  if (
    !isObject(error) ||
    !regexMatches(expected.error.kind, error.kind) ||
    (expected.error.message !== undefined &&
      !regexMatches(expected.error.message, error.message))
  ) {
    fail(
      [
        `Error validation failed for "${testCaseName}" stage ${stageIndex}.`,
        `Expected: ${JSON.stringify(expected.error)}`,
        `Received: ${JSON.stringify(error)}`,
      ].join("\n"),
    );
  }
  return { ok: true };
}

function valueContains(expected: any, actual: any): boolean {
  if (Array.isArray(expected)) {
    return (
      Array.isArray(actual) &&
      expected.length === actual.length &&
      expected.every((value, index) => valueContains(value, actual[index]))
    );
  }
  if (isObject(expected)) {
    return (
      isObject(actual) &&
      Object.entries(expected).every(
        ([key, value]) => key in actual && valueContains(value, actual[key]),
      )
    );
  }
  return Object.is(expected, actual);
}

export function validateTransportRequest(
  expected: unknown,
  actual: unknown,
): { ok: true } {
  if (!isObject(expected) || !isObject(actual)) {
    fail("Transport request validation requires request objects");
  }
  if (
    expected.method !== undefined &&
    actual.method?.toUpperCase() !== expected.method.toUpperCase()
  ) {
    fail(
      `Expected transport method ${JSON.stringify(expected.method)}, received ${JSON.stringify(actual.method)}`,
    );
  }
  if (
    expected.path !== undefined &&
    !regexMatches(`^(?:${expected.path})$`, actual.path)
  ) {
    fail(
      `Expected transport path /${expected.path}/, received ${JSON.stringify(actual.path)}`,
    );
  }
  for (const [name, pattern] of Object.entries(expected.headers ?? {}) as Array<
    [string, string]
  >) {
    if (!regexMatches(pattern, actual.headers?.[name.toLowerCase()])) {
      fail(
        `Expected transport header ${JSON.stringify(name)} to match ${JSON.stringify(pattern)}, received ${JSON.stringify(actual.headers?.[name.toLowerCase()])}`,
      );
    }
  }
  if (
    expected.body !== undefined &&
    !valueContains(expected.body, actual.body)
  ) {
    fail(
      [
        "Transport request body did not contain the expected value.",
        `Expected subset: ${JSON.stringify(expected.body, null, 2)}`,
        `Received: ${JSON.stringify(actual.body, null, 2)}`,
      ].join("\n"),
    );
  }
  return { ok: true };
}
