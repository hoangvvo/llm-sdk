import { execFile } from "node:child_process";
import { access } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { promisify } from "node:util";

import type {
  AliasDeclaration,
  CodegenDocument,
  Declaration,
  EnumDeclaration,
  StructDeclaration,
  StructField,
  TypeExpression,
  UnionDeclaration,
} from "./core.ts";

const execFileAsync = promisify(execFile);

export function renderRustDocument(document: CodegenDocument): string {
  const attributes = [
    "#![allow(clippy::enum_variant_names)]",
    "#![allow(clippy::struct_field_names)]",
    "#![allow(clippy::doc_markdown)]",
  ];
  const imports = ["use serde::{Deserialize, Serialize};"];
  if (document.usesJsonValue) {
    imports.push("use serde_json::Value;");
  }
  if (document.usesMap) {
    imports.push("use std::collections::HashMap;");
  }

  const parts: string[] = [attributes.join("\n"), imports.join("\n")];
  for (const declaration of document.declarations) {
    parts.push(renderRustDeclaration(declaration));
  }
  return `${parts.join("\n\n")}\n`;
}

export async function formatRustOutput(outputPath: string): Promise<void> {
  const manifestPath = await findNearestCargoManifest(outputPath);
  await execFileAsync(
    "cargo",
    ["+nightly", "fmt", "--manifest-path", manifestPath],
    {
      cwd: dirname(manifestPath),
    },
  );
}

function renderRustDeclaration(declaration: Declaration): string {
  switch (declaration.kind) {
    case "alias":
      return renderRustAlias(declaration);
    case "enum":
      return renderRustEnum(declaration);
    case "struct":
      return renderRustStruct(declaration);
    case "union":
      return renderRustUnion(declaration);
  }
}

function renderRustAlias(declaration: AliasDeclaration): string {
  const comments = renderRustComment(declaration.description);
  const body = `pub type ${declaration.name} = ${renderRustType(declaration.target, false, false)};`;
  return comments ? `${comments}\n${body}` : body;
}

function renderRustEnum(declaration: EnumDeclaration): string {
  if (declaration.primitive !== "string") {
    throw new Error(
      `Rust enum ${declaration.name} uses unsupported primitive ${declaration.primitive}`,
    );
  }

  const lines: string[] = [];
  const comments = renderRustComment(declaration.description);
  if (comments) {
    lines.push(comments);
  }
  lines.push("#[derive(Serialize, Deserialize)]");
  lines.push(`pub enum ${declaration.name} {`);
  for (const variant of declaration.variants) {
    lines.push(`    #[serde(rename = ${JSON.stringify(variant.value)})]`);
    lines.push(`    ${variant.name},`);
  }
  lines.push("}");
  return lines.join("\n");
}

function renderRustStruct(declaration: StructDeclaration): string {
  const lines: string[] = [];
  const comments = renderRustComment(declaration.description);
  if (comments) {
    lines.push(comments);
  }
  lines.push("#[derive(Serialize, Deserialize)]");
  lines.push(`pub struct ${declaration.name} {`);
  for (const field of declaration.fields) {
    lines.push(...renderRustStructField(field));
  }
  lines.push("}");
  return lines.join("\n");
}

function renderRustStructField(field: StructField): string[] {
  if (field.kind === "flatten") {
    return [
      "    #[serde(flatten)]",
      `    pub ${field.name}: ${field.typeName},`,
    ];
  }

  const lines: string[] = [];
  const comments = renderRustComment(field.description, "    ");
  if (comments) {
    lines.push(comments);
  }
  const attributes: string[] = [];
  if (field.originalName !== normalizeRustFieldName(field.name)) {
    attributes.push(`rename = ${JSON.stringify(field.originalName)}`);
  }
  if (!field.required) {
    attributes.push('skip_serializing_if = "Option::is_none"');
  }
  if (attributes.length > 0) {
    lines.push(`    #[serde(${attributes.join(", ")})]`);
  }
  lines.push(
    `    pub ${field.name}: ${renderRustType(field.type, field.required, field.nullable)},`,
  );
  return lines;
}

function renderRustUnion(declaration: UnionDeclaration): string {
  const lines: string[] = [];
  const comments = renderRustComment(declaration.description);
  if (comments) {
    lines.push(comments);
  }
  lines.push("#[derive(Serialize, Deserialize)]");
  if (declaration.representation === "tagged") {
    lines.push(
      `#[serde(tag = ${JSON.stringify(
        declaration.discriminator ??
          fail(`Missing discriminator for ${declaration.name}`),
      )})]`,
    );
  } else {
    lines.push("#[serde(untagged)]");
  }
  lines.push(`pub enum ${declaration.name} {`);
  for (const variant of declaration.variants) {
    if (declaration.representation === "tagged") {
      lines.push(
        `    #[serde(rename = ${JSON.stringify(
          variant.discriminatorValue ??
            fail(`Missing variant discriminator for ${declaration.name}`),
        )})]`,
      );
    }
    lines.push(`    ${variant.name}(${variant.typeName}),`);
  }
  lines.push("}");
  return lines.join("\n");
}

function renderRustType(
  type: TypeExpression,
  required: boolean,
  nullable: boolean,
): string {
  const baseType = (() => {
    switch (type.kind) {
      case "primitive":
        return renderRustPrimitive(type.primitive);
      case "named":
        return type.name;
      case "array":
        return `Vec<${renderRustType(type.item, true, false)}>`;
      case "map":
        return `HashMap<String, ${type.value ? renderRustType(type.value, true, false) : fail("Map value type is required")}>`;
      case "json_value":
        return "Value";
      case "optional":
        return `Option<${renderRustType(type.item, true, false)}>`;
      default:
        return fail(
          `Unsupported Rust type expression: ${JSON.stringify(type)}`,
        );
    }
  })();

  if (!required || nullable) {
    return `Option<${baseType}>`;
  }
  return baseType;
}

function renderRustPrimitive(
  primitive: "string" | "boolean" | "integer" | "number",
): string {
  switch (primitive) {
    case "string":
      return "String";
    case "boolean":
      return "bool";
    case "integer":
      return "i64";
    case "number":
      return "f64";
    default:
      return fail(`Unsupported Rust primitive: ${primitive}`);
  }
}

function renderRustComment(
  description: string | undefined,
  indent = "",
): string | undefined {
  if (!description) {
    return undefined;
  }
  return description
    .split("\n")
    .map((line) => `${indent}///${line.length > 0 ? ` ${line}` : ""}`)
    .join("\n");
}

function normalizeRustFieldName(fieldName: string): string {
  return fieldName.startsWith("r#") ? fieldName.slice(2) : fieldName;
}

async function findNearestCargoManifest(startPath: string): Promise<string> {
  let currentDir = dirname(resolve(startPath));
  while (true) {
    const manifestPath = join(currentDir, "Cargo.toml");
    try {
      await access(manifestPath);
      return manifestPath;
    } catch {
      const parentDir = dirname(currentDir);
      if (parentDir === currentDir) {
        throw new Error(`Could not find Cargo.toml for ${startPath}`);
      }
      currentDir = parentDir;
    }
  }
}

function fail(message: string): never {
  throw new Error(message);
}
