import type { AgentParams } from "./params.ts";
import { AgentInvariantError } from "./errors.ts";
import { RunSession, type RunOptions } from "./run.ts";
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
  async run(
    { input, context }: AgentRequest<TContext>,
    options?: RunOptions,
  ): Promise<AgentResponse> {
    const runSession = await this.createSession(context);
    let response: AgentResponse;
    try {
      response = await runSession.run({ input }, options);
    } catch (error) {
      try {
        await runSession.close();
      } catch {
        // Preserve the primary run error when cleanup also fails.
      }
      throw error;
    }
    await runSession.close();
    return response;
  }

  /**
   * Create a one-time streaming run of the agent and generate a response.
   * A session is created for the run and cleaned up afterwards.
   */
  async *runStream(
    { input, context }: AgentRequest<TContext>,
    options?: RunOptions,
  ): AsyncGenerator<AgentStreamEvent, AgentResponse> {
    const runSession = await this.createSession(context);
    let response: AgentResponse | undefined;
    let streamFailed = false;
    try {
      const stream = runSession.runStream({ input }, options);
      for await (const event of stream) {
        if (event.event === "response") {
          response = {
            content: event.content,
            output: event.output,
            status: event.status,
          };
        } else {
          yield event;
        }
      }
    } catch (error) {
      streamFailed = true;
      throw error;
    } finally {
      if (!streamFailed) {
        await runSession.close();
      } else {
        try {
          await runSession.close();
        } catch {
          // Preserve the primary stream error when cleanup also fails.
        }
      }
    }

    if (!response) {
      throw new AgentInvariantError("Agent stream ended without a response.");
    }
    yield { event: "response", ...response };
    return response;
  }

  /**
   * Create a session for stateful multiple runs of the agent
   */
  async createSession(context: TContext): Promise<RunSession<TContext>> {
    return RunSession.create({ context, ...this.#params });
  }
}
