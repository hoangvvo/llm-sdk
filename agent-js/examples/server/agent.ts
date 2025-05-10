import { Agent, type InstructionParam } from "@hoangvvo/llm-agent";
import type { LanguageModel } from "@hoangvvo/llm-sdk";
import type { ModelInfo } from "../get-model.ts";
import { getArtifactTools } from "./artifacts.tools.ts";
import type { MyContext } from "./context.ts";
import { getCryptoPriceTool, getStockPriceTool } from "./finance.tools.ts";
import { getNewsTool, searchWikipediaTool } from "./information.tools.ts";
import { getCoordinatesTool, getWeatherTool } from "./weather.tools.ts";

const instructions: InstructionParam<MyContext>[] = [
  `Answer in markdown format.
To access certain tools, the user may have to provide corresponding API keys in the context fields on the UI.`,
  (context) =>
    `The user name is ${context.name ?? "<not provided>"}.
The user location is ${context.location ?? "<not provided>"}.
The user speaks ${context.language ?? "<not provided>"} language.`,
  () => `The current date is ${new Date().toDateString()}.`,
  `For substantive deliverables (documents/specs/code), use the artifact tools (artifact_create, artifact_update, artifact_get, artifact_list, artifact_delete).
Keep chat replies brief and put the full document content into artifacts via these tools, rather than pasting large content into chat. Reference documents by their id.`,
];

export const availableTools = [
  getStockPriceTool,
  getCryptoPriceTool,
  searchWikipediaTool,
  getNewsTool,
  getCoordinatesTool,
  getWeatherTool,
  ...getArtifactTools(),
];

// Create a new instance of Agent for the request
// In your application, you may want to reuse the same instance of Agent throughout the lifetime of your app
export function createAgent(
  model: LanguageModel,
  modelInfo: ModelInfo,
  options?: {
    enabledTools?: string[] | undefined;
    disabledInstructions?: boolean | undefined;
    temperature?: number;
    top_p?: number;
    top_k?: number;
    frequency_penalty?: number;
    presence_penalty?: number;
  },
): Agent<MyContext> {
  const { enabledTools, disabledInstructions, ...agentParams } = options ?? {};

  const toolNameSet = enabledTools ? new Set(enabledTools) : null;
  const tools =
    toolNameSet === null
      ? availableTools
      : availableTools.filter((tool) => toolNameSet.has(tool.name));
  const agentInstructions = disabledInstructions ? [] : instructions;

  return new Agent<MyContext>({
    name: "MyAgent",
    instructions: agentInstructions,
    model,
    tools,
    max_turns: 5,
    ...(modelInfo.audio && { audio: modelInfo.audio }),
    ...(modelInfo.reasoning && { reasoning: modelInfo.reasoning }),
    ...(modelInfo.modalities && { modalities: modelInfo.modalities }),
    ...agentParams,
  });
}
