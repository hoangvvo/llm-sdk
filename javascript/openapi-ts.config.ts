import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
  client: "@hey-api/client-fetch",
  input: "../schemas/openapi.json",
  output: "./src/schemas/",
  exportCore: false,
  plugins: [
    "@hey-api/schemas", // preserves default output
    "@hey-api/types", // preserves default output
  ],
});
