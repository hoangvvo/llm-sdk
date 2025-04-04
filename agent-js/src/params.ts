import type {
  AudioOptions,
  LanguageModel,
  Modality,
  ReasoningOptions,
  ResponseFormatOption,
} from "@hoangvvo/llm-sdk";
import type { InstructionParam } from "./instruction.ts";
import type { AgentTool } from "./tool.ts";

/**
 * Parameters required to create a new agent.
 */
export interface AgentParams<TContext> {
  /**
   * A unique name for the agent.
   */
  name: string;

  /**
   * The default language model to use for the agent.
   */
  model: LanguageModel;

  /**
   * Instructions to be added to system messages when executing the agent.
   * This can include formatting instructions or other guidance for the agent.
   * @default []
   */
  instructions?: InstructionParam<TContext>[];

  /**
   * The tools that the agent can use to perform tasks.
   * @default []
   */
  tools?: AgentTool<TContext>[];

  /**
   * The expected format of the response. Either text or json.
   * @default { type: "text" }
   */
  response_format?: ResponseFormatOption;

  /**
   * Max number of turns for agent to run to protect against infinite loops.
   * @default 10
   */
  max_turns?: number;

  /**
   * Amount of randomness injected into the response. Ranges from 0.0 to 1.0
   * @default undefined
   */
  temperature?: number;

  /**
   * An alternative to sampling with temperature, called nucleus sampling,
   * where the model considers the results of the tokens with top_p probability mass.
   * Ranges from 0.0 to 1.0
   * @default undefined
   */
  top_p?: number;

  /**
   * Only sample from the top K options for each subsequent token.
   * Used to remove 'long tail' low probability responses.
   * Must be a non-negative integer.
   * @default undefined
   */
  top_k?: number;

  /**
   * Positive values penalize new tokens based on whether they appear in the text so far,
   * increasing the model's likelihood to talk about new topics.
   * @default undefined
   */
  presence_penalty?: number;

  /**
   * Positive values penalize new tokens based on their existing frequency in the text so far,
   * decreasing the model's likelihood to repeat the same line verbatim.
   * @default undefined
   */
  frequency_penalty?: number;

  /**
   * The modalities that the model should support.
   * @default undefined
   */

  modalities?: Modality[];

  /**
   * Options for audio generation.
   * @default undefined
   */
  audio?: AudioOptions;

  /**
   * Options for reasoning generation.
   * @default undefined
   */
  reasoning?: ReasoningOptions;
}

export type AgentParamsWithDefaults<TContext> = AgentParams<TContext> &
  Required<
    Pick<
      AgentParams<TContext>,
      "max_turns" | "tools" | "instructions" | "response_format"
    >
  >;

export function agentParamsWithDefaults<TContext>(
  params: AgentParams<TContext>,
): AgentParamsWithDefaults<TContext> {
  return {
    instructions: [],
    tools: [],
    response_format: {
      type: "text",
    },
    max_turns: 10,
    ...params,
  };
}
