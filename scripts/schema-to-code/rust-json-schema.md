# JSON Schema to Rust

The input is a JSON Schema 7 document whose root defines named schemas under `definitions`. Throw an error for any other root shape.

## Core Rules

- Preserve definition order, property order, and enum value order.
- Reuse referenced definitions for `$ref`. Do not create wrapper types for plain references.
- Support `$recursiveRef: "#"` only as a self-reference to the current top-level definition.
- Keep schema names, except for converting Rust type names to CamelCase and field names to snake_case.
- Preserve descriptions as `///` comments.
- Derive `Serialize` and `Deserialize` for structs and enums.
- Ignore validation-only metadata such as `format`, `default`, `minimum`, `maximum`, and similar non-type keywords.
- If a schema shape is not explicitly covered here, throw an error.

## Primitive Types

- `string` -> `String`
- `boolean` -> `bool`
- `integer` -> `i64`
- `number` -> `f64`

Primitive definitions become named type aliases.

## Nullability and Optionality

- A nullable schema is either `type: ["null", T]` or `anyOf: [T, { "type": "null" }]`.
- Nullable and non-required fields use `Option<T>`.
- Non-required fields use `#[serde(skip_serializing_if = "Option::is_none")]`.
- A top-level nullable definition lowers to `type Name = Option<NameValue>;`, where `NameValue` is the lowered non-null schema.

## Objects

- An object with named `properties` lowers to a struct.
- Add serde rename attributes when the Rust field name differs from the JSON property name.
- Complex property schemas must lower to named types. Do not inline nested structs, unions, enums, arrays, or maps inside a parent struct.
- If `additionalProperties: true` appears together with named properties, generate only the named fields and ignore extra keys.

## Maps

- An object with `additionalProperties` as a schema lowers to `HashMap<String, T>`.
- If that value schema is complex, lower it to a named type and use that type as `T`.
- An object with no `properties` and no `additionalProperties` lowers to `serde_json::Value`.
- An object with `additionalProperties: true` and no named properties lowers to `serde_json::Value`.

## Unconstrained Schemas

- A schema with no structural type information, such as `{}` or a metadata-only schema, lowers to `serde_json::Value`.

## Arrays

- `type: "array"` lowers to `Vec<T>`.
- `items` must be a single schema object.
- If the item schema is complex, lower it to a named type and use that type as `T`.

## Enums

- String enums lower to Rust enums with `#[serde(rename = "...")]` on each variant.
- Other enum primitive types are unsupported.

## allOf

- `allOf` lowers to a struct with one flattened field per member schema.
- Use `#[serde(flatten)]` on each field.
- A member may be a `$ref` or an inline schema.
- Inline members must lower to their own named declarations first.

## Tagged Unions

- A discriminated `oneOf` or `anyOf` lowers to an enum with `#[serde(tag = "...")]`.
- Omit the discriminator property from the variant structs.
- The discriminator may come from `discriminator.propertyName`, or by inspecting object variants for a shared property whose value is a single string literal.
- A single-value string `const` and a single-value string `enum` are both valid discriminator values.
- A tagged union with one variant is still a tagged union.

## Untagged Unions

- A non-discriminated `oneOf` or `anyOf` may lower to `#[serde(untagged)] enum`.
- Variants may be primitive, array, object, map, or a reference to a named type.
- Object variants may also be references to tagged unions and should reuse those named types directly.
- If variants cannot be distinguished by the supported lowering rules, throw an error.

## Special anyOf Rules

- `anyOf` used only for nullability follows the nullability rules above.
- `anyOf` that combines a general string schema with string literals or string enums lowers to `String`.
- Other `anyOf` cases use the tagged-union or untagged-union rules above if they fit.
- Otherwise, throw an error.

## Unsupported Shapes

 - Boolean JSON Schema.
 - Tuple arrays where `items` is an array.
 - Bare top-level `$ref` definitions.
