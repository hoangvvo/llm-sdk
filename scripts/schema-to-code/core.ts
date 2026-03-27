import type { JSONSchema7, JSONSchema7Definition } from "json-schema";
import { readFile } from "node:fs/promises";

export type TargetLanguage = "go" | "rust";

type JSONSchema7Record = Record<string, JSONSchema7Definition>;
type PrimitiveKind = "string" | "boolean" | "integer" | "number";

export interface CodegenDocument {
  declarations: Declaration[];
  usesMap: boolean;
  usesJsonValue: boolean;
}

export type Declaration =
  | AliasDeclaration
  | EnumDeclaration
  | StructDeclaration
  | UnionDeclaration;

export interface AliasDeclaration {
  kind: "alias";
  name: string;
  description: string | undefined;
  target: TypeExpression;
}

export interface EnumDeclaration {
  kind: "enum";
  name: string;
  description: string | undefined;
  primitive: PrimitiveKind;
  variants: EnumVariant[];
}

export interface EnumVariant {
  name: string;
  value: string;
}

export interface StructDeclaration {
  kind: "struct";
  name: string;
  description: string | undefined;
  fields: StructField[];
}

export type StructField = PropertyField | FlattenField;

export interface PropertyField {
  kind: "property";
  name: string;
  originalName: string;
  description: string | undefined;
  type: TypeExpression;
  required: boolean;
  nullable: boolean;
}

export interface FlattenField {
  kind: "flatten";
  name: string;
  typeName: string;
}

export interface UnionDeclaration {
  kind: "union";
  representation: "tagged" | "untagged";
  name: string;
  description: string | undefined;
  discriminator: string | undefined;
  variants: UnionVariant[];
}

export interface UnionVariant {
  name: string;
  typeName: string;
  discriminatorValue: string | undefined;
  match: UnionVariantMatch;
}

export type UnionVariantMatch =
  | { kind: "string" }
  | { kind: "number" }
  | { kind: "boolean" }
  | { kind: "array" }
  | {
      kind: "object";
      requiredProperties: string[];
      discriminator?: ObjectDiscriminatorMatch;
      nestedTaggedUnion?: NestedTaggedUnionMatch;
    };

export interface ObjectDiscriminatorMatch {
  property: string;
  value: string;
}

export interface NestedTaggedUnionMatch {
  property: string;
  values: string[];
}

export type TypeExpression =
  | { kind: "primitive"; primitive: PrimitiveKind }
  | { kind: "named"; name: string }
  | { kind: "array"; item: TypeExpression }
  | { kind: "map"; value: TypeExpression | undefined }
  | { kind: "json_value" }
  | { kind: "optional"; item: TypeExpression };

interface UnionAnalysis {
  discriminator: string;
  variants: AnalyzedVariant[];
}

interface AnalyzedVariant {
  typeName: string;
  discriminatorValue: string;
  refName?: string;
  inlineSchema?: JSONSchema7;
}

interface VariantUsage {
  propertyName: string;
  value: string;
}

interface UntaggedVariantAnalysis {
  variant: UnionVariant;
  declarations: Declaration[];
}

