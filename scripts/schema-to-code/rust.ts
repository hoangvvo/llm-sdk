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
  const needsValueImport =
    document.usesJsonValue ||
    document.declarations.some(
      (declaration) =>
        declaration.kind === "union" &&
        (declaration.representation === "untagged" ||
          declaration.untaggedDeserializeStrategy === "placeholder"),
    );
  const attributes = [
    "#![allow(dead_code)]",
    "#![allow(clippy::enum_variant_names)]",
    "#![allow(clippy::struct_field_names)]",
    "#![allow(clippy::doc_markdown)]",
    "#![allow(clippy::too_many_lines)]",
  ];
  const imports = ["use serde::{Deserialize, Serialize};"];
  if (needsValueImport) {
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
  lines.push("#[non_exhaustive]");
  lines.push(`pub enum ${declaration.name} {`);
  for (const variant of declaration.variants) {
    lines.push(`    #[serde(rename = ${JSON.stringify(variant.value)})]`);
    lines.push(`    ${variant.name},`);
  }
  lines.push("    #[serde(other)]");
  lines.push("    #[serde(skip_serializing)]");
  lines.push("    Unknown,");
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
  const usesPlaceholderDeserialize =
    declaration.representation === "untagged" &&
    declaration.untaggedDeserializeStrategy === "placeholder";
  if (!usesPlaceholderDeserialize) {
    lines.push("#[derive(Serialize, Deserialize)]");
  } else {
    lines.push("#[derive(Serialize)]");
  }
  lines.push("#[non_exhaustive]");
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
  if (declaration.representation === "tagged") {
    lines.push("    #[serde(other)]");
    lines.push("    #[serde(skip_serializing)]");
    lines.push("    Unknown,");
  } else {
    lines.push("    #[allow(dead_code)]");
    lines.push("    #[serde(skip_serializing)]");
    lines.push("    Unknown(Value),");
  }
  lines.push("}");
  if (usesPlaceholderDeserialize) {
    lines.push(...renderRustUntaggedDeserialize(declaration));
  }
  return lines.join("\n");
}

function renderRustUntaggedDeserialize(
  declaration: UnionDeclaration,
): string[] {
  const lines: string[] = [];
  lines.push("");
  lines.push(`impl<'de> Deserialize<'de> for ${declaration.name} {`);
  lines.push(
    "    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>",
  );
  lines.push("    where");
  lines.push("        D: serde::Deserializer<'de>,");
  lines.push("    {");
  lines.push("        let value = Value::deserialize(deserializer)?;");
  lines.push("        match &value {");

  const stringVariants = declaration.variants.filter(
    (variant) => variant.match.kind === "string",
  );
  if (stringVariants.length > 0) {
    lines.push("            Value::String(_) => {");
    lines.push(
      ...renderRustUntaggedPrimitiveCaseBody(
        stringVariants,
        declaration.name,
        "string",
      ),
    );
    lines.push("            }");
  }

  const numberVariants = declaration.variants.filter(
    (variant) => variant.match.kind === "number",
  );
  if (numberVariants.length > 0) {
    lines.push("            Value::Number(_) => {");
    lines.push(
      ...renderRustUntaggedPrimitiveCaseBody(
        numberVariants,
        declaration.name,
        "number",
      ),
    );
    lines.push("            }");
  }

  const booleanVariants = declaration.variants.filter(
    (variant) => variant.match.kind === "boolean",
  );
  if (booleanVariants.length > 0) {
    lines.push("            Value::Bool(_) => {");
    lines.push(
      ...renderRustUntaggedPrimitiveCaseBody(
        booleanVariants,
        declaration.name,
        "boolean",
      ),
    );
    lines.push("            }");
  }

  const arrayVariants = declaration.variants.filter(
    (variant) => variant.match.kind === "array",
  );
  if (arrayVariants.length > 0) {
    lines.push("            Value::Array(_) => {");
    lines.push(
      ...renderRustUntaggedPrimitiveCaseBody(
        arrayVariants,
        declaration.name,
        "array",
      ),
    );
    lines.push("            }");
  }

  const objectVariants = declaration.variants.filter(
    (variant) => variant.match.kind === "object",
  );
  if (objectVariants.length > 0) {
    lines.push("            Value::Object(object) => {");
    lines.push(
      ...renderRustObjectUntaggedCaseBody(objectVariants, declaration.name),
    );
    lines.push("            }");
  }

  lines.push(`            _ => Ok(Self::Unknown(value)),`);
  lines.push("        }");
  lines.push("    }");
  lines.push("}");
  return lines;
}

