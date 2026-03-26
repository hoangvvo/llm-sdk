# JSON Schema to Go

The input is a JSON Schema 7 document whose root defines named schemas under `definitions`. Throw an error for any other root shape.

## Core Rules

- Preserve definition order, property order, and enum value order.
- Reuse referenced definitions for `$ref`. Do not create wrapper types for plain references.
- Support `$recursiveRef: "#"` only as a self-reference to the current top-level definition.
- Keep schema names, except for converting Go type and field names to CamelCase.
- Preserve descriptions as `//` comments.
- Ignore validation-only metadata such as `format`, `default`, `minimum`, `maximum`, and similar non-type keywords.
- If a schema shape is not explicitly covered here, throw an error.

## Primitive Types

- `string` -> `string`
- `boolean` -> `bool`
- `integer` -> `int`
- `number` -> `float64`

Primitive definitions become named type aliases.

## Nullability and Optionality

- A nullable schema is either `type: ["null", T]` or `anyOf: [T, { "type": "null" }]`.
- Nullable fields use pointers, except slices and maps remain non-pointer.
- Non-required fields use `omitempty`.
- A top-level nullable definition lowers the same as its non-null schema. Go does not need a separate top-level optional alias.

## Objects

- An object with named `properties` lowers to a struct.
- Every field must have a `json` tag using the original property name.
- Complex property schemas must lower to named types. Do not inline nested structs, unions, enums, arrays, or maps inside a parent struct.
- Any object with `additionalProperties: true` lowers to `any`, even if it also defines named properties.

## Maps

- An object with `additionalProperties` as a schema lowers to `map[string]T`.
- If that value schema is complex, lower it to a named type and use that type as `T`.
- An object with no `properties` and no `additionalProperties` lowers to `any`.
- An object with `additionalProperties: true` and no named properties lowers to `any`.

## Unconstrained Schemas

- A schema with no structural type information, such as `{}` or a metadata-only schema, lowers to `any`.

## Arrays

- `type: "array"` lowers to `[]T`.
- `items` must be a single schema object.
- If the item schema is complex, lower it to a named type and use that type as `T`.
- Optional arrays remain `[]T` and rely on `omitempty`.

## Enums

- String enums lower to a named `string` type plus constants.
- Enum constant names are the enum type name plus the CamelCase enum value.
- Other enum primitive types are unsupported.

## allOf

- `allOf` lowers to a struct that embeds one field per member schema.
- A member may be a `$ref` or an inline schema.
- Inline members must lower to their own named declarations first.

## Tagged Unions

- A discriminated `oneOf` or `anyOf` lowers to a wrapper struct with one pointer field per variant.
- Generate `MarshalJSON` and `UnmarshalJSON` on that wrapper struct.
- Omit the discriminator property from the variant structs.
- The discriminator may come from `discriminator.propertyName`, or by inspecting object variants for a shared property whose value is a single string literal.
- A single-value string `const` and a single-value string `enum` are both valid discriminator values.
- A tagged union with one variant is still a tagged union.

## Untagged Unions

- A non-discriminated `oneOf` or `anyOf` may lower to a wrapper struct with one pointer field per variant and custom `MarshalJSON` / `UnmarshalJSON`.
- Variants may be primitive, array, object, map, or a reference to a named type.
- Object variants are matched in this order:
- by a literal discriminator property when one exists on the object schema
- by a nested tagged union discriminator when the variant is a reference to a tagged union
- by required property presence
- If variants cannot be distinguished by these rules, throw an error.

## Special anyOf Rules

- `anyOf` used only for nullability follows the nullability rules above.
- `anyOf` that combines a general string schema with string literals or string enums lowers to `string`.
- Other `anyOf` cases use the tagged-union or untagged-union rules above if they fit.
- Otherwise, throw an error.

## Unsupported Shapes

 - Boolean JSON Schema.
 - Tuple arrays where `items` is an array.
 - Bare top-level `$ref` definitions.
