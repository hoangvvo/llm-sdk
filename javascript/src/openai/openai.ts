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
import { OpenAIRefusedError } from "./errors.js";
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

  constructor(public options: OpenAIModelOptions) {
    this.provider = "openai";
    this.modelId = options.modelId;

    this.openai = new OpenAI({
      baseURL: options.baseURL,
      apiKey: options.apiKey,
    });
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const response = await this.openai.chat.completions.create({
      ...convertToOpenAIParams(this.modelId, input, this.options),
      stream: false,
    });

    if (!response.choices[0]) {
      throw new Error("no choices in response");
    }

    const choice = response.choices[0];

    if (choice.message.refusal) {
      throw new OpenAIRefusedError(choice.message.refusal);
    }

    return {
      content: mapOpenAIMessage(choice.message).content,
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
      ...convertToOpenAIParams(this.modelId, input, this.options),
      stream: true,
      stream_options: {
        include_usage: true,
      },
    });

    let usage: ModelUsage | undefined;

    const streamingMessage: OpenAI.Chat.ChatCompletionMessage = {
      content: null,
      role: "assistant",
      refusal: null,
    };

    for await (const chunk of stream) {
      const completion = chunk.choices?.[0] as
        | OpenAI.Chat.Completions.ChatCompletionChunk.Choice
        | undefined;

      if (completion?.delta.refusal) {
        streamingMessage.refusal = streamingMessage.refusal || "";
        streamingMessage.refusal += completion.delta.refusal;
      }

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

    if (streamingMessage.refusal) {
      throw new OpenAIRefusedError(streamingMessage.refusal);
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
  options: OpenAIModelOptions,
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
    | OpenAI.Chat.Completions.ChatCompletionCreateParams["response_format"]
    | undefined;
  if (input.responseFormat?.type === "json") {
    if (options.structuredOutputs && input.responseFormat.schema) {
      const schemaTitle = input.responseFormat.schema["title"] as
        | string
        | undefined;
      const schemaDescription = input.responseFormat.schema["description"] as
        | string
        | undefined;
      response_format = {
        type: "json_schema",
        json_schema: {
          strict: true,
          name: schemaTitle || "response",
          ...(schemaDescription && { description: schemaDescription }),
          schema: input.responseFormat.schema,
        },
      };
    } else {
      response_format = {
        type: "json_object",
      };
    }
  }

  return {
    model: modelId,
    messages: convertToOpenAIMessages(input.messages, input.systemPrompt),
    ...(input.tools && {
      tools: convertToOpenAITools(input.tools, options),
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
            const contentParts = message.content;
            return {
              role: "user",
              content: contentParts.map(
                (part): OpenAI.Chat.ChatCompletionContentPart => {
                  switch (part.type) {
                    case "text": {
                      return {
                        type: "text",
                        text: part.text,
                      };
                    }
                    case "image": {
                      return {
                        type: "image_url",
                        image_url: {
                          url: `data:${part.mimeType};base64,${part.imageData}`,
                        },
                      };
                    }
                    case "audio": {
                      throw new Error(
                        `Unsupported message part type: ${part.type}`,
                      );
                    }
                  }
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
  options: OpenAIModelOptions,
): OpenAI.Chat.Completions.ChatCompletionTool[] {
  return tools.map(
    (tool): OpenAI.Chat.Completions.ChatCompletionTool => ({
      type: "function",
      function: {
        ...(options.structuredOutputs && {
          strict: true,
        }),
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
