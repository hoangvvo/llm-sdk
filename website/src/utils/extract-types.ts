import snippets from "virtual:type-snippets";

type Language = "ts" | "rust" | "go";
type SourceGroup = keyof typeof snippets;

function extractCombined(
  sourceGroup: SourceGroup,
  typeNames: string[],
  language: Language,
): string {
  const declarations = snippets[sourceGroup][language];
  return typeNames
    .map((name) => declarations[name])
    .filter((snippet): snippet is string => snippet !== undefined)
    .join("\n\n");
}

export const extractGoTypes = (typeNames: string[]) =>
  extractCombined("sdk", typeNames, "go");

export const extractRustTypes = (typeNames: string[]) =>
  extractCombined("sdk", typeNames, "rust");

export const extractTypescriptTypes = (typeNames: string[]) =>
  extractCombined("sdk", typeNames, "ts");

export const extractGoAgentTypes = (typeNames: string[]) =>
  extractCombined("agentTypes", typeNames, "go");

export const extractRustAgentTypes = (typeNames: string[]) =>
  extractCombined("agentTypes", typeNames, "rust");

export const extractTypescriptAgentParamsTypes = (typeNames: string[]) =>
  extractCombined("agentParams", typeNames, "ts");

export const extractGoAgentParamsTypes = (typeNames: string[]) =>
  extractCombined("agentParams", typeNames, "go");

export const extractRustAgentParamsTypes = (typeNames: string[]) =>
  extractCombined("agentParams", typeNames, "rust");

export const extractTypescriptAgentTypes = (typeNames: string[]) =>
  extractCombined("agentTypes", typeNames, "ts");

export const extractGoAgentToolTypes = (typeNames: string[]) =>
  extractCombined("agentTool", typeNames, "go");

export const extractRustAgentToolTypes = (typeNames: string[]) =>
  extractCombined("agentTool", typeNames, "rust");

export const extractTypescriptAgentToolTypes = (typeNames: string[]) =>
  extractCombined("agentTool", typeNames, "ts");

export const extractGoAgentToolkitTypes = (typeNames: string[]) =>
  extractCombined("agentToolkit", typeNames, "go");

export const extractRustAgentToolkitTypes = (typeNames: string[]) =>
  extractCombined("agentToolkit", typeNames, "rust");

export const extractTypescriptAgentToolkitTypes = (typeNames: string[]) =>
  extractCombined("agentToolkit", typeNames, "ts");

export const extractGoAgentMcpTypes = (typeNames: string[]) =>
  extractCombined("agentMcp", typeNames, "go");

export const extractRustAgentMcpTypes = (typeNames: string[]) =>
  extractCombined("agentMcp", typeNames, "rust");

export const extractTypescriptAgentMcpTypes = (typeNames: string[]) =>
  extractCombined("agentMcp", typeNames, "ts");

export const extractTypescriptAgentInstructionTypes = (typeNames: string[]) =>
  extractCombined("agentInstruction", typeNames, "ts");

export const extractGoAgentInstructionTypes = (typeNames: string[]) =>
  extractCombined("agentInstruction", typeNames, "go");

export const extractRustAgentInstructionTypes = (typeNames: string[]) =>
  extractCombined("agentInstruction", typeNames, "rust");

export const extractTypescriptAgentRunTypes = (typeNames: string[]) =>
  extractCombined("agentRun", typeNames, "ts");

export const extractGoAgentRunTypes = (typeNames: string[]) =>
  extractCombined("agentRun", typeNames, "go");

export const extractRustAgentRunTypes = (typeNames: string[]) =>
  extractCombined("agentRun", typeNames, "rust");
