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

export function renderGoDocument(
  document: CodegenDocument,
  packageName: string,
): string {
  const needsUnionHelpers = document.declarations.some(
    (declaration) => declaration.kind === "union",
  );
  const importLines: string[] = [];
  if (needsUnionHelpers) {
    importLines.push('"encoding/json"', '"errors"');
  }

  const parts: string[] = [`package ${packageName}`];
  if (importLines.length > 0) {
    parts.push(
      `import (\n${importLines.map((line) => `\t${line}`).join("\n")}\n)`,
    );
  }

  for (const declaration of document.declarations) {
    parts.push(renderGoDeclaration(declaration));
  }

  return `${parts.join("\n\n")}\n`;
}

function renderGoDeclaration(declaration: Declaration): string {
  switch (declaration.kind) {
    case "alias":
      return renderGoAlias(declaration);
    case "enum":
      return renderGoEnum(declaration);
    case "struct":
      return renderGoStruct(declaration);
    case "union":
      return renderGoUnion(declaration);
  }
}

function renderGoAlias(declaration: AliasDeclaration): string {
  const comments = renderGoComment(declaration.description);
  const body = `type ${declaration.name} ${renderGoType(declaration.target, false, false)}`;
  return comments ? `${comments}\n${body}` : body;
}

function renderGoEnum(declaration: EnumDeclaration): string {
  if (declaration.primitive !== "string") {
    throw new Error(
      `Go enum ${declaration.name} uses unsupported primitive ${declaration.primitive}`,
    );
  }

  const lines: string[] = [];
  const comments = renderGoComment(declaration.description);
  if (comments) {
    lines.push(comments);
  }
  lines.push(`type ${declaration.name} string`);
  lines.push("");
  lines.push("const (");
  for (const variant of declaration.variants) {
    lines.push(
      `\t${declaration.name}${variant.name} ${declaration.name} = ${JSON.stringify(variant.value)}`,
    );
  }
  lines.push(")");
  return lines.join("\n");
}

function renderGoStruct(declaration: StructDeclaration): string {
  const lines: string[] = [];
  const comments = renderGoComment(declaration.description);
  if (comments) {
    lines.push(comments);
  }
  lines.push(`type ${declaration.name} struct {`);
  for (const field of declaration.fields) {
    lines.push(...renderGoStructField(field));
  }
  lines.push("}");
  return lines.join("\n");
}

function renderGoStructField(field: StructField): string[] {
  if (field.kind === "flatten") {
    return [`\t${field.typeName}`];
  }

  const lines: string[] = [];
  const comments = renderGoComment(field.description, "\t");
  if (comments) {
    lines.push(comments);
  }
  const tagValue = field.required
    ? field.originalName
    : `${field.originalName},omitempty`;
  lines.push(
    `\t${field.name} ${renderGoType(field.type, field.required, field.nullable)} \`json:"${tagValue}"\``,
  );
  return lines;
}

function renderGoUnion(declaration: UnionDeclaration): string {
  if (declaration.representation === "tagged") {
    return renderTaggedGoUnion(declaration);
  }
  return renderUntaggedGoUnion(declaration);
}

