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

  constructor(inputParams: AgentParams<TContext>) {
    const params = agentParamsWithDefaults(inputParams);
    this.name = params.name;
    this.#instructions = params.instructions;
    this.#model = params.model;
    this.#response_format = params.response_format;
    this.#tools = params.tools;
    this.#max_turns = params.max_turns;
  }

  /**
   * Create a stateless one-time run of the agent
   */
  async run(request: AgentRequest<TContext>): Promise<AgentResponse> {
    const runSession = await this.createSession();
    const result = runSession.run(request);
    await runSession.finish();
    return result;
  }

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
}

function agentParamsWithDefaults<TContext>(
  params: AgentParams<TContext>,
): Required<AgentParams<TContext>> {
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
