import OpenAI from "openai";
import type {
  LanguageModel,
  LanguageModelMetadata,
} from "../models/language-model.js";
import type {
  AssistantMessage,
  AudioContainer,
  AudioEncoding,
  AudioPart,
  ContentDelta,
  LanguageModelInput,
  ModelResponse,
  ModelTokensDetail,
  ModelUsage,
  PartialModelResponse,
  TextPart,
  Tool,
  ToolCallPart,
} from "../schemas/index.js";
import { convertAudioPartsToTextParts } from "../utils/message.utils.js";
import type { InternalContentDelta } from "../utils/stream.utils.js";
import { ContentDeltaAccumulator } from "../utils/stream.utils.js";
import { calculateCost } from "../utils/usage.utils.js";
import { OpenAIRefusedError } from "./errors.js";
import type {
  OpenAIModelOptions,
  OpenAIPatchedCompletionTokenDetails,
  OpenAIPatchedPromptTokensDetails,
} from "./types.js";

const OPENAI_AUDIO_SAMPLE_RATE = 24_000;
const OPENAI_AUDIO_CHANNELS = 1;

export type OpenAILanguageModelInput = LanguageModelInput & {
  extra?: Partial<OpenAI.Chat.Completions.ChatCompletionCreateParams>;
};

export class OpenAIModel implements LanguageModel {
  provider: string;
  modelId: string;
  metadata?: LanguageModelMetadata;

  private openai: OpenAI;

  constructor(
    public options: OpenAIModelOptions,
    metadata?: LanguageModelMetadata,
  ) {
    this.provider = "openai";
    this.modelId = options.modelId;
    if (metadata) this.metadata = metadata;

    this.openai = new OpenAI({
      baseURL: options.baseURL,
      apiKey: options.apiKey,
    });
  }

  async generate(input: OpenAILanguageModelInput): Promise<ModelResponse> {
    const openaiInput = {
      ...convertToOpenAIParams(input, this.options),
      stream: false,
    } satisfies OpenAI.Chat.ChatCompletionCreateParams;

    const response = await this.openai.chat.completions.create(openaiInput);

    if (!response.choices[0]) {
      throw new Error("no choices in response");
    }

    const choice = response.choices[0];

    if (choice.message.refusal) {
      throw new OpenAIRefusedError(choice.message.refusal);
    }

    const usage = response.usage ? mapOpenAIUsage(response.usage) : undefined;

    return {
      content: mapOpenAIMessage(choice.message, openaiInput).content,
      ...(usage && { usage }),
      ...(this.metadata?.pricing &&
        usage && {
          cost: calculateCost(usage, this.metadata.pricing),
        }),
    };
  }

  async *stream(
    input: OpenAILanguageModelInput,
  ): AsyncGenerator<PartialModelResponse, ModelResponse> {
    const openaiInput = {
      ...convertToOpenAIParams(input, this.options),
      stream: true,
      stream_options: {
        include_usage: true,
      },
    } satisfies OpenAI.Chat.ChatCompletionCreateParams;

    const stream = await this.openai.chat.completions.create(openaiInput);

    let usage: ModelUsage | undefined;
    let refusal = "";
    const accumulator = new ContentDeltaAccumulator();

    for await (const chunk of stream) {
      const choice = (
        chunk.choices as
          | OpenAI.Chat.Completions.ChatCompletionChunk.Choice[]
          | undefined
      )?.[0];
      const completion = choice;

      if (completion?.delta.refusal) {
        refusal += completion.delta.refusal;
      }

      if (completion?.delta) {
        const incomingContentDeltas = mapOpenAIDelta(
          completion.delta,
          openaiInput,
          accumulator.deltas,
        );

        accumulator.addChunks(incomingContentDeltas);

        for (const delta of incomingContentDeltas) {
          yield { delta };
        }
      }

      if (chunk.usage) {
        usage = mapOpenAIUsage(chunk.usage);
      }
    }

    if (refusal) {
      throw new OpenAIRefusedError(refusal);
    }

    return {
      content: accumulator.computeContent(),
      ...(usage && { usage }),
      ...(this.metadata?.pricing &&
        usage && {
          cost: calculateCost(usage, this.metadata.pricing),
        }),
    };
  }
}