interface LoweredType {
  type: TypeExpression;
  declarations: Declaration[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isSchemaObject(value: unknown): value is JSONSchema7 {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function assertSchemaObject(
  value: JSONSchema7Definition | undefined,
  path: string,
): JSONSchema7 {
  if (!value) {
    throw new Error(`Missing schema at ${path}`);
  }
  if (!isSchemaObject(value)) {
    throw new Error(`Boolean JSON Schema is not supported at ${path}`);
  }
  return value;
}

function formatPath(parts: string[]): string {
  return parts.join("");
}

function splitIntoWords(value: string): string[] {
  const normalized = value.replaceAll(/[^A-Za-z0-9]+/g, " ").trim();
  if (normalized === "") {
    return [];
  }

  return normalized
    .split(/\s+/)
    .flatMap((token) => {
      const segments = token.match(
        /[A-Z]+(?=[A-Z][a-z]|\d|$)|[A-Z]?[a-z]+|\d+/g,
      );
      return segments ?? [];
    })
    .filter((token) => token.length > 0);
}

function toCamelCase(value: string): string {
  const words = splitIntoWords(value);
  if (words.length === 0) {
    throw new Error(`Cannot derive CamelCase name from "${value}"`);
  }

  const identifier = words
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join("");
  return /^[0-9]/.test(identifier) ? `N${identifier}` : identifier;
}

function toSnakeCase(value: string): string {
  const words = splitIntoWords(value);
  if (words.length === 0) {
    throw new Error(`Cannot derive snake_case name from "${value}"`);
  }
  const identifier = words.map((word) => word.toLowerCase()).join("_");
  return /^[0-9]/.test(identifier) ? `field_${identifier}` : identifier;
}

function unwrapNullableSchema(
  schema: JSONSchema7,
  path: string,
): { schema: JSONSchema7; nullable: boolean } {
  if (Array.isArray(schema.type)) {
    if (schema.type.length !== 2 || !schema.type.includes("null")) {
      throw new Error(`Unsupported nullable type union at ${path}`);
    }
    const nonNullType = schema.type.find((entry) => entry !== "null");
    if (
      nonNullType !== "string" &&
      nonNullType !== "boolean" &&
      nonNullType !== "integer" &&
      nonNullType !== "number" &&
      nonNullType !== "object" &&
      nonNullType !== "array"
    ) {
      throw new Error(`Unsupported nullable type union at ${path}`);
    }
    return {
      nullable: true,
      schema: {
        ...schema,
        type: nonNullType,
      },
    };
  }

  if (!schema.anyOf) {
    return { schema, nullable: false };
  }

  if (schema.anyOf.length !== 2) {
    return { schema, nullable: false };
  }

  const schemaOptions = schema.anyOf.map((item, index) =>
    assertSchemaObject(item, `${path}.anyOf[${index}]`),
  );
  const nonNullSchemas = schemaOptions.filter((item) => item.type !== "null");
  if (nonNullSchemas.length !== 1) {
    return { schema, nullable: false };
  }
  const nullSchemas = schemaOptions.filter((item) => item.type === "null");
  if (nullSchemas.length !== 1) {
    return { schema, nullable: false };
  }
  return {
    nullable: true,
    schema: mergeSchemaMetadata(
      nonNullSchemas[0] ?? fail(`Missing non-null schema at ${path}`),
      schema,
    ),
  };
}

function mergeSchemaMetadata(
  schema: JSONSchema7,
  wrapper: JSONSchema7,
): JSONSchema7 {
  return {
    ...schema,
    description: wrapper.description ?? schema.description,
    title: wrapper.title ?? schema.title,
  };
}

function isStringEnumSchema(schema: JSONSchema7): schema is JSONSchema7 & {
  enum: string[];
} {
  return (
    Array.isArray(schema.enum) &&
    schema.enum.length > 0 &&
    schema.enum.every((entry) => typeof entry === "string")
  );
}

function isLiteralStringDiscriminatorSchema(
  schema: JSONSchema7 | undefined,
): string | undefined {
  if (!schema) {
    return undefined;
  }
  const unwrapped = unwrapNullableSchema(schema, "<discriminator>").schema;
  if (typeof unwrapped.const === "string") {
    return unwrapped.const;
  }
  if (isStringEnumSchema(unwrapped) && unwrapped.enum.length === 1) {
    return unwrapped.enum[0];
  }
  return undefined;
}

function getConstStringSchemaValue(schema: JSONSchema7): string | undefined {
  const unwrapped = unwrapNullableSchema(schema, "<const-string>").schema;
  if (typeof unwrapped.const === "string") {
    return unwrapped.const;
  }
  if (isStringEnumSchema(unwrapped) && unwrapped.enum.length === 1) {
    return unwrapped.enum[0];
  }
  return undefined;
}

function getSchemaRef(schema: JSONSchema7): string | undefined {
  return typeof schema.$ref === "string" ? schema.$ref : undefined;
}

function getObjectRequiredProperties(schema: JSONSchema7): string[] {
  return Array.isArray(schema.required)
    ? schema.required.filter((item): item is string => typeof item === "string")
    : [];
}

function getNamedVariantName(typeName: string, value?: string): string {
  return value ? toCamelCase(value) : typeName;
}

function getPrimitiveAliasSuffix(kind: PrimitiveKind): string {
  switch (kind) {
    case "string":
      return "String";
    case "boolean":
      return "Boolean";
    case "integer":
      return "Integer";
    case "number":
      return "Number";
  }
}

function isStringEnumLike(schema: JSONSchema7): boolean {
  if (schema.type === "string") {
    return true;
  }
  return getConstStringSchemaValue(schema) !== undefined;
}

function isMetadataOnlySchema(schema: JSONSchema7): boolean {
  return Object.keys(schema).every(
    (key) => METADATA_ONLY_KEYS.has(key) || key.startsWith("x-"),
  );
}

function isStringLikeAnyOf(schema: JSONSchema7): boolean {
  if (!schema.anyOf || schema.anyOf.length < 2) {
    return false;
  }
  return schema.anyOf.every((option, index) => {
    const optionSchema = assertSchemaObject(option, `<string-anyOf[${index}]>`);
    return isStringEnumLike(optionSchema);
  });
}

function normalizePrimitiveSchema(
  schema: JSONSchema7,
): PrimitiveKind | undefined {
  if (
    schema.type === "string" ||
    schema.type === "boolean" ||
    schema.type === "integer" ||
    schema.type === "number"
  ) {
    return schema.type;
  }
  if (typeof schema.const === "string" || isStringEnumSchema(schema)) {
    return "string";
  }
  return undefined;
}

function propertyDescription(schema: JSONSchema7): string | undefined {
  const nullable = unwrapNullableSchema(schema, "<description>");
  return nullable.schema.description ?? schema.description;
}

export async function loadSchemaDocument(
  filePath: string,
): Promise<Record<string, JSONSchema7Definition>> {
  const raw = await readFile(filePath, "utf8");
  const parsed = JSON.parse(raw) as unknown;
  if (!isRecord(parsed)) {
    throw new Error("Schema root must be an object");
  }
  if (!isRecord(parsed["definitions"])) {
    throw new Error('Schema root must define a "definitions" object');
  }
  const definitions = parsed["definitions"] as JSONSchema7Record;
  for (const [definitionName, definition] of Object.entries(definitions)) {
    definitions[definitionName] = rewriteRecursiveRefs(
      definition,
      definitionName,
    ) as JSONSchema7Definition;
  }
  return definitions;
}

function rewriteRecursiveRefs(value: unknown, definitionName: string): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => rewriteRecursiveRefs(item, definitionName));
  }
  if (!isRecord(value)) {
    return value;
  }

  const recursiveRef = value["$recursiveRef"];
  const next: Record<string, unknown> = Object.fromEntries(
    Object.entries(value).map(([key, nestedValue]) => [
      key,
      rewriteRecursiveRefs(nestedValue, definitionName),
    ]),
  );
  if (recursiveRef === "#") {
    delete next["$recursiveRef"];
    next["$ref"] = `#/definitions/${definitionName}`;
  }
  return next as JSONSchema7;
}

export function buildCodegenDocument(
  definitions: Record<string, JSONSchema7Definition>,
  language: TargetLanguage,
): CodegenDocument {
  const builder = new SchemaDocumentBuilder(definitions, language);
  return builder.build();
}

class SchemaDocumentBuilder {
  private readonly definitions: Record<string, JSONSchema7Definition>;
  private readonly language: TargetLanguage;
  private readonly reservedNames = new Set<string>();
  // When a named definition is referenced as a tagged-union variant via $ref,
  // we record the discriminator metadata here so the lowered struct can omit
  // that property and let the generated union wrapper inject it instead.
  //
  // This is a shared lowering behavior. It works well for definitions that are
  // only ever used as union variants, but it is a known sharp edge when the
  // same definition is also reused later as a plain nested object.
  private readonly variantUsageByDefinition = new Map<string, VariantUsage>();
  private readonly referencedOutsideTaggedUnion = new Set<string>();
  private usesMap = false;
  private usesJsonValue = false;

  constructor(
    definitions: Record<string, JSONSchema7Definition>,
    language: TargetLanguage,
  ) {
    this.definitions = definitions;
    this.language = language;
  }

