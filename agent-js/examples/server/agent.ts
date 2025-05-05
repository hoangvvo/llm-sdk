import { Agent, type InstructionParam } from "@hoangvvo/llm-agent";
import type { LanguageModel } from "@hoangvvo/llm-sdk";
import type { ModelInfo } from "../get-model.ts";
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
];

export const availableTools = [
  getStockPriceTool,
  getCryptoPriceTool,
  searchWikipediaTool,
  getNewsTool,
  getCoordinatesTool,
  getWeatherTool,
];

// Create a new instance of Agent for the request
// In your application, you may want to reuse the same instance of Agent throughout the lifetime of your app
export function createAgent(
  model: LanguageModel,
  modelInfo: ModelInfo,
  options?: {
    enabledTools?: string[] | undefined;
    disabledInstructions?: boolean | undefined;
    temperature?: number | undefined;
    top_p?: number | undefined;
    top_k?: number | undefined;
    frequency_penalty?: number | undefined;
    presence_penalty?: number | undefined;
  },
): Agent<MyContext> {
  const enabledTools = options?.enabledTools;
  const toolNameSet = enabledTools ? new Set(enabledTools) : null;
  const tools =
    toolNameSet === null
      ? availableTools
      : availableTools.filter((tool) => toolNameSet.has(tool.name));
  const agentInstructions = options?.disabledInstructions ? [] : instructions;

  return new Agent<MyContext>({
    name: "MyAgent",
    instructions: agentInstructions,
    model,
    tools,
    max_turns: 5,
    ...(options?.temperature !== undefined && {
      temperature: options.temperature,
    }),
    ...(options?.top_p !== undefined && { top_p: options.top_p }),
    ...(options?.top_k !== undefined && { top_k: options.top_k }),
    ...(options?.frequency_penalty !== undefined && {
      frequency_penalty: options.frequency_penalty,
    }),
    ...(options?.presence_penalty !== undefined && {
      presence_penalty: options.presence_penalty,
    }),
    ...(modelInfo.modalities && { modalities: modelInfo.modalities }),
    ...(modelInfo.audio && { audio: modelInfo.audio }),
    ...(modelInfo.reasoning && { reasoning: modelInfo.reasoning }),
  });
}
