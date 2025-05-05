import {
  LanguageModelError,
  StreamAccumulator,
  type LanguageModelInput,
  type Message,
  type ModelResponse,
  type Part,
} from "@hoangvvo/llm-sdk";
import {
  AgentInitError,
  AgentInvariantError,
  AgentLanguageModelError,
  AgentMaxTurnsExceededError,
  AgentToolExecutionError,
} from "./errors.ts";
import { getPromptForInstructionParams } from "./instruction.ts";
import { AgentSpan, startActiveToolSpan } from "./opentelemetry.ts";
import {
  agentParamsWithDefaults,
  type AgentParams,
  type AgentParamsWithDefaults,
} from "./params.ts";
import type {
  AgentItem,
  AgentItemModelResponse,
  AgentItemTool,
  AgentRequest,
  AgentResponse,
  AgentStreamEvent,
} from "./types.ts";

/**
 * Manages the run session for an agent run.
 * It initializes all necessary components for the agent to run
 * and handles the execution of the agent's tasks.
 * Once finished, the session cleans up any resources used during the run.
 * The session can be reused in multiple runs.
 */
export class RunSession<TContext> {
  #initialized: boolean;
  #params: AgentParamsWithDefaults<TContext>;

  constructor(inputParams: AgentParams<TContext>) {
    this.#initialized = false;
    this.#params = agentParamsWithDefaults(inputParams);
  }

  /**
   * Create a new run session and initializes dependencies
   */
  static async create<TContext>(
    params: AgentParams<TContext>,
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
  async *#process(
    context: TContext,
    state: RunState,
    content: Part[],
  ): AsyncGenerator<ProcessEvents> {
    const toolCallParts = content.filter((part) => part.type === "tool-call");

    if (!toolCallParts.length) {
      yield {
        type: "response",
        content,
      };
      return;
    }

    for (const toolCallPart of toolCallParts) {
      const agentTool = this.#params.tools.find(
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

      const agentItemTool: AgentItemTool = {
        type: "tool",
        tool_name: toolCallPart.tool_name,
        tool_call_id: toolCallPart.tool_call_id,
        input: toolCallPart.args,
        output: toolRes.content,
        is_error: toolRes.is_error,
      };

      yield {
        type: "item",
        item: agentItemTool,
      };
    }

    yield {
      type: "next",
    };
  }