export function convertToOpenAIParams(
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
    model: options.modelId,
    messages: convertToOpenAIMessages(input, options),
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
  input: Pick<LanguageModelInput, "messages" | "systemPrompt">,
  options: OpenAIModelOptions,
): OpenAI.Chat.ChatCompletionMessageParam[] {
  const openaiMessages: OpenAI.Chat.ChatCompletionMessageParam[] = [];
  let messages = input.messages;

  if (options.convertAudioPartsToTextParts) {
    messages = messages.map(convertAudioPartsToTextParts);
  }

  if (input.systemPrompt) {
    openaiMessages.push({
      role: "system",
      content: input.systemPrompt,
    });
  }

  for (const message of messages) {
    switch (message.role) {
      case "assistant": {
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
            case "audio": {
              throw new Error(
                "Audio parts are not supported in assistant messages. Use options.convertAudioPartsToTextParts" +
                  " to convert audio parts to text parts.",
              );
            }
            default: {
              throw new Error(
                `Unsupported message part type: ${(part as { type: string }).type}`,
              );
            }
          }
        });
        openaiMessages.push(openaiMessageParam);
        break;
      }
      case "tool": {
        message.content.forEach((toolResult) => {
          openaiMessages.push({
            role: "tool",
            content: JSON.stringify(toolResult.result),
            tool_call_id: toolResult.toolCallId,
          });
        });
        break;
      }
      case "user": {
        const contentParts = message.content;
        openaiMessages.push({
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
                      format: (convertToOpenAIAudioFormat(
                        part.container,
                        part.encoding,
                      ) ||
                        "wav") as OpenAI.ChatCompletionContentPartInputAudio.InputAudio["format"],
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
        });
        break;
      }
      default: {
        const exhaustiveCheck: never = message;
        throw new Error(
          `Unsupported message role: ${(exhaustiveCheck as { role: string }).role}`,
        );
      }
    }
  }

  return openaiMessages;
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
  if (!toolChoice) return undefined;

  switch (toolChoice.type) {
    case "tool":
      return {
        type: "function",
        function: {
          name: toolChoice.toolName,
        },
      };
    default:
      // 1-1 mapping with openai tool choice
      return toolChoice.type;
  }
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
        ...(tool.parameters && {
          parameters: tool.parameters,
        }),
      },
    }),
  );
}
// based on:
// - OpenAI.Chat.ChatCompletionAudioParam["format"]
// - https://platform.openai.com/docs/guides/speech-to-text
// - https://platform.openai.com/docs/api-reference/realtime-client-events/session/update
// - https://platform.openai.com/docs/guides/text-to-speech#supported-output-formats
// The returned type might not match the current specification on the OpenAI API
// but we still return in case future support is added
type PossibleOpenAIAudioFormat =
  | "wav"
  | "mp3"
  | "flac"
  | "opus"
  | "aac"
  | "pcm16"
  | "g711_ulaw"
  | "g711_alaw";

/**
 * Maps the audio container and encoding to the OpenAI audio format.
 * Loosely maps based on either container or encoding when defined.
 */
export function convertToOpenAIAudioFormat(
  container: AudioContainer | undefined,
  encoding: AudioEncoding | undefined,
): PossibleOpenAIAudioFormat | undefined {
  if (!container && !encoding) return undefined;

  const encodingMapping: {
    [key in AudioEncoding]: PossibleOpenAIAudioFormat;
  } = {
    flac: "flac",
    mp3: "mp3",
    opus: "opus",
    linear16: "pcm16",
    mulaw: "g711_ulaw",
    alaw: "g711_alaw",
    aac: "aac",
  };
  const containerMapping: {
    [key in AudioContainer]: PossibleOpenAIAudioFormat | undefined;
  } = {
    wav: "wav",
    ogg: "opus",
    flac: "flac",
    webm: undefined,
  };
  if (container) {
    return containerMapping[container];
  }
  if (encoding) {
    return encodingMapping[encoding];
  }
  return undefined;
}

export function convertToOpenAIResponseFormat(
  responseFormat: LanguageModelInput["responseFormat"],
  options: Pick<OpenAIModelOptions, "structuredOutputs">,
):
  | OpenAI.Chat.Completions.ChatCompletionCreateParams["response_format"]
  | undefined {
  if (!responseFormat) return undefined;

  switch (responseFormat.type) {
    case "json":
      if (options.structuredOutputs && responseFormat.schema) {
        const schemaTitle = responseFormat.schema["title"] as
          | string
          | undefined;
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
      }
      return { type: "json_object" };
    case "text":
      return { type: "text" };
    default: {
      const exhaustiveCheck: never = responseFormat;
      throw new Error(
        `Unsupported response format: ${(exhaustiveCheck as { type: string }).type}`,
      );
    }
  }
}

