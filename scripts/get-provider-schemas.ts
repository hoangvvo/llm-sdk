import { mkdir, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

interface ProviderSchemaConfig {
  provider: string;
  source_schema_url: string;
  schemas: ProviderRootSchema[];
}

type ProviderRootSchema =
  | string
  | {
      as: string;
      path: string | string[];
    };

const providers: ProviderSchemaConfig[] = [
  {
    provider: "openai",
    source_schema_url:
      "https://raw.githubusercontent.com/gr2m/ai-provider-monitor/refs/heads/main/cache/openai/openapi.yml",
    schemas: ["CreateResponse", "Response", "ResponseStreamEvent"],
  },
  {
    provider: "openai-chat",
    source_schema_url:
      "https://raw.githubusercontent.com/gr2m/ai-provider-monitor/refs/heads/main/cache/openai/openapi.yml",
    schemas: [
      "CreateChatCompletionRequest",
      "CreateChatCompletionResponse",
      "CreateChatCompletionStreamResponse",
    ],
  },
  {
    provider: "anthropic",
    source_schema_url:
      "https://github.com/gr2m/ai-provider-monitor/raw/refs/heads/main/cache/anthropic/openapi.json",
    schemas: ["CreateMessageParams", "Message", "MessageStreamEvent"],
  },
  {
    provider: "cohere",
    source_schema_url:
      "https://github.com/gr2m/ai-provider-monitor/raw/refs/heads/main/cache/cohere/openapi.yml",
    schemas: [
      {
        as: "ChatRequestV2",
        path: [
          "paths",
          "/v2/chat",
          "post",
          "requestBody",
          "content",
          "application/json",
          "schema",
        ],
      },
      "ChatResponseV2",
      "StreamedChatResponseV2",
    ],
  },
  {
    provider: "mistral",
    source_schema_url:
      "https://raw.githubusercontent.com/gr2m/ai-provider-monitor/refs/heads/main/cache/mistral/openapi.yml",
    schemas: [
      "ChatCompletionRequest",
      "ChatCompletionRequest",
      "CompletionEvent",
    ],
  },
];

const require = createRequire(import.meta.url);
const scriptDir = dirname(fileURLToPath(import.meta.url));
const providersOutputDir = join(scriptDir, "..", "schema", "providers");
const schemaPointerPrefixes = ["#/definitions/", "#/components/schemas/"];

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function decodeJsonPointerSegment(segment: string): string {
  return segment.replaceAll("~1", "/").replaceAll("~0", "~");
}

function extractSchemaNameFromPointer(pointer: string): string | undefined {
  for (const prefix of schemaPointerPrefixes) {
    if (!pointer.startsWith(prefix)) {
      continue;
    }

    const remainder = pointer.slice(prefix.length);
    const separatorIndex = remainder.indexOf("/");
    const encodedName =
      separatorIndex === -1 ? remainder : remainder.slice(0, separatorIndex);
    return decodeJsonPointerSegment(encodedName);
  }

  return undefined;
}

function normalizeSchemaPointer(pointer: string): string {
  for (const prefix of schemaPointerPrefixes) {
    if (pointer.startsWith(prefix)) {
      return `#/definitions/${pointer.slice(prefix.length)}`;
    }
  }

  return pointer;
}

function collectReferencedSchemaNames(
  value: unknown,
  referencedNames: Set<string>,
): void {
  if (typeof value === "string") {
    const schemaName = extractSchemaNameFromPointer(value);
    if (schemaName) {
      referencedNames.add(schemaName);
    }

    return;
  }

  if (Array.isArray(value)) {
    for (const item of value) {
      collectReferencedSchemaNames(item, referencedNames);
    }
    return;
  }

  if (!isRecord(value)) {
    return;
  }

  for (const nestedValue of Object.values(value)) {
    collectReferencedSchemaNames(nestedValue, referencedNames);
  }
}

function rewriteInternalSchemaPointers(value: unknown): unknown {
  if (typeof value === "string") {
    return normalizeSchemaPointer(value);
  }

  if (Array.isArray(value)) {
    return value.map((item) => rewriteInternalSchemaPointers(item));
  }

  if (!isRecord(value)) {
    return value;
  }

  return Object.fromEntries(
    Object.entries(value).map(([key, nestedValue]) => [
      key,
      rewriteInternalSchemaPointers(nestedValue),
    ]),
  );
}

function extractDefinitions(document: unknown): Record<string, unknown> {
  if (!isRecord(document)) {
    return {};
  }

  if (isRecord(document["definitions"])) {
    return document["definitions"];
  }

  if (
    isRecord(document["components"]) &&
    isRecord(document["components"]["schemas"])
  ) {
    return document["components"]["schemas"];
  }

  return {};
}

function tokenizePath(path: string): string[] {
  const tokens: string[] = [];
  const pathPattern =
    /\.?([A-Za-z_$][\w$]*)|\[(?:"((?:\\.|[^"\\])*)"|'((?:\\.|[^'\\])*)')\]/g;

  let lastIndex = 0;
  for (const match of path.matchAll(pathPattern)) {
    if (match.index === undefined) {
      continue;
    }

    const matchedText = match[0];
    const prefix = path.slice(lastIndex, match.index);
    if (prefix.trim() !== "") {
      throw new Error(`Invalid path syntax near "${prefix}"`);
    }

    const identifier = match[1];
    const doubleQuotedKey = match[2];
    const singleQuotedKey = match[3];
    const token = identifier ?? doubleQuotedKey ?? singleQuotedKey;
    if (token === undefined) {
      throw new Error(`Invalid path token in "${path}"`);
    }

    tokens.push(token.replaceAll('\\"', '"').replaceAll("\\'", "'"));
    lastIndex = match.index + matchedText.length;
  }

  if (path.slice(lastIndex).trim() !== "") {
    throw new Error(`Invalid trailing path syntax in "${path}"`);
  }

  return tokens;
}

