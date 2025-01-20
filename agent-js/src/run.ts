/* eslint-disable @typescript-eslint/no-explicit-any */

import {
  StreamAccumulator,
  type LanguageModel,
  type LanguageModelInput,
  type ModelResponse,
  type ResponseFormatOption,
  type ToolMessage,
} from "@hoangvvo/llm-sdk";
import { InvariantError } from "./errors.ts";
import {
  getPromptForInstructionParams,
  type InstructionParam,
} from "./instruction.ts";
import type { AgentTool } from "./tool.ts";
import type {
  AgentRequest,
  AgentResponse,
  AgentStreamResult,
} from "./types.ts";

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
    input: LanguageModelInput,
    modelResponse: ModelResponse,
    context: TContext,
  ): Promise<ProcessResult> {
    const messages = [...input.messages];

    messages.push({
      role: "assistant",
      content: modelResponse.content,
    });

    const toolCallParts = modelResponse.content.filter(
      (part) => part.type === "tool-call",
    );

    if (!toolCallParts.length) {
      const response: AgentResponse = {
        messages,
        content: modelResponse.content,
      };

      return {
        type: "response",
        response,
      };
    }

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

    messages.push(toolMessage);

    return {
      type: "next",
      input: {
        ...input,
        messages,
      },
    };
  }

  /**
   * Run a non-streaming execution of the agent.
   */
  async run(request: AgentRequest<TContext>): Promise<AgentResponse> {
    let input = this.getLlmInput(request);
    const context = request.context;

    let response: AgentResponse | undefined;
    while (!response) {
      const modelResponse = await this.model.generate(input);
      const processResult = await this.process(input, modelResponse, context);
      if (processResult.type === "response") {
        response = processResult.response;
      } else {
        input = processResult.input;
      }
    }

    return response;
  }

  /**
   * Run a streaming execution of the agent.
   */
  async *runStream(
    request: AgentRequest<TContext>,
  ): AsyncGenerator<AgentStreamResult, AgentResponse> {
    let input = this.getLlmInput(request);
    const context = request.context;

    let response: AgentResponse | undefined;

    while (!response) {
      const modelStream = this.model.stream(input);

      const accumulator = new StreamAccumulator();

      for await (const partial of modelStream) {
        accumulator.addPartial(partial);
        yield {
          type: "partial-model-response",
          ...partial,
        };
      }

      const modelResponse = accumulator.computeResponse();

      yield {
        type: "model-response",
        ...modelResponse,
      };

      const processResult = await this.process(input, modelResponse, context);

      if (processResult.type === "response") {
        response = processResult.response;
      } else {
        input = processResult.input;
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
  response: AgentResponse;
}

interface ProcessResultNext {
  type: "next";
  input: LanguageModelInput;
}
