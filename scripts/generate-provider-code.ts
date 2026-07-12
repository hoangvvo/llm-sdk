import { execFile } from "node:child_process";
import { resolve } from "node:path";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);
const root = resolve(import.meta.dirname, "..");

interface GenerationTarget {
  input: string;
  language: "go" | "rust";
  output: string;
  packageName?: string;
  overrides?: string;
}

const targets: GenerationTarget[] = [
  {
    input: "schema/providers/openai.json",
    language: "go",
    output: "sdk-go/openai/openaiapi/api.go",
    packageName: "openaiapi",
    overrides: "schema/providers/openai.override.json",
  },
  {
    input: "schema/providers/openai-chat.json",
    language: "go",
    output: "sdk-go/openai/openaichatapi/api.go",
    packageName: "openaichatapi",
  },
  {
    input: "schema/providers/anthropic.json",
    language: "go",
    output: "sdk-go/anthropic/anthropicapi/api.go",
    packageName: "anthropicapi",
    overrides: "schema/providers/anthropic.override.json",
  },
  {
    input: "schema/providers/google.json",
    language: "go",
    output: "sdk-go/google/googleapi/api.go",
    packageName: "googleapi",
  },
  {
    input: "schema/providers/openai.json",
    language: "rust",
    output: "sdk-rust/src/openai/responses_api.rs",
    overrides: "schema/providers/openai.override.json",
  },
  {
    input: "schema/providers/openai-chat.json",
    language: "rust",
    output: "sdk-rust/src/openai/chat_api.rs",
  },
  {
    input: "schema/providers/anthropic.json",
    language: "rust",
    output: "sdk-rust/src/anthropic/api.rs",
    overrides: "schema/providers/anthropic.override.json",
  },
  {
    input: "schema/providers/google.json",
    language: "rust",
    output: "sdk-rust/src/google/api.rs",
  },
];

for (const target of targets) {
  const args = [
    "scripts/schema-to-code.ts",
    "--input",
    target.input,
    "--lang",
    target.language,
    "--output",
    target.output,
  ];
  if (target.packageName) {
    args.push("--package", target.packageName);
  }
  if (target.overrides) {
    args.push("--overrides", target.overrides);
  }

  const { stdout, stderr } = await execFileAsync(process.execPath, args, {
    cwd: root,
  });
  process.stdout.write(stdout);
  process.stderr.write(stderr);
  process.stdout.write(`generated ${target.output}\n`);
}