function getValueAtPath(
  document: unknown,
  path: string | string[],
  provider: string,
): unknown {
  const segments = Array.isArray(path) ? path : tokenizePath(path);
  let currentValue: unknown = document;

  for (const segment of segments) {
    if (Array.isArray(currentValue)) {
      const index = Number(segment);
      if (
        !Number.isInteger(index) ||
        index < 0 ||
        index >= currentValue.length
      ) {
        throw new Error(
          `${provider}: array index "${segment}" was not found while resolving ${JSON.stringify(segments)}`,
        );
      }

      currentValue = currentValue[index];
      continue;
    }

    if (!isRecord(currentValue)) {
      throw new Error(
        `${provider}: cannot resolve path segment "${segment}" on non-object value`,
      );
    }

    if (!(segment in currentValue)) {
      throw new Error(
        `${provider}: path segment "${segment}" was not found while resolving ${JSON.stringify(segments)}`,
      );
    }

    currentValue = currentValue[segment];
  }

  return currentValue;
}

function resolveRootSchemas(
  document: unknown,
  definitions: Record<string, unknown>,
  rootSchemaConfigs: ProviderRootSchema[],
  provider: string,
): Record<string, unknown> {
  const rootSchemas: Record<string, unknown> = {};

  for (const rootSchemaConfig of rootSchemaConfigs) {
    if (typeof rootSchemaConfig === "string") {
      if (!(rootSchemaConfig in definitions)) {
        throw new Error(
          `${provider}: requested schema "${rootSchemaConfig}" was not found in source schema`,
        );
      }

      rootSchemas[rootSchemaConfig] = definitions[rootSchemaConfig];
      continue;
    }

    const rootSchema = getValueAtPath(
      document,
      rootSchemaConfig.path,
      provider,
    );
    if (!isRecord(rootSchema)) {
      throw new Error(
        `${provider}: resolved root schema "${rootSchemaConfig.as}" is not an object`,
      );
    }

    if (rootSchemaConfig.as in definitions) {
      throw new Error(
        `${provider}: inline root schema alias "${rootSchemaConfig.as}" collides with an existing schema name`,
      );
    }

    rootSchemas[rootSchemaConfig.as] = rootSchema;
  }

  return rootSchemas;
}