function renderTaggedGoUnion(declaration: UnionDeclaration): string {
  const discriminator =
    declaration.discriminator ??
    fail(`Missing discriminator for ${declaration.name}`);
  const lines: string[] = [];
  const comments = renderGoComment(declaration.description);
  if (comments) {
    lines.push(comments);
  }
  lines.push(`type ${declaration.name} struct {`);
  for (const variant of declaration.variants) {
    lines.push(`\t${variant.name} *${variant.typeName}`);
  }
  lines.push("}");
  lines.push("");
  lines.push(`func (u *${declaration.name}) MarshalJSON() ([]byte, error) {`);
  for (const variant of declaration.variants) {
    lines.push(`\tif u.${variant.name} != nil {`);
    lines.push("\t\treturn json.Marshal(struct {");
    lines.push(
      `\t\t\t${toGoFieldName(discriminator)} string \`json:"${discriminator}"\``,
    );
    lines.push(`\t\t\t*${variant.typeName}`);
    lines.push("\t\t}{");
    lines.push(
      `\t\t\t${toGoFieldName(discriminator)}: ${JSON.stringify(variant.discriminatorValue)},`,
    );
    lines.push(`\t\t\t${variant.typeName}: u.${variant.name},`);
    lines.push("\t\t})");
    lines.push("\t}");
  }
  lines.push(
    `\treturn nil, errors.New("invalid ${declaration.name}: all variants are nil")`,
  );
  lines.push("}");
  lines.push("");
  lines.push(
    `func (u *${declaration.name}) UnmarshalJSON(data []byte) error {`,
  );
  lines.push("\tvar raw map[string]json.RawMessage");
  lines.push("\tif err := json.Unmarshal(data, &raw); err != nil {");
  lines.push("\t\treturn err");
  lines.push("\t}");
  lines.push(
    `\traw${toGoFieldName(discriminator)}, ok := raw[${JSON.stringify(discriminator)}]`,
  );
  lines.push("\tif !ok {");
  lines.push(
    `\t\treturn errors.New("missing ${discriminator} field in ${declaration.name}")`,
  );
  lines.push("\t}");
  lines.push("\tvar discriminator string");
  lines.push(
    `\tif err := json.Unmarshal(raw${toGoFieldName(discriminator)}, &discriminator); err != nil {`,
  );
  lines.push("\t\treturn err");
  lines.push("\t}");
  lines.push(`\t*u = ${declaration.name}{}`);
  lines.push("\tswitch discriminator {");
  for (const variant of declaration.variants) {
    lines.push(`\tcase ${JSON.stringify(variant.discriminatorValue)}:`);
    lines.push(`\t\tvar value ${variant.typeName}`);
    lines.push("\t\tif err := json.Unmarshal(data, &value); err != nil {");
    lines.push("\t\t\treturn err");
    lines.push("\t\t}");
    lines.push(`\t\tu.${variant.name} = &value`);
  }
  lines.push("\tdefault:");
  lines.push(
    `\t\treturn errors.New("invalid ${discriminator} field in ${declaration.name}")`,
  );
  lines.push("\t}");
  lines.push("\treturn nil");
  lines.push("}");
  return lines.join("\n");
}

