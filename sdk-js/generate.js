import { copyFile, writeFile } from "fs/promises";
import { compileFromFile } from "json-schema-to-typescript";
import { join } from "path";

const __dirname = import.meta.dirname;

const originalSchemaPath = join(__dirname, "..", "schema", "schema.json");

await copyFile(
  originalSchemaPath,
  join(__dirname, "src", "schema", "schema.json"),
);

// Generate typescript
const ts = await compileFromFile(originalSchemaPath, {
  bannerComment: false,
  unreachableDefinitions: true,
});
await writeFile(join(__dirname, "src", "schema", "schema.ts"), ts);