  build(): CodegenDocument {
    for (const [definitionName] of Object.entries(this.definitions)) {
      this.reservedNames.add(toCamelCase(definitionName));
    }

    for (const [definitionName, definition] of Object.entries(
      this.definitions,
    )) {
      const schema = assertSchemaObject(
        definition,
        `#/definitions/${definitionName}`,
      );
      this.scanForUnionVariantUsage(
        schema,
        [`#/definitions/${definitionName}`],
        toCamelCase(definitionName),
      );
    }

    const declarations: Declaration[] = [];
    for (const [definitionName, definition] of Object.entries(
      this.definitions,
    )) {
      const schema = assertSchemaObject(
        definition,
        `#/definitions/${definitionName}`,
      );
      const typeName = toCamelCase(definitionName);
      declarations.push(
        ...this.lowerNamedDeclaration(typeName, schema, [
          `#/definitions/${definitionName}`,
        ]),
      );
    }

    return {
      declarations,
      usesMap: this.usesMap,
      usesJsonValue: this.usesJsonValue,
    };
  }

  private scanForUnionVariantUsage(
    schema: JSONSchema7,
    path: string[],
    owningTypeName: string,
    suppressDirectRefUsage = false,
  ): void {
    const unwrapped = unwrapNullableSchema(schema, formatPath(path)).schema;
    const ref = getSchemaRef(unwrapped);
    if (ref) {
      if (!suppressDirectRefUsage) {
        this.referencedOutsideTaggedUnion.add(
          toCamelCase(this.resolveRefName(ref, formatPath(path))),
        );
      }
      return;
    }

    let analysis: UnionAnalysis | undefined;
    if (unwrapped.oneOf || unwrapped.anyOf) {
      analysis = this.tryAnalyzeTaggedUnion(unwrapped, path, owningTypeName);
      if (analysis) {
        for (const variant of analysis.variants) {
          if (!variant.refName) {
            continue;
          }
          const existing = this.variantUsageByDefinition.get(variant.refName);
          if (
            existing &&
            (existing.propertyName !== analysis.discriminator ||
              existing.value !== variant.discriminatorValue)
          ) {
            throw new Error(
              `Definition ${variant.refName} is used with conflicting discriminator metadata`,
            );
          }
          this.variantUsageByDefinition.set(variant.refName, {
            propertyName: analysis.discriminator,
            value: variant.discriminatorValue,
          });
        }
      }
    }

    for (const key of ["allOf", "oneOf", "anyOf"] as const) {
      const value = unwrapped[key];
      if (!Array.isArray(value)) {
        continue;
      }
      value.forEach((item, index) => {
        if (isSchemaObject(item)) {
          const suppressChildDirectRefUsage =
            analysis !== undefined &&
            (key === "oneOf" || key === "anyOf") &&
            getSchemaRef(item) !== undefined;
          this.scanForUnionVariantUsage(
            item,
            [...path, `.${key}[${index}]`],
            owningTypeName,
            suppressChildDirectRefUsage,
          );
        }
      });
    }

    if (isSchemaObject(unwrapped.items)) {
      this.scanForUnionVariantUsage(
        unwrapped.items,
        [...path, ".items"],
        owningTypeName,
      );
    }

    if (isSchemaObject(unwrapped.additionalProperties)) {
      this.scanForUnionVariantUsage(
        unwrapped.additionalProperties,
        [...path, ".additionalProperties"],
        owningTypeName,
      );
    }

    if (isRecord(unwrapped.properties)) {
      for (const [propertyName, propertyDefinition] of Object.entries(
        unwrapped.properties,
      )) {
        if (!isSchemaObject(propertyDefinition)) {
          continue;
        }
        this.scanForUnionVariantUsage(
          propertyDefinition,
          [...path, `.properties.${propertyName}`],
          owningTypeName,
        );
      }
    }
  }

  private lowerNamedDeclaration(
    typeName: string,
    schema: JSONSchema7,
    path: string[],
  ): Declaration[] {
    const normalized = unwrapNullableSchema(schema, formatPath(path));
    if (normalized.nullable) {
      if (this.language === "go") {
        return this.lowerNamedDeclaration(typeName, normalized.schema, path);
      }

      const innerName = `${typeName}Value`;
      const loweredInner = this.lowerAnonymousType(
        normalized.schema,
        innerName,
        path,
      );
      const target =
        loweredInner.type.kind === "named" &&
        loweredInner.type.name === innerName
          ? ({ kind: "named", name: innerName } satisfies TypeExpression)
          : loweredInner.type;
      return [
        ...loweredInner.declarations,
        {
          kind: "alias",
          name: typeName,
          description: schema.description,
          target: { kind: "optional", item: target },
        },
      ];
    }
    const activeSchema = normalized.schema;
    if (
      Object.keys(activeSchema).length === 0 ||
      isMetadataOnlySchema(activeSchema)
    ) {
      this.usesJsonValue = true;
      return [
        {
          kind: "alias",
          name: typeName,
          description: activeSchema.description,
          target: { kind: "json_value" },
        },
      ];
    }

    if (activeSchema.$ref) {
      throw new Error(
        `Top-level schema ${typeName} cannot be a bare $ref without a concrete declaration`,
      );
    }

    if (activeSchema.allOf) {
      return this.lowerAllOfDeclaration(typeName, activeSchema, path);
    }

    const union = this.tryAnalyzeTaggedUnion(activeSchema, path, typeName);
    if (union) {
      return this.lowerUnionDeclaration(typeName, activeSchema, union, path);
    }

    const untaggedUnion = this.tryAnalyzeUntaggedUnion(
      activeSchema,
      path,
      typeName,
    );
    if (untaggedUnion) {
      return untaggedUnion;
    }

    if (activeSchema.type === "object") {
      const mapType = this.tryLowerMapType(
        activeSchema,
        path,
        `${typeName}Value`,
      );
      if (mapType) {
        return [
          ...mapType.declarations,
          {
            kind: "alias",
            name: typeName,
            description: activeSchema.description,
            target: mapType.type,
          },
        ];
      }
      return this.lowerStructDeclaration(typeName, activeSchema, path);
    }

    if (activeSchema.type === "array") {
      if (!activeSchema.items || Array.isArray(activeSchema.items)) {
        throw new Error(`Array schema is missing items at ${formatPath(path)}`);
      }
      const itemName = `${typeName}Item`;
      const lowered = this.lowerAnonymousType(
        assertSchemaObject(activeSchema.items, `${formatPath(path)}.items`),
        itemName,
        [...path, ".items"],
      );
      return [
        ...lowered.declarations,
        {
          kind: "alias",
          name: typeName,
          description: activeSchema.description,
          target: { kind: "array", item: lowered.type },
        },
      ];
    }

    if (isStringEnumSchema(activeSchema)) {
      return [
        {
          kind: "enum",
          name: typeName,
          description: activeSchema.description,
          primitive: "string",
          variants: this.buildEnumVariants(typeName, activeSchema.enum),
        },
      ];
    }

    const primitive = normalizePrimitiveSchema(activeSchema);
    if (primitive) {
      return [
        {
          kind: "alias",
          name: typeName,
          description: activeSchema.description,
          target: {
            kind: "primitive",
            primitive,
          },
        },
      ];
    }

    if (this.isStringPlusEnumAnyOf(activeSchema, path)) {
      return [
        {
          kind: "alias",
          name: typeName,
          description: activeSchema.description,
          target: {
            kind: "primitive",
            primitive: "string",
          },
        },
      ];
    }

    throw new Error(
      `Unsupported top-level schema shape at ${formatPath(path)} for ${typeName}`,
    );
  }