export function mapOpenAIAudioFormat(format: PossibleOpenAIAudioFormat): {
  container?: AudioContainer;
  encoding?: AudioEncoding;
} {
  switch (format) {
    case "wav":
      return { container: "wav", encoding: "linear16" };
    case "mp3":
      return { encoding: "mp3" };
    case "flac":
      return { container: "flac", encoding: "flac" };
    case "opus":
      return { encoding: "opus" };
    case "pcm16":
      return { encoding: "linear16" };
    case "aac":
      return { encoding: "aac" };
    case "g711_ulaw":
      return { encoding: "mulaw" };
    case "g711_alaw":
      return { encoding: "alaw" };
    default: {
      const exhaustiveCheck: never = format;
      throw new Error(`Unsupported audio format: ${exhaustiveCheck as string}`);
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
              ...mapOpenAIAudioFormat(options?.audio?.format || "pcm16"),
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
              args: JSON.parse(toolCall.function.arguments) as {
                [key: string]: unknown;
              },
            }),
          )
        : []),
    ],
  };
}

export function mapOpenAIDelta(
  delta: OpenAI.Chat.Completions.ChatCompletionChunk.Choice.Delta & {
    audio?: { id?: string; data?: string; transcript?: string };
  },
  options?: Partial<OpenAI.Chat.ChatCompletionCreateParams>,
  existingContentDeltas?: InternalContentDelta[],
): ContentDelta[] {
  const contentDeltas: ContentDelta[] = [];

  // OpenAI will only either have text, audio, or tool_calls
  // so we can assume that the first non-tool call index is 0
  const nonToolCallIndex = 0;

  // However, it has been noticed that OpenAI will sometimes
  // responds with a tool call after responding with text parts
  // therefore, we must increase the indexes of tool calls by this
  // TODO: we need to find a better solution here to map the indexes
  // correctly
  const nonToolCallCount =
    existingContentDeltas?.filter((delta) => delta.part.type !== "tool-call")
      .length || 0;

  if (delta.content) {
    contentDeltas.push({
      index: nonToolCallIndex,
      part: { type: "text", text: delta.content },
    });
  }
  if (delta.audio) {
    contentDeltas.push({
      index: nonToolCallIndex,
      part: {
        type: "audio",
        ...(delta.audio.data && {
          audioData: delta.audio.data,
          ...mapOpenAIAudioFormat(options?.audio?.format || "pcm16"),
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
        index: nonToolCallCount + toolCall.index,
        part: {
          type: "tool-call",
          ...(toolCall.id && { toolCallId: toolCall.id }),
          ...(toolCall.function?.name && { toolName: toolCall.function.name }),
          ...(toolCall.function?.arguments && {
            args: toolCall.function.arguments,
          }),
        },
      });
    }
  }
  return contentDeltas;
}

export function mapOpenAIUsage(usage: OpenAI.CompletionUsage): ModelUsage {
  return {
    inputTokens: usage.prompt_tokens,
    outputTokens: usage.completion_tokens,
    ...(usage.prompt_tokens_details && {
      inputTokensDetail: mapOpenAIPromptTokensDetails(
        usage.prompt_tokens_details as OpenAIPatchedPromptTokensDetails,
      ),
      outputTokensDetail: mapOpenAICompletionTokenDetails(
        usage.completion_tokens_details as OpenAIPatchedCompletionTokenDetails,
      ),
    }),
  };
}

export function mapOpenAIPromptTokensDetails(
  details: OpenAIPatchedPromptTokensDetails,
): ModelTokensDetail {
  return {
    textTokens: details.text_tokens,
    imageTokens: details.image_tokens,
    audioTokens: details.audio_tokens,
  };
}

export function mapOpenAICompletionTokenDetails(
  details: OpenAIPatchedCompletionTokenDetails,
): ModelTokensDetail {
  return {
    textTokens: details.text_tokens,
    audioTokens: details.audio_tokens,
    // note: reasoning_tokens is included in output_tokens
  };
}
