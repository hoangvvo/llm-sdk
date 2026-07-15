import { readFileSync } from "node:fs";

const TEST_DATA = JSON.parse(
  readFileSync(new URL("./tests.json", import.meta.url), "utf8"),
);

const PART_TYPES = new Set([
  "text",
  "tool_call",
  "audio",
  "image",
  "reasoning",
  "json",
]);

function fail(message) {
  throw new Error(message);
}

function isObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function clone(value) {
  return structuredClone(value);
}

function validateTestData() {
  if (!Array.isArray(TEST_DATA.tools) || !Array.isArray(TEST_DATA.test_cases)) {
    fail("sdk-tests/tests.json must contain tools and test_cases arrays");
  }

  const names = new Set();
  for (const testCase of TEST_DATA.test_cases) {
    if (typeof testCase.name !== "string" || names.has(testCase.name)) {
      fail(`Invalid or duplicate test case name: ${String(testCase.name)}`);
    }
    names.add(testCase.name);
    if (!Array.isArray(testCase.stages) || testCase.stages.length === 0) {
      fail(`Test case "${testCase.name}" must contain at least one stage`);
    }
    for (const [index, stage] of testCase.stages.entries()) {
      if (stage.type !== "generate" && stage.type !== "stream") {
        fail(`Invalid method for ${testCase.name} stage ${index}`);
      }
      if (!isObject(stage.input) || !Array.isArray(stage.expect?.content)) {
        fail(
          `Invalid input or expectation for ${testCase.name} stage ${index}`,
        );
      }
      for (const assertion of stage.expect.content) {
        if (!PART_TYPES.has(assertion.type)) {
          fail(
            `Unsupported assertion type "${String(assertion.type)}" in ${testCase.name} stage ${index}`,
          );
        }
      }
    }
  }

  for (const [name, profile] of Object.entries(TEST_DATA.profiles ?? {})) {
    if (
      !Array.isArray(profile.applies_to) ||
      profile.applies_to.some((testCaseName) => !names.has(testCaseName))
    ) {
      fail(`Profile "${name}" has invalid applies_to entries`);
    }
  }
}

validateTestData();

const TEST_CASES = new Map(
  TEST_DATA.test_cases.map((testCase) => [testCase.name, testCase]),
);
const TOOLS = new Map(
  TEST_DATA.tools.map((tool) => [
    tool.type === "function" ? tool.name : tool.type,
    tool,
  ]),
);

export const TEST_CASE_NAMES = Object.freeze(
  Object.fromEntries(
    TEST_DATA.test_cases.map((testCase) => [
      testCase.name.toUpperCase(),
      testCase.name,
    ]),
  ),
);

function getTestCase(name) {
  const testCase = TEST_CASES.get(name);
  if (!testCase) fail(`Test case "${name}" not found`);
  return testCase;
}

function getProfile(name, testCaseName) {
  if (name === undefined || name === null) return undefined;
  const profile = TEST_DATA.profiles?.[name];
  if (!profile) fail(`Test profile "${name}" not found`);
  if (!profile.applies_to.includes(testCaseName)) {
    fail(`Test profile "${name}" does not apply to "${testCaseName}"`);
  }
  return profile;
}

