import OpenAI from "openai";
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
  TextPart,
  Tool,
  ToolCallPart,
} from "../schemas/index.js";
import type { OpenAIModelOptions } from "./types.js";

export class OpenAIModel implements LanguageModel {
  provider: string;
  modelId: string;
  capabilities: LanguageModelCapability[] = [
    "streaming",
    "tool",
    "response-format-json",
  ];

  private openai: OpenAI;

  constructor(options: OpenAIModelOptions) {
    this.provider = "openai";
    this.modelId = options.modelId;

    this.openai = new OpenAI({
      baseURL: options.baseURL,
      apiKey: options.apiKey,
    });
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const response = await this.openai.chat.completions.create({
      ...convertToOpenAIParams(this.modelId, input),
      stream: false,
    });

    if (!response.choices[0]) {
      throw new Error("no choices in response");
    }

    return {
      content: mapOpenAIMessage(response.choices[0].message).content,
      ...(response.usage && {
        usage: {
          inputTokens: response.usage.prompt_tokens,
          outputTokens: response.usage.completion_tokens,
        },
      }),
    };
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse, ModelResponse> {
    const stream = await this.openai.chat.completions.create({
      ...convertToOpenAIParams(this.modelId, input),
      stream: true,
      stream_options: {
        include_usage: true,
      },
    });

    let usage: ModelUsage | undefined;

    const streamingMessage: OpenAI.Chat.ChatCompletionMessage = {
      content: null,
      role: "assistant",
    };

    for await (const chunk of stream) {
      const completion = chunk.choices?.[0] as
        | OpenAI.Chat.Completions.ChatCompletionChunk.Choice
        | undefined;

      if (completion?.delta.tool_calls) {
        // only the first delta has both `id` and `index`, the rest only have `index`
        // the `arguments` get streamed in: eg: "{" -> "title" -> ":" -> "Foo" -> "}"
        streamingMessage.tool_calls = streamingMessage.tool_calls || [];

        for (const deltaToolCall of completion.delta.tool_calls) {
          streamingMessage.tool_calls[deltaToolCall.index] = streamingMessage
            .tool_calls[deltaToolCall.index] || {
            id: "",
            function: {
              arguments: "",
              name: "",
            },
            type: "function",
          };

          const streamingToolCall =
            streamingMessage.tool_calls[deltaToolCall.index];

          if (!streamingToolCall) {
            throw new Error(
              `invariant: streamingMessage.tool_call[${deltaToolCall.index}] is undefined`,
            );
          }

          if (deltaToolCall.id) {
            streamingToolCall.id = deltaToolCall.id;
          }
          if (deltaToolCall.function?.name) {
            streamingToolCall.function.name += deltaToolCall.function.name;
          }
          if (deltaToolCall.function?.arguments) {
            streamingToolCall.function.arguments +=
              deltaToolCall.function.arguments;
          }
        }
      }

      if (completion?.delta.content) {
        streamingMessage.content = streamingMessage.content || "";
        streamingMessage.content += completion.delta.content;

        yield {
          delta: {
            // It should be safe to assume the index is always 0
            // because openai does not send text content for tool calling
            index: 0,
            part: {
              type: "text",
              text: completion.delta.content,
            },
          },
        };
      }

      if (chunk.usage) {
        usage = {
          inputTokens: chunk.usage.prompt_tokens,
          outputTokens: chunk.usage.completion_tokens,
        };
      }
    }

    // yield each tool call as a partial response
    // to guarantee that the caller can see the tool call
    const message = mapOpenAIMessage(streamingMessage);
    for (const [index, part] of Object.entries(message.content)) {
      if (part.type === "tool-call") {
        yield {
          delta: {
            index: Number(index),
            part,
          },
        };
      }
    }

    return {
      content: mapOpenAIMessage(streamingMessage).content,
      ...(usage && { usage }),
    };
  }
}

function convertToOpenAIParams(
  modelId: string,
  input: LanguageModelInput,
): OpenAI.Chat.ChatCompletionCreateParams {
  let tool_choice:
    | OpenAI.Chat.Completions.ChatCompletionToolChoiceOption
    | undefined;
  if (input.toolChoice) {
    if (input.toolChoice.type === "tool") {
      tool_choice = {
        type: "function",
        function: {
          name: input.toolChoice.toolName,
        },
      };
    } else {
      // 1-1 mapping with openai tool choice
      tool_choice = input.toolChoice.type;
    }
  }

  let response_format:
    | OpenAI.Chat.Completions.ChatCompletionCreateParams.ResponseFormat
    | undefined;
  if (input.responseFormat?.type === "json") {
    response_format = {
      type: "json_object",
    };
  }

  return {
    model: modelId,
    messages: convertToOpenAIMessages(input.messages, input.systemPrompt),
    ...(input.tools && {
      tools: convertToOpenAITools(input.tools),
    }),
    ...(tool_choice && { tool_choice }),
    ...(typeof input.maxTokens === "number" && { max_tokens: input.maxTokens }),
    ...(typeof input.temperature === "number" && {
      temperature: input.temperature,
    }),
    ...(typeof input.topP === "number" && { top_p: input.topP }),
    ...(typeof input.presencePenalty === "number" && {
      presence_penalty: input.presencePenalty,
    }),
    ...(typeof input.frequencyPenalty === "number" && {
      frequency_penalty: input.frequencyPenalty,
    }),
    ...(typeof input.seed === "number" && { seed: input.seed }),
    ...(response_format && {
      response_format,
    }),
  };
}

function convertToOpenAIMessages(
  messages: Message[],
  systemPrompt?: string,
): OpenAI.Chat.ChatCompletionMessageParam[] {
  return [
    ...(systemPrompt
      ? ([{ role: "system", content: systemPrompt }] as const)
      : []),
    ...messages
      .map(
        (
          message,
        ):
          | OpenAI.Chat.ChatCompletionMessageParam
          | OpenAI.Chat.ChatCompletionMessageParam[] => {
          if (message.role === "assistant") {
            const textParts = message.content.filter(
              (part): part is TextPart => part.type === "text",
            );
            const toolCallParts = message.content.filter(
              (part): part is ToolCallPart => part.type === "tool-call",
            );
            return {
              role: "assistant",
              content: textParts.length
                ? textParts.map((part) => part.text).join("\n")
                : null,
              ...(toolCallParts.length && {
                tool_calls: toolCallParts.map((part) => ({
                  type: "function",
                  id: part.toolCallId,
                  function: {
                    name: part.toolName,
                    arguments: JSON.stringify(part.args),
                  },
                })),
              }),
            };
          } else if (message.role === "tool") {
            return message.content.map((toolResult) => ({
              role: "tool",
              content: JSON.stringify(toolResult.result),
              tool_call_id: toolResult.toolCallId,
            }));
          } else {
            const contentParts = message.content.filter(
              (part) => part.type === "text" || part.type === "image",
            );
            return {
              role: "user",
              content: contentParts.map(
                (part): OpenAI.Chat.ChatCompletionContentPart => {
                  if (part.type === "image") {
                    return {
                      type: "image_url",
                      image_url: {
                        url: `data:${part.mimeType};base64,${part.imageData}`,
                      },
                    };
                  }
                  return {
                    type: "text",
                    text: part.text,
                  };
                },
              ),
            };
          }
        },
      )
      .flat(),
  ];
}

function convertToOpenAITools(
  tools: Tool[],
): OpenAI.Chat.Completions.ChatCompletionTool[] {
  return tools.map(
    (tool): OpenAI.Chat.Completions.ChatCompletionTool => ({
      type: "function",
      function: {
        name: tool.name,
        description: tool.description,
        parameters: (tool.parameters as OpenAI.FunctionParameters) || undefined,
      },
    }),
  );
}

function mapOpenAIMessage(
  message: OpenAI.Chat.Completions.ChatCompletionMessage,
): AssistantMessage {
  return {
    role: "assistant",
    content: [
      ...(message.content
        ? [{ type: "text", text: message.content } as TextPart]
        : []),
      // tool call and content of openai are separate, so we define
      // an order here where the text content comes first, followed by
      // tool calls.
      ...(message.tool_calls
        ? message.tool_calls.map(
            (toolCall): ToolCallPart => ({
              type: "tool-call",
              toolCallId: toolCall.id,
              toolName: toolCall.function.name,
              args: JSON.parse(toolCall.function.arguments),
            }),
          )
        : []),
    ],
  };
}
