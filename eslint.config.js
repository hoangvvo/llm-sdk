import pluginJs from "@eslint/js";
import eslintPluginPrettierRecommended from "eslint-plugin-prettier/recommended";
import { globalIgnores } from "eslint/config";
import globals from "globals";
import tseslint from "typescript-eslint";

/** @type {import('eslint').Linter.Config[]} */
export default [
  globalIgnores(["**/node_modules/**", "**/dist/**"]),
  {
    files: ["**/*.{js,mjs,cjs,ts}"],
    rules: {
      "@typescript-eslint/consistent-type-imports": "error",
    },
  },
  pluginJs.configs.recommended,
  ...tseslint.configs.strictTypeChecked,
  ...tseslint.configs.stylisticTypeChecked,
  {
    languageOptions: {
      globals: { ...globals.browser, ...globals.node },
      parserOptions: {
        project: ["./schema/tsconfig.json", "./sdk-js/tsconfig.json"],
        projectService: {
          allowDefaultProject: ["*.js", "*.mjs"],
        },
        tsconfigRootDir: import.meta.dirname,
      },
    },
  },
  {
    ...tseslint.configs.disableTypeChecked,
    files: ["**/*.js"],
  },
  eslintPluginPrettierRecommended,
];
