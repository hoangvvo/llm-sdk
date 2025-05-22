import type { AgentParams } from "./params.ts";
import { RunSession } from "./run.ts";
import type { AgentRequest, AgentResponse, AgentStreamEvent } from "./types.ts";

export class Agent<TContext> {
  /**
   * A unique name for the agent.
   */
  readonly name: string;
  readonly #params: AgentParams<TContext>;

  constructor(params: AgentParams<TContext>) {
    this.name = params.name;
    this.#params = params;
  }

  /**
   * Create a one-time run of the agent and generate a response.
   * A session is created for the run and cleaned up afterwards.
   */
  async run({
    input,
    context,
  }: AgentRequest<TContext>): Promise<AgentResponse> {
    const runSession = await this.createSession(context);
    const result = runSession.run({ input });
    await runSession.finish();
    return result;
  }

  /**
   * Create a one-time streaming run of the agent and generate a response.
   * A session is created for the run and cleaned up afterwards.
   */
  async *runStream({
    input,
    context,
  }: AgentRequest<TContext>): AsyncGenerator<AgentStreamEvent, AgentResponse> {
    const runSession = await this.createSession(context);
    const stream = runSession.runStream({ input });

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
  async createSession(context: TContext): Promise<RunSession<TContext>> {
    return RunSession.create({ context, ...this.#params });
  }
}
