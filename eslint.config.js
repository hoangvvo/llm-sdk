import { includeIgnoreFile } from "@eslint/compat";
import pluginJs from "@eslint/js";
import eslintPluginPrettierRecommended from "eslint-plugin-prettier/recommended";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import { globalIgnores } from "eslint/config";
import globals from "globals";
import { join } from "node:path";
import tseslint from "typescript-eslint";

const gitignorePath = join(import.meta.dirname, ".gitignore");

/** @type {import('eslint').Linter.Config[]} */
export default [
  globalIgnores(["**/node_modules/**", "**/dist/**"]),
  includeIgnoreFile(gitignorePath, "Imported .gitignore patterns"),
  {
    files: ["**/*.{js,mjs,cjs,ts}"],
    rules: {
      "@typescript-eslint/consistent-type-imports": "error",
    },
  },
  pluginJs.configs.recommended,
  ...tseslint.configs.recommended,
  ...tseslint.configs.strictTypeChecked.map((config) => ({
    ...config,
    files: ["agent-js/src/**/*.{ts,tsx}", "sdk-js/src/**/*.{ts,tsx}"],
  })),
  ...tseslint.configs.stylisticTypeChecked.map((config) => ({
    ...config,
    files: ["agent-js/src/**/*.{ts,tsx}", "sdk-js/src/**/*.{ts,tsx}"],
  })),
  {
    files: ["./website/**/*.{ts,tsx}"],
    ...reactHooks.configs["recommended-latest"],
  },
  {
    files: ["./website/**/*.{ts,tsx}"],
    ...reactRefresh.configs.vite,
  },
  {
    languageOptions: {
      globals: { ...globals.browser, ...globals.node },
      parserOptions: {
        project: "./**/tsconfig.json",
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