  /**
   * Run a non-streaming execution of the agent.
   */
  async run(request: AgentRequest<TContext>): Promise<AgentResponse> {
    if (!this.#initialized) {
      throw new Error("RunSession not initialized.");
    }

    const span = new AgentSpan(this.#params.name, "run");

    try {
      const state = new RunState(request.input, this.#params.max_turns);

      const input = await span.withContext(() => this.#getLlmInput(request));
      const context = request.context;

      for (;;) {
        const modelResponse: ModelResponse = await span.withContext(
          async () => {
            try {
              const response = await this.#params.model.generate({
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

        const { content } = modelResponse;

        state.appendModelResponse(modelResponse);

        const processStream = this.#process(context, state, content);
        // Patch next to propogate tracing context
        // See: https://github.com/open-telemetry/opentelemetry-js/issues/2951
        const originalNext = processStream.next.bind(processStream);
        processStream.next = (...args) =>
          span.withContext(() => originalNext(...args));

        for await (const event of processStream) {
          if (event.type === "item") {
            state.appendItem(event.item);
          }
          if (event.type === "response") {
            const response = state.createResponse(event.content);
            span.onResponse(response);
            return response;
          }
          if (event.type === "next") {
            // continue the loop
            state.turn();
            break;
          }
        }
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

    const span = new AgentSpan(this.#params.name, "run_stream");

    try {
      const state = new RunState(request.input, this.#params.max_turns);

      const input = await span.withContext(() => this.#getLlmInput(request));
      const context = request.context;

      for (;;) {
        const modelStream = this.#params.model.stream({
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
              event: "partial",
              ...partial,
            };
          }
        } catch (err) {
          if (err instanceof LanguageModelError) {
            throw new AgentLanguageModelError(err);
          }
          throw err;
        }

        const response = accumulator.computeResponse();

        const item = state.appendModelResponse(response);
        yield {
          event: "item",
          ...item,
        };

        const { content } = response;

        const processStream = this.#process(context, state, content);
        // Patch next to propogate tracing context
        // See: https://github.com/open-telemetry/opentelemetry-js/issues/2951
        const originalProcessStreamNext =
          processStream.next.bind(processStream);
        processStream.next = (...args) =>
          span.withContext(() => originalProcessStreamNext(...args));

        for await (const event of processStream) {
          if (event.type === "item") {
            state.appendItem(event.item);
            yield {
              event: "item",
              ...event.item,
            };
          }
          if (event.type === "response") {
            const response = state.createResponse(event.content);
            span.onResponse(response);
            yield {
              event: "response",
              ...response,
            };
            return response;
          }
          if (event.type === "next") {
            // continue the loop
            state.turn();
            break;
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

  async #getLlmInput(
    request: AgentRequest<TContext>,
  ): Promise<LanguageModelInput> {
    try {
      const input: LanguageModelInput = {
        // messages will be computed from getTurnMessages
        messages: [],
        response_format: this.#params.response_format,
      };

      if (this.#params.instructions.length > 0) {
        const systemPrompt = await getPromptForInstructionParams(
          this.#params.instructions,
          request.context,
        );
        input.system_prompt = systemPrompt;
      }

      if (this.#params.tools.length > 0) {
        input.tools = this.#params.tools.map((tool) => ({
          name: tool.name,
          description: tool.description,
          parameters: tool.parameters,
        }));
      }

      if (this.#params.temperature !== undefined) {
        input.temperature = this.#params.temperature;
      }
      if (this.#params.top_p !== undefined) {
        input.top_p = this.#params.top_p;
      }
      if (this.#params.top_k !== undefined) {
        input.top_k = this.#params.top_k;
      }
      if (this.#params.presence_penalty !== undefined) {
        input.presence_penalty = this.#params.presence_penalty;
      }
      if (this.#params.frequency_penalty !== undefined) {
        input.frequency_penalty = this.#params.frequency_penalty;
      }
      if (this.#params.modalities !== undefined) {
        input.modalities = this.#params.modalities;
      }
      if (this.#params.audio !== undefined) {
        input.audio = this.#params.audio;
      }
      if (this.#params.reasoning !== undefined) {
        input.reasoning = this.#params.reasoning;
      }

      return input;
    } catch (err) {
      throw new AgentInitError(err);
    }
  }
}

/**
 * ProcessEvents represents the sum type of events returned by the process function.
 */
type ProcessEvents = ProcessEventResponse | ProcessEventNext | ProcessEventItem;

/**
 * Emit when a new item is generated
 */
interface ProcessEventItem {
  type: "item";
  item: AgentItem;
}

/**
 * Emit when the final response is ready
 */
interface ProcessEventResponse {
  type: "response";
  content: Part[];
}

/**
 * Emit when the loop should continue to the next iteration
 */
interface ProcessEventNext {
  type: "next";
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

  constructor(input: AgentItem[], maxTurns: number) {
    this.#input = input;
    this.#maxTurns = maxTurns;

    this.currentTurn = 0;
    this.#output = [];
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

  // Return all items in the run, both input and output
  getItems(): AgentItem[] {
    return [...this.#input, ...this.#output];
  }

  /**
   * Add AgentItems to the run state
   */
  appendItem(item: AgentItem): void {
    this.#output.push(item);
  }

  // Append a model response to the run state as an AgentItemModelResponse
  // and return such the item
  appendModelResponse(response: ModelResponse): AgentItem {
    const item: AgentItemModelResponse = {
      type: "model",
      ...response,
    };
    this.#output.push(item);
    return item;
  }

  /**
   * Get LLM messages to use in the LanguageModelInput for the turn
   */
  getTurnMessages(): Message[] {
    const messages: Message[] = [];
    const items = this.getItems();
    for (const item of items) {
      switch (item.type) {
        case "message":
          messages.push({
            role: item.role,
            content: item.content,
          });
          break;
        // While each tool call is an individual item, merge them into a single
        // tool message as per llm-sdk's expectation
        case "tool": {
          const toolResultPart: Part = {
            type: "tool-result",
            tool_name: item.tool_name,
            tool_call_id: item.tool_call_id,
            content: item.output,
            is_error: item.is_error,
          };

          const lastMessage = messages[messages.length - 1];
          if (!lastMessage || lastMessage.role !== "tool") {
            messages.push({
              role: "tool",
              content: [toolResultPart],
            });
          } else {
            lastMessage.content.push(toolResultPart);
          }

          break;
        }
        case "model":
          messages.push({
            role: "assistant",
            content: item.content,
          });
          break;
      }
    }
    return messages;
  }

  /**
   * Create the Agent Response
   */
  createResponse(finalContent: Part[]): AgentResponse {
    return {
      content: finalContent,
      output: this.#output,
    };
  }
}
