import goAgentTypes from "../../../agent-go/types.go?raw";
import jsAgentTypes from "../../../agent-js/src/types.ts?raw";
import rustAgentTypes from "../../../agent-rust/src/types.rs?raw";
import goTypes from "../../../sdk-go/types.go?raw";
import jsTypes from "../../../sdk-js/src/types.ts?raw";
import rustTypes from "../../../sdk-rust/src/types.rs?raw";
import { extractCombined } from "./treesitter-extract";

export const extractGoTypes = (typeNames: string[]) =>
  extractCombined(goTypes, typeNames, "go");

export const extractRustTypes = (typeNames: string[]) =>
  extractCombined(rustTypes, typeNames, "rust");

export const extractTypescriptTypes = (typeNames: string[]) =>
  extractCombined(jsTypes, typeNames, "ts");

export const extractGoAgentTypes = (typeNames: string[]) =>
  extractCombined(goAgentTypes, typeNames, "go");

export const extractRustAgentTypes = (typeNames: string[]) =>
  extractCombined(rustAgentTypes, typeNames, "rust");

export const extractTypescriptAgentTypes = (typeNames: string[]) =>
  extractCombined(jsAgentTypes, typeNames, "ts");
