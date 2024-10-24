import OpenAI from "openai";
import type {
  LanguageModel,
  LanguageModelCapability,
} from "../models/language-model.js";
import type {
  AssistantMessage,
  AudioEncoding,
  AudioPart,
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

// TODO: no official documentation on this
const OPENAI_AUDIO_SAMPLE_RATE = 24_000;
const OPENAI_AUDIO_CHANNELS = 1;

type OpenAILanguageModelInput = LanguageModelInput & {
  extra?: Partial<OpenAI.Chat.Completions.ChatCompletionCreateParams>;
};

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

  async generate(input: OpenAILanguageModelInput): Promise<ModelResponse> {
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

    const usage: ModelUsage | undefined = response.usage
      ? {
          inputTokens: response.usage.prompt_tokens,
          outputTokens: response.usage.completion_tokens,
        }
      : undefined;

    return {
      content: mapOpenAIMessage(choice.message, input.extra).content,
      ...(usage && { usage }),
      ...(this.options.pricing &&
        usage && {
          cost: calculateOpenAICost(usage, this.options.pricing),
        }),
    };
  }

  async *stream(
    input: OpenAILanguageModelInput,
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
      ...(this.options.pricing &&
        usage && {
          cost: calculateOpenAICost(usage, this.options.pricing),
        }),
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

  const samplingParams = convertToOpenAISamplingParams(input);

  return {
    model: modelId,
    messages: convertToOpenAIMessages(input.messages, input.systemPrompt),
    ...(input.tools && {
      tools: convertToOpenAITools(input.tools, options),
    }),
    ...(tool_choice && { tool_choice }),
    ...samplingParams,
    ...(response_format && {
      response_format,
    }),
    ...(input.modalities && {
      modalities: input.modalities,
    }),
    ...input.extra,
  };
}

export function convertToOpenAIMessages(
  messages: Message[],
  systemPrompt?: string,
): OpenAI.Chat.ChatCompletionMessageParam[] {
  const systemMessage: OpenAI.Chat.ChatCompletionSystemMessageParam | null =
    systemPrompt
      ? {
          role: "system",
          content: systemPrompt,
        }
      : null;
  return [
    ...(systemMessage ? [systemMessage] : []),
    ...messages
      .map(
        (
          message,
        ):
          | OpenAI.Chat.ChatCompletionMessageParam
          | OpenAI.Chat.ChatCompletionMessageParam[] => {
          if (message.role === "assistant") {
            const openaiMessageParam: OpenAI.Chat.ChatCompletionAssistantMessageParam =
              {
                role: "assistant",
                content: null,
              };
            message.content.forEach((part) => {
              switch (part.type) {
                case "text": {
                  openaiMessageParam.content = [
                    ...(openaiMessageParam.content || []),
                    {
                      type: "text",
                      text: part.text,
                    },
                  ] as Array<OpenAI.Chat.ChatCompletionContentPartText>;
                  break;
                }
                case "audio": {
                  // openai does support feeding back the audio by using
                  // their internal ID, but being a more generic API, we
                  // don't want to bring that ID into our API. However,
                  // AudioPart does contain a transcript, so we can use that
                  // as TextPart
                  if (part.transcript) {
                    openaiMessageParam.content = [
                      ...(openaiMessageParam.content || []),
                      {
                        type: "text",
                        text: part.transcript,
                      },
                    ] as Array<OpenAI.Chat.ChatCompletionContentPartText>;
                  }
                  break;
                }
                case "tool-call": {
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
                  break;
                }
                default: {
                  throw new Error(
                    `Unsupported message part type: ${(part as { type: string }).type}`,
                  );
                }
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
                    case "audio": {
                      return {
                        type: "input_audio",
                        input_audio: {
                          // this as assertion is not correct, but we will rely on OpenAI to throw an error
                          format: convertToOpenAIAudioFormat(
                            part.encoding,
                          ) as OpenAI.ChatCompletionContentPartInputAudio.InputAudio["format"],
                          data: part.audioData,
                        },
                      };
                    }
                    default: {
                      throw new Error(
                        `Unsupported message part type: ${(part as { type: string }).type}`,
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

export function convertToOpenAISamplingParams(
  input: Partial<LanguageModelInput>,
) {
  return {
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
  } satisfies Partial<OpenAI.Chat.ChatCompletionCreateParams>;
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

export function convertToOpenAIAudioFormat(
  encoding: AudioEncoding,
): OpenAI.Chat.ChatCompletionAudioParam["format"] {
  switch (encoding) {
    case "linear16":
      return "pcm16";
    case "flac":
      return "flac";
    case "mp3":
      return "mp3";
    case "opus":
      return "opus";
    default: {
      throw new Error(`Unsupported audio encoding: ${encoding}`);
    }
  }
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

export function mapOpenAIAudioFormat(
  format: OpenAI.Chat.ChatCompletionAudioParam["format"],
): AudioEncoding {
  switch (format) {
    case "wav":
      return "linear16";
    case "mp3":
      return "mp3";
    case "flac":
      return "flac";
    case "opus":
      return "opus";
    case "pcm16":
      return "linear16";
    default: {
      throw new Error(`Unsupported audio format: ${format}`);
    }
  }
}

export function mapOpenAIMessage(
  message: OpenAI.Chat.Completions.ChatCompletionMessage,
  options?: Partial<OpenAI.Chat.ChatCompletionCreateParams>,
): AssistantMessage {
  return {
    role: "assistant",
    content: [
      ...(message.content
        ? [{ type: "text", text: message.content } as TextPart]
        : []),
      ...(message.audio
        ? [
            {
              type: "audio",
              encoding: options?.audio?.format
                ? mapOpenAIAudioFormat(options.audio.format)
                : "linear16",
              ...(options?.audio?.format === "pcm16" && {
                sampleRate: OPENAI_AUDIO_SAMPLE_RATE,
                channels: OPENAI_AUDIO_CHANNELS,
              }),
              audioData: message.audio.data,
              transcript: message.audio.transcript,
            } satisfies AudioPart,
          ]
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
  delta: OpenAI.Chat.Completions.ChatCompletionChunk.Choice.Delta & {
    audio?: {
      id?: string;
      data?: string;
      transcript?: string;
    };
  },
  options?: Partial<OpenAI.Chat.ChatCompletionCreateParams>,
): ContentDelta[] {
  const contentDeltas: ContentDelta[] = [];
  if (delta.content) {
    contentDeltas.push({
      // It should be safe to assume the index is always 0
      // because openai does not send text content for tool calling
      index: 0,
      part: {
        type: "text",
        text: delta.content,
      },
    });
  }
  if (delta.audio) {
    contentDeltas.push({
      index: 0,
      part: {
        type: "audio",
        ...(delta.audio.data && {
          audioData: delta.audio.data,
          encoding: options?.audio?.format
            ? mapOpenAIAudioFormat(options.audio.format)
            : "linear16",
          sampleRate: OPENAI_AUDIO_SAMPLE_RATE,
          channels: OPENAI_AUDIO_CHANNELS,
        }),
        ...(delta.audio.transcript && { transcript: delta.audio.transcript }),
      },
    });
  }
  if (delta.tool_calls) {
    for (const toolCall of delta.tool_calls) {
      contentDeltas.push({
        index: toolCall.index,
        part: {
          type: "tool-call",
          ...(toolCall.id && { toolCallId: toolCall.id }),
          ...(toolCall.function?.name && { toolName: toolCall.function.name }),
          ...(toolCall.function?.arguments && {
            args: toolCall.function?.arguments,
          }),
        },
      });
    }
  }
  return contentDeltas;
}

function calculateOpenAICost(
  usage: ModelUsage,
  pricing: NonNullable<OpenAIModelOptions["pricing"]>,
) {
  return (
    usage.inputTokens * pricing.inputTokensCost +
    usage.outputTokens * pricing.outputTokensCost
  );
}
