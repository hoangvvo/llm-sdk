/* eslint-disable @typescript-eslint/no-explicit-any */

import {
  StreamAccumulator,
  type LanguageModel,
  type LanguageModelInput,
  type Message,
  type ModelResponse,
  type Part,
  type ResponseFormatOption,
  type ToolMessage,
} from "@hoangvvo/llm-sdk";
import { AgentInvariantError, AgentMaxTurnsExceededError } from "./errors.ts";
import {
  getPromptForInstructionParams,
  type InstructionParam,
} from "./instruction.ts";
import type { AgentTool } from "./tool.ts";
import type {
  AgentRequest,
  AgentResponse,
  AgentStreamEvent,
  RunItem,
} from "./types.ts";

/**
 * Manages the run session for an agent run.
 * It initializes all necessary components for the agent to run
 * and handles the execution of the agent's tasks.
 * Once finished, the session cleans up any resources used during the run.
 * The session can be reused in multiple runs.
 */
export class RunSession<TContext> {
  readonly #instructions: InstructionParam<TContext>[];
  readonly #model: LanguageModel;
  readonly #responseFormat: ResponseFormatOption;
  readonly #tools: AgentTool<any, TContext>[];
  readonly #maxTurns: number;

  #initialized: boolean;

  constructor(
    model: LanguageModel,
    instructions: InstructionParam<TContext>[],
    tools: AgentTool<any, TContext>[],
    responseFormat: ResponseFormatOption,
    maxTurns: number,
  ) {
    this.#instructions = instructions;
    this.#model = model;
    this.#responseFormat = responseFormat;
    this.#tools = tools;
    this.#maxTurns = maxTurns;

    this.#initialized = false;
  }

  /**
   * Create a new run session and initializes dependencies
   */
  static async create<TContext>(
    model: LanguageModel,
    instructions: InstructionParam<TContext>[],
    tools: AgentTool<any, TContext>[],
    responseFormat: ResponseFormatOption,
    maxTurns: number,
  ): Promise<RunSession<TContext>> {
    const session = new RunSession(
      model,
      instructions,
      tools,
      responseFormat,
      maxTurns,
    );
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
    modelResponse: ModelResponse,
  ): Promise<ProcessResult> {
    const toolCallParts = modelResponse.content.filter(
      (part) => part.type === "tool-call",
    );

    if (!toolCallParts.length) {
      return {
        type: "response",
        content: modelResponse.content,
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

      const toolRes = await agentTool.execute(
        toolCallPart.args,
        context,
        state,
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

    const state = new RunState(request.messages, this.#maxTurns);

    const input = this.#getLlmInput(request);
    const context = request.context;

    for (;;) {
      const modelResponse = await this.#model.generate({
        ...input,
        messages: state.getTurnMessages(),
      });

      state.appendMessage({
        role: "assistant",
        content: modelResponse.content,
      });

      const processResult = await this.#process(context, state, modelResponse);
      if (processResult.type === "response") {
        return state.createResponse(processResult.content);
      } else {
        for (const message of processResult.messages) {
          state.appendMessage(message);
        }
      }

      state.turn();
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

    const state = new RunState(request.messages, this.#maxTurns);

    const input = this.#getLlmInput(request);
    const context = request.context;

    for (;;) {
      const modelStream = this.#model.stream(input);

      const accumulator = new StreamAccumulator();

      for await (const partial of modelStream) {
        accumulator.addPartial(partial);
        yield {
          type: "partial",
          ...partial,
        };
      }

      const modelResponse = accumulator.computeResponse();

      const assistantMessage: Message = {
        role: "assistant",
        content: modelResponse.content,
      };

      state.appendMessage(assistantMessage);
      yield {
        type: "message",
        ...assistantMessage,
      };

      const processResult = await this.#process(context, state, modelResponse);

      if (processResult.type === "response") {
        const response = state.createResponse(processResult.content);
        yield {
          type: "response",
          ...response,
        };
        return response;
      } else {
        for (const message of processResult.messages) {
          state.appendMessage(message);
          yield {
            type: "message",
            ...message,
          };
        }
      }

      state.turn();
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
    return {
      messages: request.messages,
      system_prompt: getPromptForInstructionParams(
        this.#instructions,
        request.context,
      ),
      tools: this.#tools.map((tool) => ({
        name: tool.name,
        description: tool.description,
        parameters: tool.parameters,
      })),
      response_format: this.#responseFormat,
    };
  }
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
  readonly #inputMessages: Message[];

  /**
   * The current turn number in the run.
   */
  currentTurn: number;

  /**
   * All items generated during the run, such as new ToolMessage and AssistantMessage
   */
  items: RunItem[];

  constructor(inputMessages: Message[], maxTurns: number) {
    this.#inputMessages = inputMessages;
    this.#maxTurns = maxTurns;

    this.currentTurn = 0;
    this.items = [];
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
  appendMessage(message: Message) {
    this.items.push({ type: "message", ...message });
  }

  /**
   * Get LLM messages to use in the LanguageModelInput for the turn
   */
  getTurnMessages(): Message[] {
    return [...this.#inputMessages, ...this.items];
  }

  createResponse(finalContent: Part[]): AgentResponse {
    return {
      content: finalContent,
      items: this.items,
    };
  }
}
