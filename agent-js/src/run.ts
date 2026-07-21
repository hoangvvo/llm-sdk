import {
  LanguageModelError,
  StreamAccumulator,
  type LanguageModelInput,
  type Message,
  type ModelResponse,
  type Part,
  type ToolCallPart,
} from "@hoangvvo/llm-sdk";
import {
  agentErrorWithSnapshot,
  AgentCleanupError,
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
import { type AgentFunctionTool, type AgentTool } from "./tool.ts";
import type { ToolkitSession } from "./toolkit.ts";
import type {
  AgentItem,
  AgentItemModelResponse,
  AgentItemTool,
  AgentResponse,
  AgentResponseStatus,
  AgentRunSnapshot,
  AgentStreamEvent,
} from "./types.ts";

function createCancelledToolItem(toolCall: ToolCallPart): AgentItemTool {
  if (toolCall.call.type !== "function") {
    throw new AgentInvariantError("Cannot cancel a provider-hosted tool call");
  }
  return {
    type: "tool",
    tool_call_id: toolCall.tool_call_id,
    tool_name: toolCall.call.name,
    input: toolCall.call.args,
    output: [],
    status: "cancelled",
  };
}

function* createToolCancellationEvents(
  pendingToolCalls: readonly ToolCallPart[],
): Generator<ProcessEvent> {
  for (const pendingToolCall of pendingToolCalls) {
    yield {
      type: "item",
      item: createCancelledToolItem(pendingToolCall),
    };
  }
  yield { type: "response", content: [], status: "cancelled" };
}

function finishCancelledRun(
  state: RunState,
  span: AgentSpan,
  content: Part[] = [],
): AgentResponse {
  const response = state.createResponse(content, "cancelled");
  span.onResponse(response);
  return response;
}

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
    try {
      // Resolve the instructions using the provided context
      if (this.#params.instructions.length > 0) {
        const systemPrompt = await getPromptForInstructionParams(
          this.#params.instructions,
          this.#context,
        );
        this.#static_system_prompt = systemPrompt;
      }

      if (this.#params.toolkits.length > 0) {
        const results = await Promise.allSettled(
          this.#params.toolkits.map((toolkit) =>
            toolkit.createSession(this.#context),
          ),
        );
        const failed = results.find(
          (result): result is PromiseRejectedResult =>
            result.status === "rejected",
        );
        if (failed) {
          await Promise.allSettled(
            results
              .filter(
                (
                  result,
                ): result is PromiseFulfilledResult<ToolkitSession<TContext>> =>
                  result.status === "fulfilled",
              )
              .map(async (result) => result.value.close()),
          );
          throw failed.reason;
        }
        this.#toolkit_sessions = results.map(
          (result) =>
            (result as PromiseFulfilledResult<ToolkitSession<TContext>>).value,
        );
      }
    } catch (error) {
      throw new AgentInitError(error);
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
  ): AsyncGenerator<ProcessEvent> {
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

    const allToolCallParts = content.filter(
      (part) => part.type === "tool-call",
    );
    const toolCallParts = allToolCallParts.filter(
      (part) => part.call.type === "function",
    );

    if (!toolCallParts.length) {
      yield {
        type: "response",
        content,
        status: state.signal?.aborted ? "cancelled" : "completed",
      };
      return;
    }

    const toolCallIds = new Set<string>();
    for (const toolCallPart of allToolCallParts) {
      if (toolCallIds.has(toolCallPart.tool_call_id)) {
        throw new AgentInvariantError(
          `Duplicate tool call ID: ${toolCallPart.tool_call_id}`,
        );
      }
      toolCallIds.add(toolCallPart.tool_call_id);
    }

    const pendingToolCalls = toolCallParts.filter(
      (part) => !processedToolCallIds.has(part.tool_call_id),
    );

    for (const [index, toolCallPart] of pendingToolCalls.entries()) {
      if (state.signal?.aborted) {
        yield* createToolCancellationEvents(pendingToolCalls.slice(index));
        return;
      }
      const call = toolCallPart.call;
      if (call.type !== "function") continue;

      const agentTool = tools.find(
        (tool): tool is AgentFunctionTool<TContext> =>
          tool.type === "function" && tool.name === call.name,
      );

      if (!agentTool) {
        throw new AgentInvariantError(`Tool ${call.name} not found in agent`);
      }

      let toolRes;
      try {
        toolRes = await startActiveToolSpan(
          toolCallPart.tool_call_id,
          agentTool.name,
          agentTool.description,
          async () => {
            try {
              return await agentTool.execute(call.args, this.#context, state);
            } catch (e) {
              throw new AgentToolExecutionError(e);
            }
          },
        );
      } catch (error) {
        if (!state.signal?.aborted) throw error;
        yield* createToolCancellationEvents(pendingToolCalls.slice(index));
        return;
      }

      const agentItemTool: AgentItemTool = {
        type: "tool",
        tool_name: call.name,
        tool_call_id: toolCallPart.tool_call_id,
        input: call.args,
        output: toolRes.content,
        status: toolRes.is_error ? "failed" : "completed",
      };

      yield {
        type: "item",
        item: agentItemTool,
      };

      if (state.signal?.aborted) {
        yield* createToolCancellationEvents(pendingToolCalls.slice(index + 1));
        return;
      }
    }

    yield {
      type: "next",
    };
  }

  /**
   * Run a non-streaming execution of the agent.
   */
  async run(
    request: RunSessionRequest,
    options?: RunOptions,
  ): Promise<AgentResponse> {
    if (!this.#initialized) {
      throw new AgentInvariantError("RunSession not initialized.");
    }

    const span = new AgentSpan(this.#params.name, "run");
    const state = new RunState(
      request.input,
      this.#params.max_turns,
      options?.signal,
    );

    try {
      let tools = this.#getTools(); // get initial tool set

      for (;;) {
        const processStream = this.#process(state, tools);
        // Patch next to propagate tracing context
        // See: https://github.com/open-telemetry/opentelemetry-js/issues/2951
        const originalNext = processStream.next.bind(processStream);
        processStream.next = (...args) =>
          span.withContext(() => originalNext(...args));

        for await (const event of processStream) {
          if (event.type === "item") {
            state.appendItem(event.item);
          }
          if (event.type === "response") {
            const response = state.createResponse(event.content, event.status);
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

        if (state.signal?.aborted) {
          return finishCancelledRun(state, span);
        }
        let modelResponse: ModelResponse;
        try {
          modelResponse = await span.withContext(async () => {
            try {
              return await this.#params.model.generate(
                languageModelInput,
                state.signal ? { signal: state.signal } : undefined,
              );
            } catch (err) {
              if (err instanceof LanguageModelError) {
                throw new AgentLanguageModelError(err);
              }
              throw err;
            }
          });
        } catch (error) {
          if (!state.signal?.aborted) throw error;
          return finishCancelledRun(state, span);
        }

        state.appendModelResponse(modelResponse);

        // Continue to loop to process the model response
      }
    } catch (err) {
      const error = agentErrorWithSnapshot(err, state.createSnapshot());
      span.onError(error);
      throw error;
    } finally {
      span.onEnd();
    }
  }

  /**
   * Run a streaming execution of the agent.
   */
  async *runStream(
    request: RunSessionRequest,
    options?: RunOptions,
  ): AsyncGenerator<AgentStreamEvent, AgentResponse> {
    if (!this.#initialized) {
      throw new AgentInvariantError("RunSession not initialized.");
    }

    const span = new AgentSpan(this.#params.name, "run_stream");
    const state = new RunState(
      request.input,
      this.#params.max_turns,
      options?.signal,
    );

    try {
      let tools = this.#getTools(); // get initial tool set

      for (;;) {
        const processStream = this.#process(state, tools);
        // Patch next to propagate tracing context
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
            const response = state.createResponse(event.content, event.status);
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

        if (state.signal?.aborted) {
          const response = finishCancelledRun(state, span);
          yield { event: "response", ...response };
          return response;
        }
        const modelStream = this.#params.model.stream(
          languageModelInput,
          state.signal ? { signal: state.signal } : undefined,
        );
        // Patch next to propagate tracing context
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
          const snapshot = accumulator.snapshot();
          const appended = state.appendModelSnapshot(snapshot);
          if (appended) {
            yield { event: "item", ...appended };
          }
          if (state.signal?.aborted) {
            if (appended) continue;
            const response = finishCancelledRun(state, span, snapshot.content);
            yield { event: "response", ...response };
            return response;
          }
          if (err instanceof LanguageModelError) {
            throw agentErrorWithSnapshot(
              new AgentLanguageModelError(err),
              state.createSnapshot(),
            );
          }
          throw agentErrorWithSnapshot(err, state.createSnapshot());
        }

        if (state.signal?.aborted) {
          const snapshot = accumulator.snapshot();
          const appended = state.appendModelSnapshot(snapshot);
          if (appended) {
            yield { event: "item", ...appended };
            continue;
          }
          const response = finishCancelledRun(state, span, snapshot.content);
          yield { event: "response", ...response };
          return response;
        }

        let response: ModelResponse;
        try {
          response = accumulator.computeResponse();
        } catch (error) {
          const snapshot = accumulator.snapshot();
          const appended = state.appendModelSnapshot(snapshot);
          if (appended) {
            yield { event: "item", ...appended };
          }
          throw agentErrorWithSnapshot(error, state.createSnapshot());
        }

        const { item, index } = state.appendModelResponse(response);
        yield {
          event: "item",
          index,
          item,
        };

        // Continue to loop to process the model response
      }
    } catch (err) {
      const error = agentErrorWithSnapshot(err, state.createSnapshot());
      span.onError(error);
      throw error;
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

    const results = await Promise.allSettled(
      this.#toolkit_sessions.map(async (session) => session.close()),
    );
    this.#toolkit_sessions = [];
    this.#initialized = false;

    const failed = results.find(
      (result): result is PromiseRejectedResult => result.status === "rejected",
    );
    if (failed) throw new AgentCleanupError(failed.reason);
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
        input.tools = tools.map((tool) =>
          tool.type === "function"
            ? {
                type: "function",
                name: tool.name,
                description: tool.description,
                parameters: tool.parameters,
              }
            : tool,
        );
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

export interface RunOptions {
  signal?: AbortSignal;
}

/**
 * ProcessEvent represents the sum type of events returned by the process function.
 */
type ProcessEvent = ProcessEventResponse | ProcessEventNext | ProcessEventItem;

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
  status: AgentResponseStatus;
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
   * The signal used to cancel the current run.
   */
  readonly signal: AbortSignal | undefined;

  /**
   * The current turn number in the run.
   */
  currentTurn: number;

  /**
   * All items generated during the run, such as new ToolMessage and AssistantMessage
   */
  readonly #output: AgentItem[];

  constructor(input: AgentItem[], maxTurns: number, signal?: AbortSignal) {
    this.#input = input;
    this.#maxTurns = maxTurns;
    this.signal = signal;

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
   * Append the independently materializable portion of an interrupted model
   * stream. An empty snapshot does not represent an output item.
   */
  appendModelSnapshot(
    response: ModelResponse,
  ): { item: AgentItem; index: number } | undefined {
    if (response.content.length === 0) return undefined;
    return this.appendModelResponse(response);
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
            tool_call_id: item.tool_call_id,
            result: {
              type: "function",
              name: item.tool_name,
              content: item.output,
            },
            status: item.status,
          };

          const lastMessage = messages[messages.length - 1];
          if (lastMessage?.role !== "tool") {
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
   * Create a best-effort snapshot of the current run.
   */
  createSnapshot(): AgentRunSnapshot {
    return {
      output: [...this.#output],
    };
  }

  /**
   * Create the Agent Response
   */
  createResponse(
    finalContent: Part[],
    status: AgentResponseStatus,
  ): AgentResponse {
    return {
      content: finalContent,
      output: this.#output,
      status,
    };
  }
}
