/* eslint-disable @typescript-eslint/no-explicit-any */
import type { LanguageModel, ResponseFormatOption } from "@hoangvvo/llm-sdk";
import type { InstructionParam } from "./instruction.ts";
import { RunSession } from "./run.ts";
import type { AgentTool } from "./tool.ts";
import type { AgentRequest, AgentResponse, AgentStreamEvent } from "./types.ts";

export class Agent<TContext> {
  /**
   * A unique name for the agent.
   */
  readonly name: string;
  readonly #instructions: InstructionParam<TContext>[];
  readonly #model: LanguageModel;
  readonly #response_format: ResponseFormatOption;
  readonly #tools: AgentTool<any, TContext>[];
  readonly #max_turns: number;
  readonly #temperature: number | undefined;
  readonly #top_p: number | undefined;
  readonly #top_k: number | undefined;
  readonly #presence_penalty: number | undefined;
  readonly #frequency_penalty: number | undefined;

  constructor(inputParams: AgentParams<TContext>) {
    const params = agentParamsWithDefaults(inputParams);
    this.name = params.name;
    this.#instructions = params.instructions;
    this.#model = params.model;
    this.#response_format = params.response_format;
    this.#tools = params.tools;
    this.#max_turns = params.max_turns;
    this.#temperature = params.temperature;
    this.#top_p = params.top_p;
    this.#top_k = params.top_k;
    this.#presence_penalty = params.presence_penalty;
    this.#frequency_penalty = params.frequency_penalty;
  }

  /**
   * Create a one-time run of the agent and generate a response.
   * A session is created for the run and cleaned up afterwards.
   */
  async run(request: AgentRequest<TContext>): Promise<AgentResponse> {
    const runSession = await this.createSession();
    const result = runSession.run(request);
    await runSession.finish();
    return result;
  }

  /**
   * Create a one-time streaming run of the agent and generate a response.
   * A session is created for the run and cleaned up afterwards.
   */
  async *runStream(
    request: AgentRequest<TContext>,
  ): AsyncGenerator<AgentStreamEvent, AgentResponse> {
    const runSession = await this.createSession();
    const stream = runSession.runStream(request);

    let current = await stream.next();
    while (!current.done) {
      yield current.value;
      current = await stream.next();
    }

    await runSession.finish();

    return current.value;
  }

  /**
   * Create a session for stateful multiple runs of the agent
   */
  async createSession(): Promise<RunSession<TContext>> {
    return RunSession.create(
      this.#model,
      this.#instructions,
      this.#tools,
      this.#response_format,
      this.#max_turns,
      this.#temperature,
      this.#top_p,
      this.#top_k,
      this.#presence_penalty,
      this.#frequency_penalty,
    );
  }
}

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
  tools?: AgentTool<any, TContext>[];

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
}

type AgentParamsWithDefaults<TContext> = Omit<
  Required<AgentParams<TContext>>,
  "temperature" | "top_p" | "top_k" | "presence_penalty" | "frequency_penalty"
> &
  Pick<
    AgentParams<TContext>,
    "temperature" | "top_p" | "top_k" | "presence_penalty" | "frequency_penalty"
  >;

function agentParamsWithDefaults<TContext>(
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