  private lowerAllOfDeclaration(
    typeName: string,
    schema: JSONSchema7,
    path: string[],
  ): Declaration[] {
    if (!schema.allOf || schema.allOf.length === 0) {
      throw new Error(`allOf must contain members at ${formatPath(path)}`);
    }

    const declarations: Declaration[] = [];
    const fields: FlattenField[] = [];
    const seenFieldNames = new Set<string>();

    schema.allOf.forEach((member, index) => {
      const memberPath = [...path, `.allOf[${index}]`];
      const memberSchema = assertSchemaObject(member, formatPath(memberPath));
      if (memberSchema.$ref) {
        const refName = this.resolveRefName(
          memberSchema.$ref,
          formatPath(memberPath),
        );
        const typeRefName = toCamelCase(refName);
        const fieldName = this.makeRustFieldName(
          typeRefName,
          seenFieldNames,
          formatPath(memberPath),
        );
        fields.push({
          kind: "flatten",
          name: fieldName,
          typeName: typeRefName,
        });
        return;
      }

      const memberTypeName = this.reserveGeneratedName(
        `${typeName}AllOf${index + 1}`,
        formatPath(memberPath),
      );
      declarations.push(
        ...this.lowerNamedDeclaration(memberTypeName, memberSchema, memberPath),
      );
      const fieldName = this.makeRustFieldName(
        memberTypeName,
        seenFieldNames,
        formatPath(memberPath),
      );
      fields.push({
        kind: "flatten",
        name: fieldName,
        typeName: memberTypeName,
      });
    });

    declarations.push({
      kind: "struct",
      name: typeName,
      description: schema.description,
      fields,
    });
    return declarations;
  }

  private lowerStructDeclaration(
    typeName: string,
    schema: JSONSchema7,
    path: string[],
  ): Declaration[] {
    const properties = schema.properties;
    if (!properties || !isRecord(properties)) {
      throw new Error(
        `Object schema must define properties at ${formatPath(path)}`,
      );
    }

    if (
      schema.additionalProperties !== undefined &&
      schema.additionalProperties !== false &&
      schema.additionalProperties !== true
    ) {
      throw new Error(
        `Object schemas with both properties and additionalProperties are not supported at ${formatPath(path)}`,
      );
    }

    const requiredProperties = new Set(
      Array.isArray(schema.required) ? schema.required : [],
    );
    // If this definition was recorded as a tagged-union variant, omit the
    // discriminator field from the lowered struct because the generated union
    // serializer emits it on the outer wrapper object. If the same definition
    // is also reused outside tagged-union wrapping, we must keep the field on
    // the struct itself so nested plain-object serialization stays valid.
    const variantUsage = this.variantUsageByDefinition.get(typeName);
    const fields: PropertyField[] = [];
    const declarations: Declaration[] = [];
    const seenFieldNames = new Set<string>();

    for (const [propertyName, propertyDefinition] of Object.entries(
      properties as Record<string, JSONSchema7Definition>,
    )) {
      const propertyPath = [...path, `.properties.${propertyName}`];
      const propertySchema = assertSchemaObject(
        propertyDefinition,
        formatPath(propertyPath),
      );

      if (
        variantUsage &&
        propertyName === variantUsage.propertyName &&
        !this.referencedOutsideTaggedUnion.has(typeName)
      ) {
        continue;
      }

      const lowered = this.lowerAnonymousType(
        propertySchema,
        `${typeName}${toCamelCase(propertyName)}`,
        propertyPath,
      );

      const fieldName = this.makeStructFieldName(
        propertyName,
        seenFieldNames,
        formatPath(propertyPath),
      );
      const nullable = unwrapNullableSchema(
        propertySchema,
        formatPath(propertyPath),
      ).nullable;

      fields.push({
        kind: "property",
        name: fieldName,
        originalName: propertyName,
        description: propertyDescription(propertySchema),
        type: lowered.type,
        required: requiredProperties.has(propertyName),
        nullable,
      });
      declarations.push(...lowered.declarations);
    }

    return [
      {
        kind: "struct",
        name: typeName,
        description: schema.description,
        fields,
      },
      ...declarations,
    ];
  }

  private lowerUnionDeclaration(
    typeName: string,
    schema: JSONSchema7,
    union: UnionAnalysis,
    path: string[],
  ): Declaration[] {
    const declarations: Declaration[] = [];
    for (const variant of union.variants) {
      if (!variant.inlineSchema) {
        continue;
      }
      declarations.push(
        ...this.lowerNamedDeclaration(variant.typeName, variant.inlineSchema, [
          ...path,
          `.variant(${variant.discriminatorValue})`,
        ]),
      );
    }
    declarations.push({
      kind: "union",
      representation: "tagged",
      name: typeName,
      description: schema.description,
      discriminator: union.discriminator,
      variants: union.variants.map((variant) => ({
        name: getNamedVariantName(variant.typeName, variant.discriminatorValue),
        typeName: variant.typeName,
        discriminatorValue: variant.discriminatorValue,
        match: {
          kind: "object",
          requiredProperties: [],
          discriminator: {
            property: union.discriminator,
            value: variant.discriminatorValue,
          },
        },
      })),
    });
    return declarations;
  }

