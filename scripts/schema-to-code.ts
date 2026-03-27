import { mkdir, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";

import {
  buildCodegenDocument,
  loadSchemaDocument,
  type TargetLanguage,
} from "./schema-to-code/core.ts";
import { formatGoOutput, renderGoDocument } from "./schema-to-code/go.ts";
import { formatRustOutput, renderRustDocument } from "./schema-to-code/rust.ts";

interface CliOptions {
  input: string;
  language: TargetLanguage;
  output: string | undefined;
  packageName: string | undefined;
}

async function main(): Promise<void> {
  const options = parseArgs(process.argv.slice(2));
  const definitions = await loadSchemaDocument(resolve(options.input));
  const document = buildCodegenDocument(definitions, options.language);

  const rendered =
    options.language === "go"
      ? renderGoDocument(
          document,
          options.packageName ?? fail("Missing --package for Go output"),
        )
      : renderRustDocument(document);

  if (options.output) {
    const outputPath = resolve(options.output);
    await mkdir(dirname(outputPath), { recursive: true });
    await writeFile(outputPath, rendered, "utf8");
    if (options.language === "go") {
      await formatGoOutput(outputPath);
    } else {
      await formatRustOutput(outputPath);
    }
    return;
  }

  process.stdout.write(rendered);
}

function parseArgs(args: string[]): CliOptions {
  const values = new Map<string, string>();

  for (let index = 0; index < args.length; index += 1) {
    const arg = args[index];
    if (!arg) {
      throw new Error("Unexpected empty argument");
    }
    if (!arg.startsWith("--")) {
      throw new Error(`Unexpected positional argument: ${arg}`);
    }
    const value = args[index + 1];
    if (!value || value.startsWith("--")) {
      throw new Error(`Expected a value after ${arg}`);
    }
    values.set(arg.slice(2), value);
    index += 1;
  }

  const input = values.get("input");
  if (!input) {
    throw new Error("Missing required --input");
  }

  const language = values.get("lang");
  if (language !== "go" && language !== "rust") {
    throw new Error('Missing or invalid --lang. Expected "go" or "rust".');
  }

  return {
    input,
    language,
    output: values.get("output"),
    packageName: values.get("package"),
  };
}

function fail(message: string): never {
  throw new Error(message);
}

main().catch((error: unknown) => {
  const message = error instanceof Error ? error.message : String(error);
  process.stderr.write(`${message}\n`);
  process.exitCode = 1;
});
