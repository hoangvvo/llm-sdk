import goAgentInstructionTypes from "../../../agent-go/instruction.go?raw";
import goAgentParamsTypes from "../../../agent-go/params.go?raw";
import goAgentToolTypes from "../../../agent-go/tool.go?raw";
import goAgentToolkitTypes from "../../../agent-go/toolkit.go?raw";
import goAgentTypes from "../../../agent-go/types.go?raw";
import jsAgentInstructionTypes from "../../../agent-js/src/instruction.ts?raw";
import jsAgentParamsTypes from "../../../agent-js/src/params.ts?raw";
import jsAgentToolTypes from "../../../agent-js/src/tool.ts?raw";
import jsAgentToolkitTypes from "../../../agent-js/src/toolkit.ts?raw";
import jsAgentTypes from "../../../agent-js/src/types.ts?raw";
import rustAgentInstructionTypes from "../../../agent-rust/src/instruction.rs?raw";
import rustAgentParamsTypes from "../../../agent-rust/src/params.rs?raw";
import rustAgentToolTypes from "../../../agent-rust/src/tool.rs?raw";
import rustAgentToolkitTypes from "../../../agent-rust/src/toolkit.rs?raw";
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

export const extractTypescriptAgentParamsTypes = (typeNames: string[]) =>
  extractCombined(jsAgentParamsTypes, typeNames, "ts");

export const extractGoAgentParamsTypes = (typeNames: string[]) =>
  extractCombined(goAgentParamsTypes, typeNames, "go");

export const extractRustAgentParamsTypes = (typeNames: string[]) =>
  extractCombined(rustAgentParamsTypes, typeNames, "rust");

export const extractTypescriptAgentTypes = (typeNames: string[]) =>
  extractCombined(jsAgentTypes, typeNames, "ts");

export const extractGoAgentToolTypes = (typeNames: string[]) =>
  extractCombined(goAgentToolTypes, typeNames, "go");

export const extractRustAgentToolTypes = (typeNames: string[]) =>
  extractCombined(rustAgentToolTypes, typeNames, "rust");

export const extractTypescriptAgentToolTypes = (typeNames: string[]) =>
  extractCombined(jsAgentToolTypes, typeNames, "ts");

export const extractGoAgentToolkitTypes = (typeNames: string[]) =>
  extractCombined(goAgentToolkitTypes, typeNames, "go");

export const extractRustAgentToolkitTypes = (typeNames: string[]) =>
  extractCombined(rustAgentToolkitTypes, typeNames, "rust");

export const extractTypescriptAgentToolkitTypes = (typeNames: string[]) =>
  extractCombined(jsAgentToolkitTypes, typeNames, "ts");

export const extractTypescriptAgentInstructionTypes = (typeNames: string[]) =>
  extractCombined(jsAgentInstructionTypes, typeNames, "ts");

export const extractGoAgentInstructionTypes = (typeNames: string[]) =>
  extractCombined(goAgentInstructionTypes, typeNames, "go");

export const extractRustAgentInstructionTypes = (typeNames: string[]) =>
  extractCombined(rustAgentInstructionTypes, typeNames, "rust");