  private tryAnalyzeUntaggedUnion(
    schema: JSONSchema7,
    path: string[],
    ownerTypeName: string,
  ): Declaration[] | undefined {
    const options = schema.oneOf ?? schema.anyOf;
    if (!options) {
      return undefined;
    }
    if (schema.anyOf) {
      if (
        this.isNullableAnyOf(schema, path) ||
        this.isStringPlusEnumAnyOf(schema, path)
      ) {
        return undefined;
      }
    }

    const variantDeclarations: Declaration[] = [];
    const variants: UnionVariant[] = [];
    const seenNames = new Set<string>();

    options.forEach((option, index) => {
      const optionPath = [
        ...path,
        schema.oneOf ? `.oneOf[${index}]` : `.anyOf[${index}]`,
      ];
      const optionSchema = assertSchemaObject(option, formatPath(optionPath));
      const analyzed = this.analyzeUntaggedVariant(
        optionSchema,
        ownerTypeName,
        optionPath,
        index,
      );
      if (seenNames.has(analyzed.variant.name)) {
        throw new Error(
          `Duplicate untagged union variant name ${analyzed.variant.name} at ${formatPath(optionPath)}`,
        );
      }
      seenNames.add(analyzed.variant.name);
      variants.push(analyzed.variant);
      variantDeclarations.push(...analyzed.declarations);
    });

    if (variants.length === 0) {
      return undefined;
    }

    return [
      ...variantDeclarations,
      {
        kind: "union",
        representation: "untagged",
        name: ownerTypeName,
        description: schema.description,
        discriminator: undefined,
        variants,
      },
    ];
  }

  private analyzeUntaggedVariant(
    schema: JSONSchema7,
    ownerTypeName: string,
    path: string[],
    index: number,
  ): UntaggedVariantAnalysis {
    const normalized = unwrapNullableSchema(schema, formatPath(path));
    if (normalized.nullable) {
      throw new Error(
        `Nullable union variants are not supported at ${formatPath(path)}`,
      );
    }
    const activeSchema = normalized.schema;
    if (
      Object.keys(activeSchema).length === 0 ||
      isMetadataOnlySchema(activeSchema)
    ) {
      this.usesJsonValue = true;
      const typeName = this.reserveGeneratedName(
        `${ownerTypeName}JsonValue`,
        formatPath(path),
        false,
      );
      return {
        declarations: [
          {
            kind: "alias",
            name: typeName,
            description: activeSchema.description,
            target: { kind: "json_value" },
          },
        ],
        variant: {
          name: typeName,
          typeName,
          discriminatorValue: undefined,
          match: {
            kind: "object",
            requiredProperties: [],
          },
        },
      };
    }

    if (getSchemaRef(activeSchema)) {
      const refName = this.resolveRefName(
        getSchemaRef(activeSchema) ??
          fail(`Missing ref at ${formatPath(path)}`),
        formatPath(path),
      );
      const refTypeName = toCamelCase(refName);
      const refDefinition = this.resolveDefinitionSchema(
        getSchemaRef(activeSchema) ??
          fail(`Missing ref at ${formatPath(path)}`),
        formatPath(path),
      );
      const taggedUnion = this.tryAnalyzeTaggedUnion(
        refDefinition,
        path,
        refTypeName,
      );
      if (taggedUnion) {
        return {
          declarations: [],
          variant: {
            name: refTypeName,
            typeName: refTypeName,
            discriminatorValue: undefined,
            match: {
              kind: "object",
              requiredProperties: [],
              nestedTaggedUnion: {
                property: taggedUnion.discriminator,
                values: [
                  ...new Set(
                    taggedUnion.variants.map(
                      (variant) => variant.discriminatorValue,
                    ),
                  ),
                ],
              },
            },
          },
        };
      }

      const refPrimitive = normalizePrimitiveSchema(refDefinition);
      if (refPrimitive || this.isStringPlusEnumAnyOf(refDefinition, path)) {
        return {
          declarations: [],
          variant: {
            name: refTypeName,
            typeName: refTypeName,
            discriminatorValue: undefined,
            match: {
              kind:
                (refPrimitive ?? "string") === "integer" ||
                (refPrimitive ?? "string") === "number"
                  ? "number"
                  : ((refPrimitive ?? "string") as "string" | "boolean"),
            },
          },
        };
      }

      return {
        declarations: [],
        variant: {
          name: refTypeName,
          typeName: refTypeName,
          discriminatorValue: undefined,
          match: this.getUntaggedMatchForSchema(refDefinition, path),
        },
      };
    }

    const primitive = normalizePrimitiveSchema(activeSchema);
    if (primitive) {
      const typeName = this.reserveGeneratedName(
        `${ownerTypeName}${getPrimitiveAliasSuffix(primitive)}`,
        formatPath(path),
        false,
      );
      return {
        declarations: [
          {
            kind: "alias",
            name: typeName,
            description: activeSchema.description,
            target: { kind: "primitive", primitive },
          },
        ],
        variant: {
          name: typeName,
          typeName,
          discriminatorValue: undefined,
          match: {
            kind:
              primitive === "integer" || primitive === "number"
                ? "number"
                : primitive,
          },
        },
      };
    }

    if (activeSchema.type === "array") {
      const typeName = this.reserveGeneratedName(
        `${ownerTypeName}Array`,
        formatPath(path),
        false,
      );
      const lowered = this.lowerAnonymousType(activeSchema, typeName, path);
      return {
        declarations:
          lowered.type.kind === "named"
            ? []
            : [
                ...lowered.declarations,
                {
                  kind: "alias",
                  name: typeName,
                  description: activeSchema.description,
                  target: lowered.type,
                },
              ],
        variant: {
          name: typeName,
          typeName,
          discriminatorValue: undefined,
          match: { kind: "array" },
        },
      };
    }

    if (activeSchema.type === "object") {
      const mapType = this.tryLowerMapType(
        activeSchema,
        path,
        `${ownerTypeName}Value`,
      );
      if (mapType) {
        const typeName = this.reserveGeneratedName(
          `${ownerTypeName}Map`,
          formatPath(path),
          false,
        );
        return {
          declarations: [
            ...mapType.declarations,
            {
              kind: "alias",
              name: typeName,
              description: activeSchema.description,
              target: mapType.type,
            },
          ],
          variant: {
            name: typeName,
            typeName,
            discriminatorValue: undefined,
            match: {
              kind: "object",
              requiredProperties: [],
            },
          },
        };
      }

      const typeName = this.reserveGeneratedName(
        `${ownerTypeName}Variant${index + 1}`,
        formatPath(path),
        false,
      );
      return {
        declarations: this.lowerStructDeclaration(typeName, activeSchema, path),
        variant: {
          name: typeName,
          typeName,
          discriminatorValue: undefined,
          match: this.getUntaggedMatchForSchema(activeSchema, path),
        },
      };
    }

    const taggedUnion = this.tryAnalyzeTaggedUnion(
      activeSchema,
      path,
      ownerTypeName,
    );
    if (taggedUnion) {
      const typeName = this.reserveGeneratedName(
        `${ownerTypeName}Variant${index + 1}`,
        formatPath(path),
        false,
      );
      return {
        declarations: this.lowerUnionDeclaration(
          typeName,
          activeSchema,
          taggedUnion,
          path,
        ),
        variant: {
          name: typeName,
          typeName,
          discriminatorValue: undefined,
          match: {
            kind: "object",
            requiredProperties: [],
            nestedTaggedUnion: {
              property: taggedUnion.discriminator,
              values: [
                ...new Set(
                  taggedUnion.variants.map(
                    (variant) => variant.discriminatorValue,
                  ),
                ),
              ],
            },
          },
        },
      };
    }

    throw new Error(
      `Unsupported untagged union variant at ${formatPath(path)}`,
    );
  }

