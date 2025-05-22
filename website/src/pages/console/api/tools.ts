import { availableTools } from "../../../../../agent-js/examples/server/agent.ts";

export function GET() {
  const tools = availableTools.map((tool) => ({
    name: tool.name,
    description: tool.description,
  }));

  return Response.json(tools);
}