function renderUntaggedGoUnion(declaration: UnionDeclaration): string {
  const lines: string[] = [];
  const comments = renderGoComment(declaration.description);
  if (comments) {
    lines.push(comments);
  }
  lines.push(`type ${declaration.name} struct {`);
  for (const variant of declaration.variants) {
    lines.push(`\t${variant.name} *${variant.typeName}`);
  }
  lines.push("}");
  lines.push("");
  lines.push(`func (u *${declaration.name}) MarshalJSON() ([]byte, error) {`);
  lines.push("\tif u == nil {");
  lines.push('\t\treturn []byte("null"), nil');
  lines.push("\t}");
  for (const variant of declaration.variants) {
    lines.push(`\tif u.${variant.name} != nil {`);
    lines.push(`\t\treturn json.Marshal(u.${variant.name})`);
    lines.push("\t}");
  }
  lines.push(
    `\treturn nil, errors.New("invalid ${declaration.name}: all variants are nil")`,
  );
  lines.push("}");
  lines.push("");
  lines.push(
    `func (u *${declaration.name}) UnmarshalJSON(data []byte) error {`,
  );
  lines.push("\tvar raw interface{}");
  lines.push("\tif err := json.Unmarshal(data, &raw); err != nil {");
  lines.push("\t\treturn err");
  lines.push("\t}");
  lines.push(`\t*u = ${declaration.name}{}`);
  const objectVariants = declaration.variants.filter(
    (variant) => variant.match.kind === "object",
  );
  const needsObjectValue = objectVariants.some((variant) => {
    const match = variant.match;
    return (
      match.kind === "object" &&
      (Boolean(match.nestedTaggedUnion) ||
        Boolean(match.discriminator) ||
        match.requiredProperties.length > 0)
    );
  });
  lines.push(
    needsObjectValue
      ? "\tswitch value := raw.(type) {"
      : "\tswitch raw.(type) {",
  );

  const stringVariants = declaration.variants.filter(
    (variant) => variant.match.kind === "string",
  );
  if (stringVariants.length > 0) {
    lines.push("\tcase string:");
    lines.push(
      ...renderGoUntaggedCaseBody(stringVariants, declaration.name, "string"),
    );
  }
  const numberVariants = declaration.variants.filter(
    (variant) => variant.match.kind === "number",
  );
  if (numberVariants.length > 0) {
    lines.push("\tcase float64:");
    lines.push(
      ...renderGoUntaggedCaseBody(numberVariants, declaration.name, "number"),
    );
  }
  const booleanVariants = declaration.variants.filter(
    (variant) => variant.match.kind === "boolean",
  );
  if (booleanVariants.length > 0) {
    lines.push("\tcase bool:");
    lines.push(
      ...renderGoUntaggedCaseBody(booleanVariants, declaration.name, "boolean"),
    );
  }
  const arrayVariants = declaration.variants.filter(
    (variant) => variant.match.kind === "array",
  );
  if (arrayVariants.length > 0) {
    lines.push("\tcase []interface{}:");
    lines.push(
      ...renderGoUntaggedCaseBody(arrayVariants, declaration.name, "array"),
    );
  }
  if (objectVariants.length > 0) {
    lines.push("\tcase map[string]interface{}:");
    lines.push(
      ...renderGoObjectUntaggedCaseBody(objectVariants, declaration.name),
    );
  }
  lines.push("\t}");
  lines.push(`\treturn errors.New("invalid ${declaration.name}")`);
  lines.push("}");
  return lines.join("\n");
}

function renderGoUntaggedCaseBody(
  variants: UnionDeclaration["variants"],
  unionName: string,
  _kind: string,
): string[] {
  if (variants.length !== 1) {
    throw new Error(
      `Untagged Go union ${unionName} (${_kind}) has multiple indistinguishable variants`,
    );
  }
  const variant = variants[0] ?? fail(`Missing variant for ${unionName}`);
  const lines: string[] = [];
  lines.push(`\t\tvar v ${variant.typeName}`);
  lines.push("\t\tif err := json.Unmarshal(data, &v); err != nil {");
  lines.push("\t\t\treturn err");
  lines.push("\t\t}");
  lines.push(`\t\tu.${variant.name} = &v`);
  lines.push("\t\treturn nil");
  return lines;
}