  private getUntaggedMatchForSchema(
    schema: JSONSchema7,
    path: string[],
  ): UnionVariantMatch {
    const primitive = normalizePrimitiveSchema(schema);
    if (primitive) {
      if (primitive === "integer" || primitive === "number") {
        return { kind: "number" };
      }
      return { kind: primitive };
    }
    if (schema.type === "array") {
      return { kind: "array" };
    }
    if (schema.type === "object") {
      const requiredProperties = getObjectRequiredProperties(schema);
      const discriminatorProperty =
        schema.properties && isRecord(schema.properties)
          ? Object.entries(schema.properties).find(
              ([, propertyDefinition]) =>
                isSchemaObject(propertyDefinition) &&
                isLiteralStringDiscriminatorSchema(propertyDefinition) !==
                  undefined,
            )
          : undefined;
      const discriminator =
        discriminatorProperty && isSchemaObject(discriminatorProperty[1])
          ? {
              property: discriminatorProperty[0],
              value:
                isLiteralStringDiscriminatorSchema(discriminatorProperty[1]) ??
                fail(`Missing discriminator at ${formatPath(path)}`),
            }
          : undefined;
      return discriminator
        ? {
            kind: "object",
            requiredProperties,
            discriminator,
          }
        : {
            kind: "object",
            requiredProperties,
          };
    }
    throw new Error(`Unsupported untagged match schema at ${formatPath(path)}`);
  }

  private lowerAnonymousType(
    schema: JSONSchema7,
    generatedName: string,
    path: string[],
  ): LoweredType {
    const normalized = unwrapNullableSchema(schema, formatPath(path));
    const activeSchema = normalized.schema;
    if (
      Object.keys(activeSchema).length === 0 ||
      isMetadataOnlySchema(activeSchema)
    ) {
      this.usesJsonValue = true;
      return {
        type: { kind: "json_value" },
        declarations: [],
      };
    }

    if (activeSchema.$ref) {
      const refName = this.resolveRefName(activeSchema.$ref, formatPath(path));
      return {
        type: { kind: "named", name: toCamelCase(refName) },
        declarations: [],
      };
    }

    if (activeSchema.allOf) {
      this.reserveGeneratedName(generatedName, formatPath(path), false);
      return {
        type: { kind: "named", name: generatedName },
        declarations: this.lowerAllOfDeclaration(
          generatedName,
          activeSchema,
          path,
        ),
      };
    }

    const union = this.tryAnalyzeTaggedUnion(activeSchema, path, generatedName);
    if (union) {
      this.reserveGeneratedName(generatedName, formatPath(path), false);
      return {
        type: { kind: "named", name: generatedName },
        declarations: this.lowerUnionDeclaration(
          generatedName,
          activeSchema,
          union,
          path,
        ),
      };
    }

    const untaggedUnion = this.tryAnalyzeUntaggedUnion(
      activeSchema,
      path,
      generatedName,
    );
    if (untaggedUnion) {
      this.reserveGeneratedName(generatedName, formatPath(path), false);
      return {
        type: { kind: "named", name: generatedName },
        declarations: untaggedUnion,
      };
    }

    if (activeSchema.type === "object") {
      const mapType = this.tryLowerMapType(
        activeSchema,
        path,
        `${generatedName}Value`,
      );
      if (mapType) {
        return {
          type: mapType.type,
          declarations: mapType.declarations,
        };
      }
      this.reserveGeneratedName(generatedName, formatPath(path), false);
      return {
        type: { kind: "named", name: generatedName },
        declarations: this.lowerStructDeclaration(
          generatedName,
          activeSchema,
          path,
        ),
      };
    }

    if (activeSchema.type === "array") {
      if (!activeSchema.items || Array.isArray(activeSchema.items)) {
        throw new Error(`Array schema is missing items at ${formatPath(path)}`);
      }
      const itemSchema = assertSchemaObject(
        activeSchema.items,
        `${formatPath(path)}.items`,
      );
      const loweredItem = this.lowerAnonymousType(
        itemSchema,
        `${generatedName}Item`,
        [...path, ".items"],
      );
      return {
        type: { kind: "array", item: loweredItem.type },
        declarations: loweredItem.declarations,
      };
    }

    if (isStringEnumSchema(activeSchema)) {
      this.reserveGeneratedName(generatedName, formatPath(path), false);
      return {
        type: { kind: "named", name: generatedName },
        declarations: [
          {
            kind: "enum",
            name: generatedName,
            description: activeSchema.description,
            primitive: "string",
            variants: this.buildEnumVariants(generatedName, activeSchema.enum),
          },
        ],
      };
    }

    const primitive = normalizePrimitiveSchema(activeSchema);
    if (primitive) {
      return {
        type: { kind: "primitive", primitive },
        declarations: [],
      };
    }

    if (this.isStringPlusEnumAnyOf(activeSchema, path)) {
      return {
        type: { kind: "primitive", primitive: "string" },
        declarations: [],
      };
    }

    throw new Error(`Unsupported schema shape at ${formatPath(path)}`);
  }

