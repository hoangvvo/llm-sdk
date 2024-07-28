import Anthropic from "@anthropic-ai/sdk";
import type {
  LanguageModel,
  LanguageModelCapability,
} from "../models/language-model.js";
import type {
  AssistantMessage,
  LanguageModelInput,
  Message,
  ModelResponse,
  ModelUsage,
  PartialModelResponse,
  Tool,
} from "../schemas/index.js";
import type { AnthropicModelOptions } from "./types.js";

export class AnthropicModel implements LanguageModel {
  provider: string;
  modelId: string;
  capabilities: LanguageModelCapability[] = ["streaming", "tool"];
  private anthropic: Anthropic;

  constructor(options: AnthropicModelOptions) {
    this.provider = "anthropic";
    this.modelId = options.modelId;

    this.anthropic = new Anthropic({
      baseURL: options.baseURL,
      apiKey: options.apiKey,
    });
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const response = await this.anthropic.messages.create({
      ...convertToAnthropicParams(this.modelId, input),
      stream: false,
    });

    return {
      content: mapAnthropicMessage(response.content).content,
      usage: {
        inputTokens: response.usage.input_tokens,
        outputTokens: response.usage.output_tokens,
      },
    };
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse, ModelResponse> {
    const stream = await this.anthropic.messages.stream({
      ...convertToAnthropicParams(this.modelId, input),
      stream: true,
    });

    const usage: ModelUsage = {
      inputTokens: 0,
      outputTokens: 0,
    };
    const streamingContentBlocks: (Anthropic.Messages.ContentBlock & {
      partial_json?: string;
    })[] = [];

    for await (const chunk of stream) {
      // https://docs.anthropic.com/claude/reference/messages-streaming#raw-http-stream-response

      switch (chunk.type) {
        case "message_start":
          usage.inputTokens += chunk.message.usage.input_tokens;
          usage.outputTokens += chunk.message.usage.output_tokens;
          break;
        case "message_delta":
          usage.outputTokens += chunk.usage.output_tokens;
          break;
        case "content_block_start":
          streamingContentBlocks.length = Math.max(
            streamingContentBlocks.length,
            chunk.index + 1,
          );
          streamingContentBlocks[chunk.index] = {
            ...chunk.content_block,
          };
          break;
        case "content_block_delta": {
          const block = streamingContentBlocks[chunk.index];
          if (chunk.delta.type === "text_delta") {
            if (!block || block.type !== "text") {
              throw new Error(
                `invariant: expected text block at streamingContentBlocks[${chunk.index}]`,
              );
            }
            block.text += chunk.delta.text;
            yield {
              delta: {
                index: chunk.index,
                part: {
                  type: "text",
                  text: chunk.delta.text,
                },
              },
            };
          } else if (chunk.delta.type === "input_json_delta") {
            if (!block || block.type !== "tool_use") {
              throw new Error(
                `invariant: expected tool_use block at streamingContentBlocks[${chunk.index}]`,
              );
            }
            block.partial_json = block.partial_json ?? "";
            block.partial_json += chunk.delta.partial_json;
          }
          break;
        }
        case "content_block_stop": {
          // if a tool call block is completed, we can yield it
          const block = streamingContentBlocks[chunk.index];
          if (!block) {
            throw new Error(
              `invariant: expected block at streamingContentBlocks[${chunk.index}]`,
            );
          }
          if (block.type === "tool_use") {
            yield {
              delta: {
                index: chunk.index,
                part: {
                  type: "tool-call",
                  toolCallId: block.id,
                  args: JSON.parse(block.partial_json ?? "{}"),
                  toolName: block.name,
                },
              },
            };
          }
          break;
        }
      }
    }

    const content = streamingContentBlocks.map(
      (block): Anthropic.Messages.ContentBlock => {
        if (block.type === "tool_use") {
          return {
            ...block,
            input: JSON.parse(block.partial_json ?? "{}"),
          };
        }
        return block;
      },
    );

    return {
      content: mapAnthropicMessage(content).content,
      usage,
    };
  }
}

function convertToAnthropicParams(
  modelId: string,
  input: LanguageModelInput,
): Anthropic.Messages.MessageCreateParams {
  let tool_choice: Anthropic.Messages.MessageCreateParams["tool_choice"];
  if (input.toolChoice) {
    if (input.toolChoice.type === "auto") {
      tool_choice = { type: "auto" };
    } else if (input.toolChoice.type === "required") {
      tool_choice = {
        type: "any",
      };
    } else if (input.toolChoice.type === "tool") {
      tool_choice = {
        type: "tool",
        name: input.toolChoice.toolName,
      };
    }
  }

  return {
    model: modelId,
    messages: convertToAnthropicMessages(input.messages),
    ...(input.systemPrompt && { system: input.systemPrompt }),
    ...(input.tools &&
      input.toolChoice?.type !== "none" && {
        tools: convertToAnthropicTools(input.tools),
      }),
    ...(tool_choice && { tool_choice }),
    max_tokens: input.maxTokens ?? 4096,
    ...(typeof input.temperature === "number" && {
      temperature: input.temperature,
    }),
    ...(typeof input.topP === "number" && { top_p: input.topP }),
    ...(typeof input.topK === "number" && { top_k: input.topK }),
  };
}

function convertToAnthropicMessages(
  messages: Message[],
): Anthropic.Messages.MessageParam[] {
  return messages.map((message): Anthropic.Messages.MessageParam => {
    if (message.role === "assistant") {
      return {
        role: "assistant",
        content: message.content.map(
          (
            part,
          ):
            | Anthropic.Messages.TextBlockParam
            | Anthropic.Messages.ToolUseBlockParam => {
            if (part.type === "tool-call") {
              return {
                type: "tool_use",
                id: part.toolCallId,
                name: part.toolName,
                input: part.args,
              };
            } else {
              return {
                type: "text",
                text: part.text,
              };
            }
          },
        ),
      };
    } else if (message.role === "tool") {
      // anthropic does not have a dedicated tool message type
      return {
        role: "user",
        content: message.content.map(
          (part): Anthropic.Messages.ToolResultBlockParam => ({
            type: "tool_result",
            tool_use_id: part.toolCallId,
            content: [
              {
                type: "text",
                text: JSON.stringify(part.result),
              },
            ],
          }),
        ),
      };
    } else {
      return {
        role: "user",
        content: message.content.map(
          (
            part,
          ):
            | Anthropic.Messages.TextBlockParam
            | Anthropic.Messages.ImageBlockParam => {
            switch (part.type) {
              case "text": {
                return {
                  type: "text",
                  text: part.text,
                };
              }
              case "image": {
                return {
                  type: "image",
                  source: {
                    data: part.imageData,
                    type: "base64",
                    media_type:
                      part.mimeType as Anthropic.Messages.ImageBlockParam["source"]["media_type"],
                  },
                };
              }
              default: {
                throw new Error(`Unsupported message part type: ${part.type}`);
              }
            }
          },
        ),
      };
    }
  });
}

function convertToAnthropicTools(tools: Tool[]): Anthropic.Tool[] {
  return tools.map((tool) => ({
    name: tool.name,
    description: tool.description,
    input_schema: (tool.parameters as Anthropic.Tool.InputSchema) || {
      type: "object",
      // anthropic tool call parameters are required
      // so if no parameters, we define it as null
      properties: null,
    },
  }));
}

function mapAnthropicMessage(
  content: Array<Anthropic.Messages.ContentBlock>,
): AssistantMessage {
  return {
    role: "assistant",
    content: content.map((block): AssistantMessage["content"][number] => {
      if (block.type === "text") {
        return {
          type: "text",
          text: block.text,
        };
      }
      return {
        type: "tool-call",
        toolCallId: block.id,
        toolName: block.name,
        args: block.input as Record<string, unknown>,
      };
    }),
  };
}
