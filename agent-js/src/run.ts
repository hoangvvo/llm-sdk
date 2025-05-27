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
import { type AgentTool } from "./tool.ts";
import type { ToolkitSession } from "./toolkit.ts";
import type {
  AgentItem,
  AgentItemModelResponse,
  AgentItemTool,
  AgentResponse,
  AgentStreamEvent,
} from "./types.ts";

/**
 * Manages the run session for an agent run.
 * It initializes all necessary components for the agent to run
 * and handles the execution of the agent's tasks.
 * Once closed, the session cleans up any resources used during the run.
 * The session can be reused in multiple runs.
 *
 * A RunSession binds to a specific context value, which is used to resolve
 * instructions and passed to tool executions. Input items remain per run and are
 * supplied when invoking `run` or `runStream`.
 */
export class RunSession<TContext> {
  #initialized: boolean;
  #params: AgentParamsWithDefaults<TContext>;
  #context: TContext;
  /**
   * The system prompt generated from the params instructions.
   */
  #static_system_prompt?: string;
  /**
   * The tools provided from params.
   */
  #static_tools: AgentTool<TContext>[];
  /**
   * The toolkit sessions created for the run session.
   */
  #toolkit_sessions: ToolkitSession<TContext>[];

  constructor({ context, ...agentParams }: RunSessionParams<TContext>) {
    this.#initialized = false;
    this.#params = agentParamsWithDefaults(agentParams);
    this.#context = context;
    this.#static_tools = [...this.#params.tools];
    this.#toolkit_sessions = [];
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

  // Initialize any resources needed for the run session
  async #initialize() {
    // Resolve the instructions using the provided context
    if (this.#params.instructions.length > 0) {
      const systemPrompt = await getPromptForInstructionParams(
        this.#params.instructions,
        this.#context,
      );
      this.#static_system_prompt = systemPrompt;
    }

    if (this.#params.toolkits.length > 0) {
      try {
        this.#toolkit_sessions = await Promise.all(
          this.#params.toolkits.map((toolkit) =>
            toolkit.createSession(this.#context),
          ),
        );
      } catch (error) {
        throw new AgentInitError(error);
      }
    }

    this.#initialized = true;
  }

  /**
   * Process the model response and decide whether to continue the loop or
   * return the response
   */
  async *#process(
    state: RunState,
    content: Part[],
    tools: AgentTool<TContext>[],
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
      const agentTool = tools.find(
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
            return await agentTool.execute(
              toolCallPart.args,
              this.#context,
              state,
            );
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
  async run(request: RunSessionRequest): Promise<AgentResponse> {
    if (!this.#initialized) {
      throw new AgentInvariantError("RunSession not initialized.");
    }

    const span = new AgentSpan(this.#params.name, "run");

    try {
      const state = new RunState(request.input, this.#params.max_turns);

      for (;;) {
        const { input: languageModelInput, tools } = this.#getTurnParams(state);

        const modelResponse: ModelResponse = await span.withContext(
          async () => {
            try {
              const response =
                await this.#params.model.generate(languageModelInput);
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

        const processStream = this.#process(state, content, tools);
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
    request: RunSessionRequest,
  ): AsyncGenerator<AgentStreamEvent, AgentResponse> {
    if (!this.#initialized) {
      throw new AgentInvariantError("RunSession not initialized.");
    }

    const span = new AgentSpan(this.#params.name, "run_stream");

    try {
      const state = new RunState(request.input, this.#params.max_turns);

      for (;;) {
        const { input: languageModelInput, tools } = this.#getTurnParams(state);

        const modelStream = this.#params.model.stream(languageModelInput);
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

        const { item, index } = state.appendModelResponse(response);
        yield {
          event: "item",
          index,
          item,
        };

        const { content } = response;

        const processStream = this.#process(state, content, tools);
        // Patch next to propogate tracing context
        // See: https://github.com/open-telemetry/opentelemetry-js/issues/2951
        const originalProcessStreamNext =
          processStream.next.bind(processStream);
        processStream.next = (...args) =>
          span.withContext(() => originalProcessStreamNext(...args));

        for await (const event of processStream) {
          if (event.type === "item") {
            const index = state.appendItem(event.item);
            yield {
              event: "item",
              item: event.item,
              index,
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
  async close(): Promise<void> {
    if (!this.#initialized) {
      return;
    }
    this.#static_system_prompt = "";
    this.#static_tools = [];

    await Promise.all(this.#toolkit_sessions.map((session) => session.close()));
    this.#toolkit_sessions = [];

    this.#initialized = false;
  }

  /**
   * Compute the current snapshot of system prompt and tools from all toolkits
   * as well as the base LanguageModelInput to be passed to the model.
   */
  #getTurnParams(state: RunState): {
    /**
     * The system prompt to use for the model input.
     */
    input: LanguageModelInput;
    /**
     * The tools to use for the model input.
     */
    tools: AgentTool<TContext>[];
  } {
    try {
      const input: LanguageModelInput = {
        messages: state.getTurnMessages(),
        response_format: this.#params.response_format,
      };

      // Add static system prompt and tools
      const systemPrompts: string[] = [];
      if (this.#static_system_prompt && this.#static_system_prompt.length > 0) {
        systemPrompts.push(this.#static_system_prompt);
      }
      const tools = [...this.#static_tools];

      // Add toolkit prompts and tools
      for (const toolkitSession of this.#toolkit_sessions) {
        const toolkitPrompt = toolkitSession.getSystemPrompt();
        if (toolkitPrompt?.length) {
          systemPrompts.push(toolkitPrompt);
        }
        const toolkitTools = toolkitSession.getTools();
        if (toolkitTools.length > 0) {
          tools.push(...toolkitTools);
        }
      }

      // Add system prompt and tools to language model input if available
      if (systemPrompts.length > 0) {
        input.system_prompt = systemPrompts.join("\n");
      }
      if (tools.length > 0) {
        input.tools = tools.map((tool) => ({
          name: tool.name,
          description: tool.description,
          parameters: tool.parameters,
        }));
      }

      // Add other model params
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

      return { input, tools };
    } catch (err) {
      throw new AgentInitError(err);
    }
  }
}

/**
 * RunSessionParams represents the parameters needed to create a RunSession.
 * It extends AgentParams to also include context for the agent.
 */
export interface RunSessionParams<TContext> extends AgentParams<TContext> {
  /**
   * The context used to resolve instructions and passed to tool executions.
   */
  context: TContext;
}

/**
 * RunSessionRequest contains the input used for a run.
 */
export interface RunSessionRequest {
  input: AgentItem[];
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
   * Add AgentItems to the run state.
   * Returns the index of the added item in the output array.
   */
  appendItem(item: AgentItem): number {
    this.#output.push(item);
    return this.#output.length - 1;
  }

  // Append a model response to the run state as an AgentItemModelResponse
  // and return such the item and its index in the output array
  appendModelResponse(response: ModelResponse): {
    item: AgentItem;
    index: number;
  } {
    const item: AgentItemModelResponse = {
      type: "model",
      ...response,
    };
    this.#output.push(item);
    return {
      item,
      index: this.#output.length - 1,
    };
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
