/* eslint-disable @typescript-eslint/no-unsafe-call */
/* eslint-disable @typescript-eslint/no-unsafe-return */
/* eslint-disable @typescript-eslint/no-unsafe-assignment */
/* eslint-disable @typescript-eslint/no-unsafe-member-access */
/* eslint-disable @typescript-eslint/no-explicit-any */
import type { JSONSchema, LanguageModelInput } from "../src/types.ts";

/**
 * If the schema has a type of [T, "null"], it means that the property is optional
 * and can be omitted in older models. This function will convert the schema
 * to a compatible format by removing the "null" type and making the property
 * required again.
 */
export function transformCompatibleSchema(schema: any): JSONSchema {
  const newSchema = {
    ...schema,
  };
  if (schema.type === "object" && schema.properties) {
    const newProperties = {
      ...schema.properties,
    };
    for (const [key, value] of Object.entries(
      schema.properties as Record<string, any>,
    )) {
      const prop = value;
      if (Array.isArray(prop.type) && prop.type[1] === "null") {
        newProperties[key] = {
          ...prop,
          type: prop.type[0],
        };
        newSchema.required =
          schema.required?.filter((k: string) => k !== key) ?? [];
      }
    }
    return {
      ...newSchema,
      properties: newProperties,
    };
  }
  return schema;
}

export function transformInputForCompatibleSchema(
  input: LanguageModelInput,
): LanguageModelInput {
  const newInput = { ...input };
  if (
    newInput.response_format?.type === "json" &&
    newInput.response_format.schema
  ) {
    newInput.response_format.schema = transformCompatibleSchema(
      newInput.response_format.schema,
    );
  }
  if (newInput.tools) {
    newInput.tools = newInput.tools.map((tool) => {
      return {
        ...tool,
        parameters: transformCompatibleSchema(tool.parameters),
      };
    });
  }
  return newInput;
}