function deepMerge(base, patch) {
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

function omitPath(value, segments) {
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

function resolvePath(path, root) {
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

function resolveRefs(value, context) {
  if (Array.isArray(value)) {
    return value.map((child) => resolveRefs(child, context));
  }
  if (isObject(value)) {
    const keys = Object.keys(value);
    if (keys.length === 1 && typeof value.$ref === "string") {
      return resolvePath(value.$ref, context);
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

function resolveTools(input, names) {
  if (names === undefined) return input;
  const tools = names.map((name) => {
    const tool = TOOLS.get(name);
    if (!tool) fail(`Tool "${name}" not found in test data`);
    return clone(tool);
  });
  return { ...input, tools };
}

export function getTestCaseInfo(testCaseName) {
  const testCase = getTestCase(testCaseName);
  return { name: testCase.name, stage_count: testCase.stages.length };
}

export function prepareStage({
  test_case: testCaseName,
  stage: stageIndex,
  context = { stages: [] },
  profile: profileName,
}) {
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

function regexMatches(pattern, value) {
  if (typeof value !== "string") return false;
  try {
    return new RegExp(pattern, "s").test(value);
  } catch (error) {
    fail(`Invalid test pattern ${JSON.stringify(pattern)}: ${error.message}`);
  }
}

function valueMatches(expected, actual) {
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

function citationMatches(assertion, citation) {
  return (
    isObject(citation) &&
    (assertion.source === undefined ||
      regexMatches(assertion.source, citation.source)) &&
    (assertion.title === undefined ||
      regexMatches(assertion.title, citation.title)) &&
    (assertion.cited_text === undefined ||
      regexMatches(assertion.cited_text, citation.cited_text)) &&
    (assertion.any_of === undefined ||
      assertion.any_of.some((option) => citationMatches(option, citation)))
  );
}

function parseJsonText(text) {
  if (typeof text !== "string") return undefined;
  try {
    return JSON.parse(text);
  } catch {
    return undefined;
  }
}

function assertionMatches(assertion, part) {
  switch (assertion.type) {
    case "text":
      return (
        part.type === "text" &&
        (assertion.text === undefined ||
          regexMatches(assertion.text, part.text)) &&
        (assertion.contains_all === undefined ||
          assertion.contains_all.every((value) => part.text.includes(value))) &&
        (assertion.citation === undefined ||
          part.citations?.some((citation) =>
            citationMatches(assertion.citation, citation),
          ))
      );
    case "tool_call":
      return (
        (part.type === "tool-call" || part.type === "tool_call") &&
        part.tool_name === assertion.tool_name &&
        (assertion.tool_call_id === false ||
          (typeof part.tool_call_id === "string" &&
            part.tool_call_id.length > 0)) &&
        (assertion.args === undefined ||
          valueMatches(assertion.args, part.args))
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

function findDistinctMatches(assertions, content, index = 0, used = new Set()) {
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

function formatValidationFailure(testCaseName, stageIndex, expected, content) {
  return [
    `Output validation failed for "${testCaseName}" stage ${stageIndex}.`,
    "Expected each assertion to match a distinct content part:",
    JSON.stringify(expected, null, 2),
    "Received:",
    JSON.stringify(content, null, 2),
  ].join("\n");
}

export function validateOutput({
  test_case: testCaseName,
  stage: stageIndex,
  content,
  profile: profileName,
}) {
  const testCase = getTestCase(testCaseName);
  const stage = testCase.stages[stageIndex];
  if (!stage)
    fail(`Stage ${stageIndex} not found in test case "${testCaseName}"`);
  if (!Array.isArray(content)) fail("Model output content must be an array");
  const profile = getProfile(profileName, testCaseName);
  const expected = profile?.expect ?? stage.expect;
  const aggregateAssertions = expected.content.filter(
    (assertion) => assertion.type === "text" && assertion.aggregate === true,
  );
  const partAssertions = expected.content.filter(
    (assertion) => !aggregateAssertions.includes(assertion),
  );
  const aggregateText = content
    .filter((part) => part.type === "text")
    .map((part) => part.text)
    .join("\n");

  if (
    aggregateAssertions.some(
      (assertion) =>
        !assertionMatches(assertion, { type: "text", text: aggregateText }),
    ) ||
    !findDistinctMatches(partAssertions, content)
  ) {
    fail(formatValidationFailure(testCaseName, stageIndex, expected, content));
  }

  const expectedToolCalls = expected.content.filter(
    (part) => part.type === "tool_call",
  ).length;
  const actualToolCalls = content.filter(
    (part) => part.type === "tool-call" || part.type === "tool_call",
  );
  if (actualToolCalls.length !== expectedToolCalls) {
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