  private tryLowerMapType(
    schema: JSONSchema7,
    path: string[],
    valueTypeName: string,
  ): LoweredType | undefined {
    if (schema.type !== "object") {
      return undefined;
    }
    if (schema.additionalProperties === undefined) {
      const properties =
        schema.properties && isRecord(schema.properties)
          ? Object.keys(schema.properties)
          : [];
      if (properties.length === 0) {
        this.usesJsonValue = true;
        return {
          type: { kind: "json_value" },
          declarations: [],
        };
      }
      return undefined;
    }
    if (schema.additionalProperties === false) {
      return undefined;
    }

    const properties =
      schema.properties && isRecord(schema.properties)
        ? Object.keys(schema.properties)
        : [];
    if (properties.length > 0 && schema.additionalProperties !== true) {
      throw new Error(
        `Objects with both named properties and typed additionalProperties are not supported at ${formatPath(path)}`,
      );
    }
    if (schema.additionalProperties === true) {
      this.usesJsonValue = true;
      return {
        type: { kind: "json_value" },
        declarations: [],
      };
    }
    if (!isSchemaObject(schema.additionalProperties)) {
      throw new Error(
        `Unsupported additionalProperties at ${formatPath(path)}`,
      );
    }

    this.usesMap = true;
    const loweredValue = this.lowerAnonymousType(
      schema.additionalProperties,
      valueTypeName,
      [...path, ".additionalProperties"],
    );
    return {
      type: { kind: "map", value: loweredValue.type },
      declarations: loweredValue.declarations,
    };
  }

  private buildEnumVariants(typeName: string, values: string[]): EnumVariant[] {
    const seenNames = new Set<string>();
    return values.map((value) => {
      const variantName = `${typeName}${toCamelCase(value)}`;
      if (seenNames.has(variantName)) {
        throw new Error(
          `Enum ${typeName} has duplicate variant name ${variantName}`,
        );
      }
      seenNames.add(variantName);
      return {
        name: variantName.slice(typeName.length),
        value,
      };
    });
  }

  private tryAnalyzeTaggedUnion(
    schema: JSONSchema7,
    path: string[],
    ownerTypeName: string,
  ): UnionAnalysis | undefined {
    const options = schema.oneOf ?? schema.anyOf;
    if (!options) {
      return undefined;
    }

    if (schema.anyOf) {
      if (this.isNullableAnyOf(schema, path)) {
        return undefined;
      }
      if (this.isStringPlusEnumAnyOf(schema, path)) {
        return undefined;
      }
    }

    const analyzedOptions = options.map((option, index) => {
      const optionPath = [
        ...path,
        schema.oneOf ? `.oneOf[${index}]` : `.anyOf[${index}]`,
      ];
      const optionSchema = assertSchemaObject(option, formatPath(optionPath));
      const resolvedSchema = optionSchema.$ref
        ? this.resolveDefinitionSchema(
            optionSchema.$ref,
            formatPath(optionPath),
          )
        : optionSchema;
      return {
        optionPath,
        optionSchema,
        resolvedSchema,
      };
    });

    const discriminatorName = this.determineDiscriminatorName(
      schema,
      analyzedOptions,
    );
    if (!discriminatorName) {
      return undefined;
    }

    let variants: AnalyzedVariant[];
    try {
      variants = analyzedOptions.map((option) =>
        this.analyzeUnionVariant(
          ownerTypeName,
          option.optionSchema,
          option.resolvedSchema,
          formatPath(option.optionPath),
          discriminatorName,
          getDiscriminator(schema)?.mapping,
        ),
      );
    } catch (error) {
      this.warnTaggedUnionFallback(
        ownerTypeName,
        path,
        `could not resolve discriminator values for property ${JSON.stringify(discriminatorName)}`,
        error,
      );
      return undefined;
    }
    const discriminatorValues = new Set<string>();
    for (const variant of variants) {
      if (discriminatorValues.has(variant.discriminatorValue)) {
        this.warnTaggedUnionFallback(
          ownerTypeName,
          path,
          `duplicate discriminator value ${JSON.stringify(variant.discriminatorValue)} for property ${JSON.stringify(discriminatorName)}`,
        );
        return undefined;
      }
      discriminatorValues.add(variant.discriminatorValue);
    }

    return {
      discriminator: discriminatorName,
      variants,
    };
  }

  private warnTaggedUnionFallback(
    ownerTypeName: string,
    path: string[],
    reason: string,
    error?: unknown,
  ): void {
    const errorSuffix =
      error instanceof Error && error.message.length > 0
        ? ` (${error.message})`
        : "";
    // We intentionally warn instead of throwing here because some upstream
    // schemas are still representable as untagged unions, and some generated
    // outputs are currently patched manually after codegen. The warning makes
    // that silent downgrade visible during generation.
    console.warn(
      `[schema-to-code] Falling back from tagged union for ${ownerTypeName} at ${formatPath(path)}: ${reason}${errorSuffix}`,
    );
  }

  private determineDiscriminatorName(
    schema: JSONSchema7,
    options: Array<{
      optionSchema: JSONSchema7;
      resolvedSchema: JSONSchema7;
    }>,
  ): string | undefined {
    const explicit = getDiscriminator(schema)?.propertyName;
    if (explicit) {
      return explicit;
    }

    const candidateSets = options.map(({ resolvedSchema }) => {
      if (resolvedSchema.type !== "object" || !resolvedSchema.properties) {
        return new Map<string, string>();
      }
      const result = new Map<string, string>();
      for (const [propertyName, propertyDefinition] of Object.entries(
        resolvedSchema.properties,
      )) {
        const propertySchema = assertSchemaObject(
          propertyDefinition,
          `property ${propertyName}`,
        );
        const literal = isLiteralStringDiscriminatorSchema(propertySchema);
        if (literal) {
          result.set(propertyName, literal);
        }
      }
      return result;
    });

    const first = candidateSets[0];
    if (!first) {
      return undefined;
    }
    for (const candidate of first.keys()) {
      const values = new Set<string>();
      let valid = true;
      for (const candidates of candidateSets) {
        const value = candidates.get(candidate);
        if (!value || values.has(value)) {
          valid = false;
          break;
        }
        values.add(value);
      }
      if (valid) {
        return candidate;
      }
    }
    return undefined;
  }

