import pluginJs from "@eslint/js";
import eslintPluginPrettierRecommended from "eslint-plugin-prettier/recommended";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
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
  ...tseslint.configs.strictTypeChecked.map((config) => ({
    ...config,
    files: ["**/src/**/*.{ts,tsx}"],
  })),
  ...tseslint.configs.stylisticTypeChecked,
  {
    files: ["./app/**/*.{ts,tsx}"],
    ...reactHooks.configs["recommended-latest"],
  },
  {
    files: ["./app/**/*.{ts,tsx}"],
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
