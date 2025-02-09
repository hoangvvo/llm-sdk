import {
  LanguageModelError,
  StreamAccumulator,
  type LanguageModel,
  type LanguageModelInput,
  type Message,
  type ModelResponse,
  type Part,
  type ResponseFormatOption,
  type ToolMessage,
} from "@hoangvvo/llm-sdk";
import {
  AgentInvariantError,
  AgentLanguageModelError,
  AgentMaxTurnsExceededError,
  AgentToolExecutionError,
} from "./errors.ts";
import {
  getPromptForInstructionParams,
  type InstructionParam,
} from "./instruction.ts";
import { AgentSpan, startActiveToolSpan } from "./opentelemetry.ts";
import type { AgentTool } from "./tool.ts";
import type {
  AgentItem,
  AgentItemMessage,
  AgentRequest,
  AgentResponse,
  AgentStreamEvent,
  ModelCallInfo,
} from "./types.ts";

/**
 * Manages the run session for an agent run.
 * It initializes all necessary components for the agent to run
 * and handles the execution of the agent's tasks.
 * Once finished, the session cleans up any resources used during the run.
 * The session can be reused in multiple runs.
 */
export class RunSession<TContext> {
  readonly #agentName: string;
  readonly #instructions: InstructionParam<TContext>[];
  readonly #model: LanguageModel;
  readonly #responseFormat: ResponseFormatOption;
  readonly #tools: AgentTool<TContext>[];
  readonly #maxTurns: number;
  readonly #temperature: number | undefined;
  readonly #topP: number | undefined;
  readonly #topK: number | undefined;
  readonly #presencePenalty: number | undefined;
  readonly #frequencyPenalty: number | undefined;

  #initialized: boolean;

  constructor({
    agentName,
    instructions,
    model,
    responseFormat,
    tools,
    maxTurns,
    temperature,
    topP,
    topK,
    presencePenalty,
    frequencyPenalty,
  }: RunSessionParams<TContext>) {
    this.#agentName = agentName;
    this.#instructions = instructions;
    this.#model = model;
    this.#responseFormat = responseFormat;
    this.#tools = tools;
    this.#maxTurns = maxTurns;
    this.#temperature = temperature;
    this.#topP = topP;
    this.#topK = topK;
    this.#presencePenalty = presencePenalty;
    this.#frequencyPenalty = frequencyPenalty;

    this.#initialized = false;
  }

  /**
   * Create a new run session and initializes dependencies
   */
  static async create<TContext>(
    params: RunSessionParams<TContext>,
  ): Promise<RunSession<TContext>> {
    const session = new RunSession(params);
    await session.#initialize();
    return session;
  }

