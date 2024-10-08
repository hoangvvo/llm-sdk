import OpenAI from "openai";
import type {
  LanguageModel,
  LanguageModelCapability,
} from "../models/language-model.js";
import type {
  AssistantMessage,
  ContentDelta,
  LanguageModelInput,
  Message,
  ModelResponse,
  ModelUsage,
  PartialModelResponse,
  TextPart,
  Tool,
  ToolCallPart,
} from "../schemas/index.js";
import { mapContentDeltas, mergeContentDeltas } from "../utils/stream.utils.js";
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

    let refusal = "";

    let contentDeltas: ContentDelta[] = [];

    for await (const chunk of stream) {
      const choice = chunk.choices?.[0];

      const completion = choice as
        | OpenAI.Chat.Completions.ChatCompletionChunk.Choice
        | undefined;

      if (completion?.delta.refusal) {
        refusal += completion.delta.refusal;
      }

      if (completion?.delta) {
        const incomingContentDeltas = mapOpenAIDelta(completion.delta);
        contentDeltas = mergeContentDeltas(
          contentDeltas,
          incomingContentDeltas,
        );

        for (const delta of incomingContentDeltas) {
          yield { delta };
        }
      }

      if (chunk.usage) {
        usage = {
          inputTokens: chunk.usage.prompt_tokens,
          outputTokens: chunk.usage.completion_tokens,
        };
      }
    }

    if (refusal) {
      throw new OpenAIRefusedError(refusal);
    }

    return {
      content: mapContentDeltas(contentDeltas),
      ...(usage && { usage }),
    };
  }
}

export function convertToOpenAIParams(
  modelId: string,
  input: LanguageModelInput,
  options: OpenAIModelOptions,
): OpenAI.Chat.ChatCompletionCreateParams {
  const tool_choice = convertToOpenAIToolChoice(input.toolChoice);

  const response_format = convertToOpenAIResponseFormat(
    input.responseFormat,
    options,
  );

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

export function convertToOpenAIMessages(
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
            const openaiMessageParam: OpenAI.Chat.ChatCompletionMessageParam = {
              role: "assistant",
              content:
                null as Array<OpenAI.Chat.ChatCompletionContentPartText> | null,
            };
            message.content.forEach((part) => {
              if (part.type === "text") {
                openaiMessageParam.content = [
                  ...(openaiMessageParam.content || []),
                  {
                    type: "text",
                    text: part.text,
                  },
                ] as Array<OpenAI.Chat.ChatCompletionContentPartText>;
              } else if (part.type === "tool-call") {
                openaiMessageParam.tool_calls = [
                  ...(openaiMessageParam.tool_calls || []),
                  {
                    type: "function",
                    id: part.toolCallId,
                    function: {
                      name: part.toolName,
                      arguments: JSON.stringify(part.args),
                    },
                  },
                ];
              } else {
                throw new Error(
                  `Unsupported message part type: ${(part as { type: string }).type}`,
                );
              }
            });
            return openaiMessageParam;
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
                    default: {
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

export function convertToOpenAIToolChoice(
  toolChoice: LanguageModelInput["toolChoice"],
): OpenAI.Chat.Completions.ChatCompletionToolChoiceOption | undefined {
  if (toolChoice) {
    if (toolChoice.type === "tool") {
      return {
        type: "function",
        function: {
          name: toolChoice.toolName,
        },
      };
    } else {
      // 1-1 mapping with openai tool choice
      return toolChoice.type;
    }
  }
  return undefined;
}

export function convertToOpenAITools(
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

export function convertToOpenAIResponseFormat(
  responseFormat: LanguageModelInput["responseFormat"],
  options: Pick<OpenAIModelOptions, "structuredOutputs">,
):
  | OpenAI.Chat.Completions.ChatCompletionCreateParams["response_format"]
  | undefined {
  if (!responseFormat) {
    return undefined;
  }
  if (responseFormat.type === "json") {
    if (options.structuredOutputs && responseFormat.schema) {
      const schemaTitle = responseFormat.schema["title"] as string | undefined;
      const schemaDescription = responseFormat.schema["description"] as
        | string
        | undefined;
      return {
        type: "json_schema",
        json_schema: {
          strict: true,
          name: schemaTitle || "response",
          ...(schemaDescription && { description: schemaDescription }),
          schema: responseFormat.schema,
        },
      };
    } else {
      return {
        type: "json_object",
      };
    }
  } else if (responseFormat.type === "text") {
    return {
      type: "text",
    };
  } else {
    throw new Error(
      `Unsupported response format: ${(responseFormat as { type: "string" }).type}`,
    );
  }
}

export function mapOpenAIMessage(
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

export function mapOpenAIDelta(
  delta: OpenAI.Chat.Completions.ChatCompletionChunk.Choice.Delta,
): ContentDelta[] {
  if (delta.content) {
    return [
      {
        // It should be safe to assume the index is always 0
        // because openai does not send text content for tool calling
        index: 0,
        part: {
          type: "text",
          text: delta.content,
        },
      },
    ];
  }
  if (delta.tool_calls) {
    return delta.tool_calls.map((toolCall) => ({
      index: toolCall.index,
      part: {
        type: "tool-call",
        ...(toolCall.id && { toolCallId: toolCall.id }),
        ...(toolCall.function?.name && { toolName: toolCall.function.name }),
        ...(toolCall.function?.arguments && {
          args: toolCall.function?.arguments,
        }),
      },
    }));
  }
  return [];
}