function renderGoObjectUntaggedCaseBody(
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
  for (const variant of rankedVariants) {
    const match = variant.match;
    if (match.kind !== "object") {
      continue;
    }
    const conditions: string[] = [];
    let hasNestedTaggedUnion = false;
    if (match.nestedTaggedUnion) {
      hasNestedTaggedUnion = true;
      const field = toGoFieldName(match.nestedTaggedUnion.property);
      lines.push(
        `\t\tif raw${field}, ok := value[${JSON.stringify(match.nestedTaggedUnion.property)}]; ok {`,
      );
      lines.push(`\t\t\tif discriminator, ok := raw${field}.(string); ok {`);
      lines.push(`\t\t\t\tswitch discriminator {`);
      for (const allowedValue of match.nestedTaggedUnion.values) {
        lines.push(`\t\t\t\tcase ${JSON.stringify(allowedValue)}:`);
      }
      lines.push(`\t\t\t\t\tvar v ${variant.typeName}`);
      lines.push("\t\t\t\t\tif err := json.Unmarshal(data, &v); err != nil {");
      lines.push("\t\t\t\t\t\treturn err");
      lines.push("\t\t\t\t\t}");
      lines.push(`\t\t\t\t\tu.${variant.name} = &v`);
      lines.push("\t\t\t\t\treturn nil");
      lines.push("\t\t\t\t}");
      lines.push("\t\t\t}");
      lines.push("\t\t}");
    }
    if (match.discriminator) {
      conditions.push(
        `raw${toGoFieldName(match.discriminator.property)}, ok := value[${JSON.stringify(match.discriminator.property)}]; !ok || raw${toGoFieldName(match.discriminator.property)} == ${JSON.stringify(match.discriminator.value)}`,
      );
    }
    if (match.requiredProperties.length > 0) {
      for (const property of match.requiredProperties) {
        conditions.push(`value[${JSON.stringify(property)}] != nil`);
      }
    }
    if (conditions.length > 0) {
      lines.push(`\t\tif ${conditions.join(" && ")} {`);
      lines.push(`\t\t\tvar v ${variant.typeName}`);
      lines.push("\t\t\tif err := json.Unmarshal(data, &v); err != nil {");
      lines.push("\t\t\t\treturn err");
      lines.push("\t\t\t}");
      lines.push(`\t\t\tu.${variant.name} = &v`);
      lines.push("\t\t\treturn nil");
      lines.push("\t\t}");
      continue;
    }
    if (hasNestedTaggedUnion) {
      continue;
    }
    if (variant !== rankedVariants[rankedVariants.length - 1]) {
      throw new Error(
        `Untagged Go union ${unionName} has an ambiguous object variant ordering`,
      );
    }
    lines.push(`\t\tvar v ${variant.typeName}`);
    lines.push("\t\tif err := json.Unmarshal(data, &v); err != nil {");
    lines.push("\t\t\treturn err");
    lines.push("\t\t}");
    lines.push(`\t\tu.${variant.name} = &v`);
    lines.push("\t\treturn nil");
    hasFallbackError = false;
  }
  if (hasFallbackError) {
    lines.push(`\t\treturn errors.New("invalid ${unionName}")`);
  }
  return lines;
}

function renderGoType(
  type: TypeExpression,
  required: boolean,
  nullable: boolean,
): string {
  switch (type.kind) {
    case "primitive":
      return wrapOptionalGoType(
        renderGoPrimitive(type.primitive),
        required,
        nullable,
        type,
      );
    case "named":
      return wrapOptionalGoType(type.name, required, nullable, type);
    case "array":
      return `[]${renderGoType(type.item, true, false)}`;
    case "map":
      return `map[string]${type.value ? renderGoType(type.value, true, false) : fail("Map value type is required")}`;
    case "json_value":
      return "any";
    case "optional":
      return renderGoType(type.item, false, true);
    default:
      return fail(`Unsupported Go type expression: ${JSON.stringify(type)}`);
  }
}

function wrapOptionalGoType(
  renderedType: string,
  required: boolean,
  nullable: boolean,
  originalType: TypeExpression,
): string {
  if (required && !nullable) {
    return renderedType;
  }
  if (originalType.kind === "array" || originalType.kind === "map") {
    return renderedType;
  }
  return `*${renderedType}`;
}

function renderGoPrimitive(
  primitive: "string" | "boolean" | "integer" | "number",
): string {
  switch (primitive) {
    case "string":
      return "string";
    case "boolean":
      return "bool";
    case "integer":
      return "int";
    case "number":
      return "float64";
    default:
      return fail(`Unsupported Go primitive: ${primitive}`);
  }
}

function renderGoComment(
  description: string | undefined,
  indent = "",
): string | undefined {
  if (!description) {
    return undefined;
  }
  return description
    .split("\n")
    .map((line) => `${indent}//${line.length > 0 ? ` ${line}` : ""}`)
    .join("\n");
}

function toGoFieldName(value: string): string {
  const words = value
    .replaceAll(/[^A-Za-z0-9]+/g, " ")
    .trim()
    .split(/\s+/)
    .filter(Boolean);
  const identifier = words
    .flatMap(
      (word) => word.match(/[A-Z]+(?=[A-Z][a-z]|\d|$)|[A-Z]?[a-z]+|\d+/g) ?? [],
    )
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join("");
  return /^[0-9]/.test(identifier) ? `N${identifier}` : identifier;
}

function fail(message: string): never {
  throw new Error(message);
}
