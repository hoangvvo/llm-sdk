declare module "virtual:type-snippets" {
  type Language = "ts" | "rust" | "go";
  type SourceGroup =
    | "sdk"
    | "agentTypes"
    | "agentParams"
    | "agentTool"
    | "agentToolkit"
    | "agentMcp"
    | "agentInstruction"
    | "agentRun";

  const snippets: Record<SourceGroup, Record<Language, Record<string, string>>>;
  export default snippets;
}