function renderRustUntaggedPrimitiveCaseBody(
  variants: UnionDeclaration["variants"],
  unionName: string,
  kind: string,
): string[] {
  if (variants.length !== 1) {
    throw new Error(
      `Untagged Rust union ${unionName} (${kind}) has multiple indistinguishable variants`,
    );
  }

  const variant = variants[0] ?? fail(`Missing variant for ${unionName}`);
  return [
    `                serde_json::from_value(value.clone())`,
    `                    .map(Self::${variant.name})`,
    "                    .or(Ok(Self::Unknown(value.clone())))",
  ];
}

function renderRustObjectUntaggedCaseBody(
  variants: UnionDeclaration["variants"],
  unionName: string,
): string[] {
  const rankedVariants = [...variants].sort((left, right) => {
    const leftMatch =
      left.match.kind === "object"
        ? left.match
        : fail(`Expected object match for ${unionName}`);
    const rightMatch =
      right.match.kind === "object"
        ? right.match
        : fail(`Expected object match for ${unionName}`);
    const leftScore =
      (leftMatch.nestedTaggedUnion ? 100 : 0) +
      (leftMatch.discriminator ? 10 : 0) +
      leftMatch.requiredProperties.length;
    const rightScore =
      (rightMatch.nestedTaggedUnion ? 100 : 0) +
      (rightMatch.discriminator ? 10 : 0) +
      rightMatch.requiredProperties.length;
    return rightScore - leftScore;
  });

  const lines: string[] = [];
  let hasFallbackError = true;

  rankedVariants.forEach((variant, index) => {
    if (variant.match.kind !== "object") {
      return;
    }

    const match = variant.match;
    let hasNestedTaggedUnion = false;

    if (match.nestedTaggedUnion) {
      hasNestedTaggedUnion = true;
      lines.push(
        `                if let Some(discriminator) = object.get(${JSON.stringify(
          match.nestedTaggedUnion.property,
        )}).and_then(Value::as_str) {`,
      );
      lines.push("                    match discriminator {");
      match.nestedTaggedUnion.values.forEach((allowedValue) => {
        lines.push(
          `                        ${JSON.stringify(allowedValue)} => {`,
        );
        lines.push(
          `                            return serde_json::from_value(value.clone())`,
        );
        lines.push(
          `                                .map(Self::${variant.name})`,
        );
        lines.push(
          "                                .or(Ok(Self::Unknown(value.clone())));",
        );
        lines.push("                        }");
      });
      lines.push("                        _ => {}");
      lines.push("                    }");
      lines.push("                }");
    }

    const conditions: string[] = [];
    if (match.discriminator) {
      conditions.push(
        `object.get(${JSON.stringify(
          match.discriminator.property,
        )}).is_none_or(|raw| raw.as_str() == Some(${JSON.stringify(
          match.discriminator.value,
        )}))`,
      );
    }
    match.requiredProperties.forEach((property) => {
      conditions.push(`object.get(${JSON.stringify(property)}).is_some()`);
    });

    if (conditions.length > 0) {
      lines.push(`                if ${conditions.join(" && ")} {`);
      lines.push(
        `                    return serde_json::from_value(value.clone())`,
      );
      lines.push(`                        .map(Self::${variant.name})`);
      lines.push(
        "                        .or(Ok(Self::Unknown(value.clone())));",
      );
      lines.push("                }");
      return;
    }

    if (hasNestedTaggedUnion) {
      return;
    }

    if (index !== rankedVariants.length - 1) {
      throw new Error(
        `Untagged Rust union ${unionName} has an ambiguous object variant ordering`,
      );
    }

    lines.push(`                return serde_json::from_value(value.clone())`);
    lines.push(`                    .map(Self::${variant.name})`);
    lines.push("                    .or(Ok(Self::Unknown(value.clone())));");
    hasFallbackError = false;
  });

  if (hasFallbackError) {
    lines.push(`                Ok(Self::Unknown(value.clone()))`);
  }

  return lines;
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