  async #initialize() {
    // Initialize any resources needed for the run session
    this.#initialized = true;
    return Promise.resolve();
  }

  /**
   * Process the model response and decide whether to continue the loop or
   * return the response
   */
  async #process(
    context: TContext,
    state: RunState,
    content: Part[],
  ): Promise<ProcessResult> {
    const toolCallParts = content.filter((part) => part.type === "tool-call");

    if (!toolCallParts.length) {
      return {
        type: "response",
        content,
      };
    }

    const toolMessage: ToolMessage = {
      role: "tool",
      content: [],
    };

    for (const toolCallPart of toolCallParts) {
      const agentTool = this.#tools.find(
        (tool) => tool.name === toolCallPart.tool_name,
      );

      if (!agentTool) {
        throw new AgentInvariantError(
          `Tool ${toolCallPart.tool_name} not found in agent`,
        );
      }

      const toolRes = await startActiveToolSpan(
        toolCallPart.tool_call_id,
        agentTool.name,
        agentTool.description,
        async () => {
          try {
            return await agentTool.execute(toolCallPart.args, context, state);
          } catch (e) {
            throw new AgentToolExecutionError(e);
          }
        },
      );

      toolMessage.content.push({
        type: "tool-result",
        tool_name: toolCallPart.tool_name,
        tool_call_id: toolCallPart.tool_call_id,
        content: toolRes.content,
        is_error: toolRes.is_error,
      });
    }

    return {
      type: "next",
      messages: [toolMessage],
    };
  }

  /**
   * Run a non-streaming execution of the agent.
   */
  async run(request: AgentRequest<TContext>): Promise<AgentResponse> {
    if (!this.#initialized) {
      throw new Error("RunSession not initialized.");
    }

    const span = new AgentSpan(this.#agentName, "run");

    try {
      const state = new RunState(request.input, this.#maxTurns);

      const input = span.withContext(() => this.#getLlmInput(request));
      const context = request.context;

      for (;;) {
        const modelResponse: ModelResponse = await span.withContext(
          async () => {
            try {
              const response = await this.#model.generate({
                ...input,
                messages: state.getTurnMessages(),
              });
              return response;
            } catch (err) {
              if (err instanceof LanguageModelError) {
                throw new AgentLanguageModelError(err);
              }
              throw err;
            }
          },
        );

        const { content, usage, cost } = modelResponse;

        state.appendMessages([
          {
            role: "assistant",
            content,
          },
        ]);

        state.appendModelCall({
          usage: usage ?? null,
          cost: cost ?? null,
          model_id: this.#model.modelId,
          provider: this.#model.provider,
        });

        const processResult = await span.withContext(() =>
          this.#process(context, state, content),
        );
        if (processResult.type === "response") {
          const response = state.createResponse(processResult.content);
          span.onResponse(response);
          return response;
        } else {
          for (const message of processResult.messages) {
            state.appendMessages([message]);
          }
        }

        state.turn();
      }
    } catch (err) {
      span.onError(err);
      throw err;
    } finally {
      span.onEnd();
    }
  }

  /**
   * Run a streaming execution of the agent.
   */
  async *runStream(
    request: AgentRequest<TContext>,
  ): AsyncGenerator<AgentStreamEvent, AgentResponse> {
    if (!this.#initialized) {
      throw new Error("RunSession not initialized.");
    }

    const span = new AgentSpan(this.#agentName, "run_stream");

    try {
      const state = new RunState(request.input, this.#maxTurns);

      const input = span.withContext(() => this.#getLlmInput(request));
      const context = request.context;

      for (;;) {
        const modelStream = this.#model.stream({
          ...input,
          messages: state.getTurnMessages(),
        });
        // Patch next to propogate tracing context
        // See: https://github.com/open-telemetry/opentelemetry-js/issues/2951
        const originalNext = modelStream.next.bind(modelStream);
        modelStream.next = (...args) =>
          span.withContext(() => originalNext(...args));

        const accumulator = new StreamAccumulator();

        try {
          for await (const partial of modelStream) {
            accumulator.addPartial(partial);
            yield {
              type: "partial",
              ...partial,
            };
          }
        } catch (err) {
          if (err instanceof LanguageModelError) {
            throw new AgentLanguageModelError(err);
          }
          throw err;
        }

        const { content, cost, usage } = accumulator.computeResponse();

        const assistantMessage: Message = {
          role: "assistant",
          content,
        };
        state.appendMessages([assistantMessage]);

        state.appendModelCall({
          usage: usage ?? null,
          cost: cost ?? null,
          model_id: this.#model.modelId,
          provider: this.#model.provider,
        });

        yield {
          type: "message",
          ...assistantMessage,
        };

        const processResult = await span.withContext(() =>
          this.#process(context, state, content),
        );

        if (processResult.type === "response") {
          const response = state.createResponse(processResult.content);
          yield {
            type: "response",
            ...response,
          };
          span.onResponse(response);
          return response;
        } else {
          for (const message of processResult.messages) {
            state.appendMessages([message]);
            yield {
              type: "message",
              ...message,
            };
          }
        }

        state.turn();
      }
    } catch (err) {
      span.onError(err);
      throw err;
    } finally {
      span.onEnd();
    }
  }

  /**
   * Finalize any resources or state for the run session
   */
  async finish(): Promise<void> {
    this.#initialized = false;
    return Promise.resolve();
  }

  #getLlmInput(request: AgentRequest<TContext>): LanguageModelInput {
    const systemPrompt = getPromptForInstructionParams(
      this.#instructions,
      request.context,
    );

    const input: LanguageModelInput = {
      // messages will be computed from getTurnMessages
      messages: [],
      system_prompt: systemPrompt,
      tools: this.#tools.map((tool) => ({
        name: tool.name,
        description: tool.description,
        parameters: tool.parameters,
      })),
      response_format: this.#responseFormat,
    };

    if (this.#temperature !== undefined) {
      input.temperature = this.#temperature;
    }
    if (this.#topP !== undefined) {
      input.top_p = this.#topP;
    }
    if (this.#topK !== undefined) {
      input.top_k = this.#topK;
    }
    if (this.#presencePenalty !== undefined) {
      input.presence_penalty = this.#presencePenalty;
    }
    if (this.#frequencyPenalty !== undefined) {
      input.frequency_penalty = this.#frequencyPenalty;
    }

    return input;
  }
}

interface RunSessionParams<TContext> {
  agentName: string;
  model: LanguageModel;
  instructions: InstructionParam<TContext>[];
  tools: AgentTool<TContext>[];
  responseFormat: ResponseFormatOption;
  maxTurns: number;
  temperature?: number;
  topP?: number;
  topK?: number;
  presencePenalty?: number;
  frequencyPenalty?: number;
}

type ProcessResult = ProcessResultResponse | ProcessResultNext;

interface ProcessResultResponse {
  type: "response";
  content: Part[];
}

interface ProcessResultNext {
  type: "next";
  /**
   * Messages generated by the process() calls
   */
  messages: Message[];
}

export class RunState {
  readonly #maxTurns: number;
  readonly #input: AgentItem[];

  /**
   * The current turn number in the run.
   */
  currentTurn: number;

  /**
   * All items generated during the run, such as new ToolMessage and AssistantMessage
   */
  readonly #output: AgentItem[];
  /**
   * Information about the LLM calls made during the run
   */
  readonly #modelCalls: ModelCallInfo[];

  constructor(input: AgentItem[], maxTurns: number) {
    this.#input = input;
    this.#maxTurns = maxTurns;

    this.currentTurn = 0;
    this.#output = [];
    this.#modelCalls = [];
  }

  /**
   * Mark a new turn in the conversation and throw an error if max turns exceeded.
   */
  turn() {
    this.currentTurn++;
    if (this.currentTurn > this.#maxTurns) {
      throw new AgentMaxTurnsExceededError(this.#maxTurns);
    }
  }

  /**
   * Add a message to the run state.
   */
  appendMessages(messages: Message[]) {
    this.#output.push(
      ...messages.map(
        (message): AgentItemMessage => ({ type: "message", ...message }),
      ),
    );
  }

  /**
   * Add a model call to the run state.
   */
  appendModelCall(call: ModelCallInfo) {
    this.#modelCalls.push(call);
  }

  /**
   * Get LLM messages to use in the LanguageModelInput for the turn
   */
  getTurnMessages(): Message[] {
    return [...this.#input, ...this.#output];
  }

  /**
   * Create the Agent Response
   */
  createResponse(finalContent: Part[]): AgentResponse {
    return {
      content: finalContent,
      output: this.#output,
      model_calls: this.#modelCalls,
    };
  }
}
