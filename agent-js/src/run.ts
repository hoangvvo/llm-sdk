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
   * process() flow:
   * 1. Peek latest run item to locate assistant content.
   *    1a. Tail is user message → emit `next`. Go to 3.
   *    1b. Tail is tool/tool message → gather processed ids, backtrack to assistant/model content. Go to 2.
   *    1c. Tail is assistant/model → use its content. Go to 2.
   * 2. Scan assistant content for tool calls.
   *    2a. Tool calls remaining → execute unprocessed tools, emit each as `item`, then emit `next`. Go to 3.
   *    2b. No tool calls → emit `response`. Go to 4.
   * 3. Outer loop: bump turn, refresh params, request model response, append it, then re-enter step 1.
   * 4. Return final response to caller.
   */
  async *#process(
    state: RunState,
    tools: AgentTool<TContext>[],
  ): AsyncGenerator<ProcessEvents> {
    // Examining the last items in the state determines the next step.
    const allItems = state.getItems();
    const lastItem = allItems.at(-1);
    if (!lastItem) {
      throw new AgentInvariantError("No items in the run state.");
    }

    let content: Part[] | null = null;
    const processedToolCallIds = new Set<string>();
    if (lastItem.type === "model") {
      // ========== Case: Assistant Message [from AgentItemModelResponse] ==========
      // Last item is a model response, process it
      content = lastItem.content;
    } else if (lastItem.type === "message") {
      if (lastItem.role === "assistant") {
        // ========== Case: Assistant Message [from AgentItemMessage] ==========
        // Last item is an assistant message, process it
        content = lastItem.content;
      } else if (lastItem.role === "user") {
        // ========== Case: User Message ==========
        // last item is a user message, so we need to generate a model response
        yield {
          type: "next",
        };
        return;
      } else {
        // ========== Case: Tool Results (from AgentItemMessage) ==========
        // Track the tool call ids that have been processed to avoid duplicate execution
        for (const part of lastItem.content) {
          if (part.type === "tool-result") {
            processedToolCallIds.add(part.tool_call_id);
          }
        }

        // We are in the middle of processing tool results, the 2nd last item should be a model response
        const secondLastItem = allItems.at(-2);
        if (!secondLastItem) {
          throw new AgentInvariantError(
            "No second last item in the run state.",
          );
        }
        if (secondLastItem.type === "model") {
          content = secondLastItem.content;
        } else if (
          secondLastItem.type === "message" &&
          secondLastItem.role === "assistant"
        ) {
          content = secondLastItem.content;
        } else {
          throw new AgentInvariantError(
            "Expected a model item or assistant message before tool results.",
          );
        }
      }
    } else {
      // ========== Case: Tool Results (from AgentItemTool) ==========
      // Each tool result is an individual item in this representation, so there could be other
      // AgentItemTool before this one. We loop backwards to find the first non-tool item while also
      // tracking the called tool ids to avoid duplicate execution
      for (let i = allItems.length - 1; i >= 0; i--) {
        const item = allItems[i];
        if (!item) {
          // should not happen
          throw new AgentInvariantError("No items in the run state.");
        }
        if (item.type === "tool") {
          processedToolCallIds.add(item.tool_call_id);
          // Continue searching for the originating model/assistant item
          continue;
        } else if (item.type === "model") {
          // Found the originating model response
          content = item.content;
          break;
        } else {
          // Remaining possibility is a message item
          if (item.role === "tool") {
            // Collect all tool call ids in the tool message
            const toolCallParts = item.content.filter(
              (part) => part.type === "tool-result",
            );
            for (const part of toolCallParts) {
              processedToolCallIds.add(part.tool_call_id);
            }
            // Continue searching for the originating model/assistant item
            continue;
          }
          if (item.role === "assistant") {
            // Found the originating model response
            content = item.content;
            break;
          }
          throw new AgentInvariantError(
            "Expected a model item or assistant message before tool results.",
          );
        }
      }
      if (!content) {
        throw new AgentInvariantError(
          "No model or assistant message found before tool results.",
        );
      }
    }

    if (!content.length) {
      throw new AgentInvariantError("No content in the assistant message.");
    }

    const toolCallParts = content.filter((part) => part.type === "tool-call");

    if (!toolCallParts.length) {
      yield {
        type: "response",
        content,
      };
      return;
    }

    for (const toolCallPart of toolCallParts) {
      if (processedToolCallIds.has(toolCallPart.tool_call_id)) {
        // Tool call has already been processed
        continue;
      }

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
      let tools = this.#getTools(); // get initial tool set

      for (;;) {
        const processStream = this.#process(state, tools);
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
            // continue the loop and generate a new model response
            state.turn();
            break;
          }
        }

        const turnParams = this.#getTurnParams(state);
        const languageModelInput = turnParams.input;
        tools = turnParams.tools;

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

        state.appendModelResponse(modelResponse);

        // Continue to loop to process the model response
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
      let tools = this.#getTools(); // get initial tool set

      for (;;) {
        const processStream = this.#process(state, tools);
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
            // continue the loop and generate a new model response
            state.turn();
            break;
          }
        }

        const turnParams = this.#getTurnParams(state);
        const languageModelInput = turnParams.input;
        tools = turnParams.tools;

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

        // Continue to loop to process the model response
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
   * Compute all the tools from static params and toolkit sessions
   */
  #getTools(): AgentTool<TContext>[] {
    const tools = [...this.#static_tools];

    for (const toolkitSession of this.#toolkit_sessions) {
      const toolkitTools = toolkitSession.getTools();
      if (toolkitTools.length > 0) {
        tools.push(...toolkitTools);
      }
    }

    return tools;
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
      // Add toolkit prompts
      for (const toolkitSession of this.#toolkit_sessions) {
        const toolkitPrompt = toolkitSession.getSystemPrompt();
        if (toolkitPrompt?.length) {
          systemPrompts.push(toolkitPrompt);
        }
      }

      // Add system prompt
      if (systemPrompts.length > 0) {
        input.system_prompt = systemPrompts.join("\n");
      }

      // Add tools
      const tools = this.#getTools();
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