function trimDefinitions(
  definitions: Record<string, unknown>,
  rootSchemas: Record<string, unknown>,
  provider: string,
): Record<string, unknown> {
  const retainedSchemas = new Map<string, unknown>(Object.entries(rootSchemas));
  const queue = Object.entries(rootSchemas);

  while (queue.length > 0) {
    const currentEntry = queue.shift();
    if (!currentEntry) {
      continue;
    }

    const [currentSchemaName, currentSchema] = currentEntry;
    const referencedSchemaNames = new Set<string>();
    collectReferencedSchemaNames(currentSchema, referencedSchemaNames);

    for (const referencedSchemaName of referencedSchemaNames) {
      if (!(referencedSchemaName in definitions)) {
        throw new Error(
          `${provider}: dependency "${referencedSchemaName}" referenced by "${currentSchemaName}" was not found in source schema`,
        );
      }

      if (retainedSchemas.has(referencedSchemaName)) {
        continue;
      }

      const referencedSchema = definitions[referencedSchemaName];
      retainedSchemas.set(referencedSchemaName, referencedSchema);
      queue.push([referencedSchemaName, referencedSchema]);
    }
  }

  const trimmedDefinitions: Record<string, unknown> = {};
  for (const [schemaName, schema] of retainedSchemas) {
    trimmedDefinitions[schemaName] = rewriteInternalSchemaPointers(schema);
  }

  return trimmedDefinitions;
}

function parseMaybeJson(rawSchema: string): unknown | undefined {
  try {
    return JSON.parse(rawSchema) as unknown;
  } catch {
    return undefined;
  }
}

function parseYaml(rawSchema: string): unknown {
  try {
    const yaml = require("yaml") as { parse: (source: string) => unknown };
    return yaml.parse(rawSchema);
  } catch {
    const jsYaml = require("js-yaml") as {
      load: (source: string) => unknown;
    };
    return jsYaml.load(rawSchema);
  }
}

async function downloadSchemaDocument(
  sourceSchemaUrl: string,
  provider: string,
): Promise<unknown> {
  const response = await fetch(sourceSchemaUrl);
  if (!response.ok) {
    throw new Error(
      `${provider}: failed to download schema (${response.status} ${response.statusText})`,
    );
  }

  const rawSchema = await response.text();
  const parsedJson = parseMaybeJson(rawSchema);
  if (parsedJson !== undefined) {
    return parsedJson;
  }

  return parseYaml(rawSchema);
}

async function writeProviderSchema(
  config: ProviderSchemaConfig,
): Promise<void> {
  const sourceDocument = await downloadSchemaDocument(
    config.source_schema_url,
    config.provider,
  );
  const definitions = extractDefinitions(sourceDocument);
  const rootSchemas = resolveRootSchemas(
    sourceDocument,
    definitions,
    config.schemas,
    config.provider,
  );
  const trimmedDefinitions = trimDefinitions(
    definitions,
    rootSchemas,
    config.provider,
  );
  const outputPath = join(providersOutputDir, `${config.provider}.json`);

  await writeFile(
    outputPath,
    `${JSON.stringify({ definitions: trimmedDefinitions }, null, 2)}\n`,
    "utf8",
  );

  console.log(
    `${config.provider}: wrote ${outputPath} (${Object.keys(trimmedDefinitions).length} schemas)`,
  );
}

async function main(): Promise<void> {
  await mkdir(providersOutputDir, { recursive: true });

  for (const provider of providers) {
    await writeProviderSchema(provider);
  }
}

await main();