  private analyzeUnionVariant(
    ownerTypeName: string,
    optionSchema: JSONSchema7,
    resolvedSchema: JSONSchema7,
    path: string,
    discriminatorName: string,
    discriminatorMapping: Record<string, string> | undefined,
  ): AnalyzedVariant {
    if (resolvedSchema.type !== "object" || !resolvedSchema.properties) {
      throw new Error(`Union variants must be object schemas at ${path}`);
    }

    const discriminatorProperty = resolvedSchema.properties[discriminatorName];
    const literalValue = discriminatorProperty
      ? isLiteralStringDiscriminatorSchema(
          assertSchemaObject(
            discriminatorProperty,
            `${path}.properties.${discriminatorName}`,
          ),
        )
      : undefined;

    let discriminatorValue = literalValue;
    if (!discriminatorValue && optionSchema.$ref && discriminatorMapping) {
      for (const [mappingValue, mappingRef] of Object.entries(
        discriminatorMapping,
      )) {
        if (mappingRef === optionSchema.$ref) {
          discriminatorValue = mappingValue;
          break;
        }
      }
    }

    if (!discriminatorValue) {
      throw new Error(
        `Could not determine discriminator value for union variant at ${path}`,
      );
    }

    if (optionSchema.$ref) {
      const refName = this.resolveRefName(optionSchema.$ref, path);
      return {
        typeName: toCamelCase(refName),
        discriminatorValue,
        refName: toCamelCase(refName),
      };
    }

    const variantTypeName = this.reserveGeneratedName(
      `${ownerTypeName}${toCamelCase(discriminatorValue)}`,
      path,
    );
    return {
      typeName: variantTypeName,
      discriminatorValue,
      inlineSchema: resolvedSchema,
    };
  }

  private isNullableAnyOf(schema: JSONSchema7, path: string[]): boolean {
    return unwrapNullableSchema(schema, formatPath(path)).nullable;
  }

  private isStringPlusEnumAnyOf(schema: JSONSchema7, path: string[]): boolean {
    if (!schema.anyOf || schema.anyOf.length < 2) {
      return false;
    }
    const options = schema.anyOf.map((option, index) => {
      const optionSchema = assertSchemaObject(
        option,
        `${formatPath(path)}.anyOf[${index}]`,
      );
      const ref = getSchemaRef(optionSchema);
      return ref
        ? this.resolveDefinitionSchema(
            ref,
            `${formatPath(path)}.anyOf[${index}]`,
          )
        : optionSchema;
    });
    const isStringLikeOption = (option: JSONSchema7): boolean =>
      isStringEnumLike(option) || this.isStringPlusEnumAnyOf(option, path);
    const stringOption = options.find((option) => isStringLikeOption(option));
    const everyOtherOptionIsStringLiteral = options
      .filter((option) => option !== stringOption)
      .every(
        (option) =>
          getConstStringSchemaValue(option) !== undefined ||
          isStringEnumSchema(option) ||
          isStringLikeOption(option),
      );
    return (
      Boolean(stringOption && everyOtherOptionIsStringLiteral) ||
      isStringLikeAnyOf(schema)
    );
  }

  private resolveRefName(ref: string, path: string): string {
    const prefix = "#/definitions/";
    if (!ref.startsWith(prefix)) {
      throw new Error(
        `Only internal #/definitions refs are supported at ${path}: ${ref}`,
      );
    }
    return ref.slice(prefix.length);
  }

  private resolveDefinitionSchema(ref: string, path: string): JSONSchema7 {
    const refName = this.resolveRefName(ref, path);
    const definition = this.definitions[refName];
    return assertSchemaObject(definition, `#/definitions/${refName}`);
  }

  private reserveGeneratedName(
    name: string,
    path: string,
    requireFresh = true,
  ): string {
    if (requireFresh && this.reservedNames.has(name)) {
      throw new Error(
        `Generated type name ${name} at ${path} collides with an existing name`,
      );
    }
    this.reservedNames.add(name);
    return name;
  }

  private makeStructFieldName(
    propertyName: string,
    seenNames: Set<string>,
    path: string,
  ): string {
    const name =
      this.language === "go"
        ? toCamelCase(propertyName)
        : this.makeRustFieldName(propertyName, seenNames, path);
    if (this.language === "go") {
      if (seenNames.has(name)) {
        throw new Error(`Duplicate field name ${name} at ${path}`);
      }
      seenNames.add(name);
    }
    return name;
  }

  private makeRustFieldName(
    rawName: string,
    seenNames: Set<string>,
    path: string,
  ): string {
    const snake = toSnakeCase(rawName);
    const identifier = RUST_KEYWORDS.has(snake) ? `r#${snake}` : snake;
    if (seenNames.has(identifier)) {
      throw new Error(`Duplicate field name ${identifier} at ${path}`);
    }
    seenNames.add(identifier);
    return identifier;
  }
}

function getDiscriminator(schema: JSONSchema7):
  | {
      propertyName: string | undefined;
      mapping: Record<string, string> | undefined;
    }
  | undefined {
  const discriminator = (schema as Record<string, unknown>)["discriminator"];
  if (!isRecord(discriminator)) {
    return undefined;
  }

  const propertyName =
    typeof discriminator["propertyName"] === "string"
      ? discriminator["propertyName"]
      : undefined;

  const mapping = isRecord(discriminator["mapping"])
    ? Object.fromEntries(
        Object.entries(discriminator["mapping"]).filter(
          (entry): entry is [string, string] => typeof entry[1] === "string",
        ),
      )
    : undefined;

  return {
    propertyName,
    mapping,
  };
}

function fail(message: string): never {
  throw new Error(message);
}

const METADATA_ONLY_KEYS = new Set([
  "description",
  "title",
  "default",
  "example",
  "examples",
  "deprecated",
  "readOnly",
  "writeOnly",
  "nullable",
]);

const RUST_KEYWORDS = new Set([
  "as",
  "break",
  "const",
  "continue",
  "crate",
  "else",
  "enum",
  "extern",
  "false",
  "fn",
  "for",
  "if",
  "impl",
  "in",
  "let",
  "loop",
  "match",
  "mod",
  "move",
  "mut",
  "pub",
  "ref",
  "return",
  "self",
  "Self",
  "static",
  "struct",
  "super",
  "trait",
  "true",
  "type",
  "unsafe",
  "use",
  "where",
  "while",
  "async",
  "await",
  "dyn",
  "abstract",
  "become",
  "box",
  "do",
  "final",
  "macro",
  "override",
  "priv",
  "try",
  "typeof",
  "unsized",
  "virtual",
  "yield",
]);
