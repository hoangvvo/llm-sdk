import { copyFile, writeFile } from "fs/promises";
import { compileFromFile } from "json-schema-to-typescript";
import { join } from "path";
import * as prettier from "prettier";

const __dirname = import.meta.dirname;

const originalSchemaPath = join(__dirname, "..", "..", "schema", "schema.json");
const outputSchemaPath = join(__dirname, "..", "schema.json");
const outputTypesPath = join(__dirname, "..", "src", "types.ts");

await copyFile(originalSchemaPath, outputSchemaPath);

// Generate typescript
let ts = await compileFromFile(originalSchemaPath, {
  bannerComment: "",
  unreachableDefinitions: true,
});
ts = await prettier.format(ts, { parser: "typescript" });
await writeFile(outputTypesPath, ts);
