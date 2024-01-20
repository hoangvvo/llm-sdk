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
import { InvariantError } from "./errors.ts";
import {
  getPromptForInstructionParams,
  type InstructionParam,
} from "./instruction.ts";
import type { AgentTool } from "./tool.ts";
import type { AgentRequest, AgentResponse, AgentStreamEvent } from "./types.ts";

/**
 * Manages the run session for an agent run.
 * It initializes all necessary components for the agent to run
 * and handles the execution of the agent's tasks.
 * Once finished, the session cleans up any resources used during the run.
 * The session can be reused in multiple runs.
 */
export class RunSession<TContext> {
  private readonly instructions: InstructionParam<TContext>[];
  private readonly model: LanguageModel;
  private readonly response_format: ResponseFormatOption;
  private readonly tools: AgentTool<any, TContext>[];

  constructor(
    model: LanguageModel,
    instructions: InstructionParam<TContext>[],
    tools: AgentTool<any, TContext>[],
    response_format: ResponseFormatOption,
  ) {
    this.instructions = instructions;
    this.model = model;
    this.response_format = response_format;
    this.tools = tools;
  }

  /**
   * Create a new run session and initializes dependencies
   */
  static async create<TContext>(
    model: LanguageModel,
    instructions: InstructionParam<TContext>[],
    tools: AgentTool<any, TContext>[],
    response_format: ResponseFormatOption,
  ): Promise<RunSession<TContext>> {
    const session = new RunSession(model, instructions, tools, response_format);
    await session.initialize();
    return session;
  }

  async initialize() {
    // Initialize any resources needed for the run session
  }

  /**
   * Process the model response and decide whether to continue the loop or
   * return the response
   */
  private async process(
    modelResponse: ModelResponse,
    context: TContext,
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

    const nextMessages: Message[] = [];

    const toolMessage: ToolMessage = {
      role: "tool",
      content: [],
    };

    for (const toolCallPart of toolCallParts) {
      const agentTool = this.tools.find(
        (tool) => tool.name === toolCallPart.tool_name,
      );

      if (!agentTool) {
        throw new InvariantError(
          `Tool ${toolCallPart.tool_name} not found in agent`,
        );
      }

      const toolRes = await agentTool.execute(toolCallPart.args, context);

      toolMessage.content.push({
        type: "tool-result",
        tool_name: toolCallPart.tool_name,
        tool_call_id: toolCallPart.tool_call_id,
        content: toolRes.content,
        is_error: toolRes.is_error,
      });
    }

    nextMessages.push(toolMessage);

    return {
      type: "next",
      next_messages: nextMessages,
    };
  }

  /**
   * Run a non-streaming execution of the agent.
   */
  async run(request: AgentRequest<TContext>): Promise<AgentResponse> {
    const input = this.getLlmInput(request);
    const context = request.context;

    const newMessages: Message[] = [];

    let response: AgentResponse | undefined;
    while (!response) {
      const modelResponse = await this.model.generate({
        ...input,
        messages: [...input.messages, ...newMessages],
      });

      newMessages.push({
        role: "assistant",
        content: modelResponse.content,
      });

      const processResult = await this.process(modelResponse, context);
      if (processResult.type === "response") {
        response = {
          content: processResult.content,
          new_messages: newMessages,
        };
      } else {
        newMessages.push(...processResult.next_messages);
      }
    }

    return response;
  }

  /**
   * Run a streaming execution of the agent.
   */
  async *runStream(
    request: AgentRequest<TContext>,
  ): AsyncGenerator<AgentStreamEvent, AgentResponse> {
    const input = this.getLlmInput(request);
    const context = request.context;

    let response: AgentResponse | undefined;

    const newMessages: Message[] = [];

    while (!response) {
      const modelStream = this.model.stream(input);

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

      newMessages.push(assistantMessage);

      yield {
        type: "message",
        ...assistantMessage,
      };

      const processResult = await this.process(modelResponse, context);

      if (processResult.type === "response") {
        response = {
          content: processResult.content,
          new_messages: newMessages,
        };
      } else {
        newMessages.push(...processResult.next_messages);
        for (const message of processResult.next_messages) {
          yield {
            type: "message",
            ...message,
          };
        }
      }
    }

    yield {
      type: "response",
      ...response,
    };

    return response;
  }

  /**
   * Finalize any resources or state for the run session
   */
  async finish(): Promise<void> {
    return Promise.resolve();
  }

  private getLlmInput(request: AgentRequest<TContext>): LanguageModelInput {
    return {
      messages: request.messages,
      system_prompt: getPromptForInstructionParams(
        this.instructions,
        request.context,
      ),
      tools: this.tools.map((tool) => ({
        name: tool.name,
        description: tool.description,
        parameters: tool.parameters,
      })),
      response_format: this.response_format,
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
  next_messages: Message[];
}
