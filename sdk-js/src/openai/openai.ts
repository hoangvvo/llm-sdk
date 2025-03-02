import OpenAI from "openai";
import {
  InvalidValueError,
  ModelUnsupportedMessagePart,
  NotImplementedError,
} from "../errors/errors.js";
import {
  LanguageModel,
  type LanguageModelMetadata,
} from "../models/language-model.js";
import type {
  AssistantMessage,
  AudioFormat,
  AudioPart,
  AudioPartDelta,
  ContentDelta,
  LanguageModelInput,
  ModelResponse,
  ModelTokensDetails,
  ModelUsage,
  PartialModelResponse,
  TextPartDelta,
  Tool,
  ToolCallPartDelta,
} from "../types.js";
import { convertAudioPartsToTextParts } from "../utils/message.utils.js";
import type { InternalContentDelta } from "../utils/stream.utils.js";
import {
  ContentDeltaAccumulator,
  guessDeltaIndex,
} from "../utils/stream.utils.js";
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

export class OpenAIModel extends LanguageModel {
  provider: string;
  modelId: string;
  metadata?: LanguageModelMetadata;

  private openai: OpenAI;

  constructor(
    public options: OpenAIModelOptions,
    metadata?: LanguageModelMetadata,
  ) {
    super();
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

    const usage = response.usage
      ? mapOpenAIUsage(response.usage, input)
      : undefined;

    const result: ModelResponse = {
      content: mapOpenAIMessage(choice.message, openaiInput).content,
    };
    if (usage) {
      result.usage = usage;
      if (this.metadata?.pricing) {
        result.cost = calculateCost(usage, this.metadata.pricing);
      }
    }

    return result;
  }

  async *stream(
    input: OpenAILanguageModelInput,
  ): AsyncGenerator<PartialModelResponse, ModelResponse> {
    const openaiInput: OpenAI.Chat.ChatCompletionCreateParams = {
      ...convertToOpenAIParams(input, this.options),
      stream: true,
      stream_options: {
        include_usage: true,
      },
    };

    const stream = await this.openai.chat.completions.create(openaiInput);

    let usage: ModelUsage | undefined;
    let refusal = "";
    const accumulator = new ContentDeltaAccumulator();

    for await (const chunk of stream) {
      const choices = chunk.choices as
        | OpenAI.Chat.Completions.ChatCompletionChunk.Choice[]
        | undefined;
      const choice = choices ? choices[0] : undefined;
      if (choice && choice.delta.refusal) {
        refusal += choice.delta.refusal;
      }
      if (choice?.delta) {
        const incomingContentDeltas = mapOpenAIDelta(
          choice.delta,
          accumulator.deltas,
          openaiInput,
        );
        accumulator.addChunks(incomingContentDeltas);
        for (const delta of incomingContentDeltas) {
          yield { delta };
        }
      }
      if (chunk.usage) {
        usage = mapOpenAIUsage(chunk.usage, input);
      }
    }

    if (refusal) {
      throw new OpenAIRefusedError(refusal);
    }

    const finalResult: ModelResponse = {
      content: accumulator.computeContent(),
    };
    if (usage) {
      finalResult.usage = usage;
      if (this.metadata?.pricing) {
        finalResult.cost = calculateCost(usage, this.metadata.pricing);
      }
    }

    return finalResult;
  }
}

export function convertToOpenAIParams(
  input: LanguageModelInput,
  options: OpenAIModelOptions,
): OpenAI.Chat.ChatCompletionCreateParams {
  const params: OpenAI.Chat.ChatCompletionCreateParams = {
    model: options.modelId,
    messages: convertToOpenAIMessages(input, options),
    ...convertToOpenAISamplingParams(input),
    ...input.extra,
  };
  if (input.tools) {
    params.tools = input.tools.map((tool) =>
      convertToOpenAITool(tool, options),
    );
  }
  if (input.tool_choice) {
    params.tool_choice = convertToOpenAIToolChoice(input.tool_choice);
  }
  if (input.response_format) {
    params.response_format = convertToOpenAIResponseFormat(
      input.response_format,
      options,
    );
  }
  if (input.modalities) {
    params.modalities = input.modalities;
  }
  if (input.metadata) {
    params.metadata = input.metadata;
  }
  return params;
}

export function convertToOpenAIMessages(
  input: Pick<LanguageModelInput, "messages" | "system_prompt">,
  options: OpenAIModelOptions,
): OpenAI.Chat.ChatCompletionMessageParam[] {
  const openaiMessages: OpenAI.Chat.ChatCompletionMessageParam[] = [];
  let messages = input.messages;
  if (options.convertAudioPartsToTextParts) {
    messages = messages.map(convertAudioPartsToTextParts);
  }
  if (input.system_prompt) {
    openaiMessages.push({
      role: "system",
      content: input.system_prompt,
    });
  }
  messages.forEach((message) => {
    switch (message.role) {
      case "assistant": {
        const openaiMessageParam: Omit<
          OpenAI.Chat.ChatCompletionAssistantMessageParam,
          "content"
        > & {
          content: Array<OpenAI.Chat.ChatCompletionContentPartText> | null;
        } = {
          role: "assistant",
          content: null,
        };
        message.content.forEach((part) => {
          switch (part.type) {
            case "text": {
              openaiMessageParam.content = openaiMessageParam.content || [];
              openaiMessageParam.content.push({
                type: "text",
                text: part.text,
              });
              break;
            }
            case "tool-call": {
              openaiMessageParam.tool_calls =
                openaiMessageParam.tool_calls || [];
              openaiMessageParam.tool_calls.push({
                type: "function",
                id: part.tool_call_id,
                function: {
                  name: part.tool_name,
                  arguments: JSON.stringify(part.args),
                },
              });
              break;
            }
            case "audio": {
              if (!part.id) {
                throw new Error("audio part must have an id");
              }
              openaiMessageParam.audio = { id: part.id };
              break;
            }
            case "image":
            case "tool-result":
              throw new ModelUnsupportedMessagePart("openai", message, part);

            default: {
              const exhaustiveCheck: never = part;
              throw new InvalidValueError(
                "message.part.type",
                (exhaustiveCheck as { type: string }).type,
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
            tool_call_id: toolResult.tool_call_id,
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
                      url: `data:${part.mime_type};base64,${part.image_data}`,
                    },
                  };
                }
                case "audio": {
                  if (!part.format) {
                    throw new Error("audio part must have a format");
                  }
                  return {
                    type: "input_audio",
                    input_audio: {
                      // this as assertion is not correct, but we will rely on OpenAI to throw an error
                      format: (AUDIO_FORMAT_MAP[part.format] ||
                        "wav") as OpenAI.ChatCompletionContentPartInputAudio.InputAudio["format"],
                      data: part.audio_data,
                    },
                  };
                }
                case "tool-call":
                case "tool-result":
                  throw new ModelUnsupportedMessagePart(
                    "openai",
                    message,
                    part,
                  );
                default: {
                  const exhaustiveCheck: never = part;
                  throw new InvalidValueError(
                    "message.part.type",
                    (exhaustiveCheck as { type: string }).type,
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
        throw new InvalidValueError(
          "message.role",
          (exhaustiveCheck as { role: string }).role,
        );
      }
    }
  });
  return openaiMessages;
}

export function convertToOpenAISamplingParams(
  input: Partial<LanguageModelInput>,
): Partial<OpenAI.Chat.ChatCompletionCreateParams> {
  const sampling: Partial<OpenAI.Chat.ChatCompletionCreateParams> = {};
  if (typeof input.max_tokens === "number") {
    sampling.max_tokens = input.max_tokens;
  }
  if (typeof input.temperature === "number") {
    sampling.temperature = input.temperature;
  }
  if (typeof input.top_p === "number") {
    sampling.top_p = input.top_p;
  }
  if (typeof input.presence_penalty === "number") {
    sampling.presence_penalty = input.presence_penalty;
  }
  if (typeof input.frequency_penalty === "number") {
    sampling.frequency_penalty = input.frequency_penalty;
  }
  if (typeof input.seed === "number") {
    sampling.seed = input.seed;
  }
  return sampling;
}

export function convertToOpenAIToolChoice(
  toolChoice: NonNullable<LanguageModelInput["tool_choice"]>,
): OpenAI.Chat.Completions.ChatCompletionToolChoiceOption {
  switch (toolChoice.type) {
    case "tool":
      return {
        type: "function",
        function: {
          name: toolChoice.tool_name,
        },
      };
    default:
      return toolChoice.type;
  }
}

export function convertToOpenAITool(
  tool: Tool,
  options: OpenAIModelOptions,
): OpenAI.Chat.Completions.ChatCompletionTool {
  const openaiTool: OpenAI.Chat.Completions.ChatCompletionTool = {
    type: "function",
    function: {
      name: tool.name,
      description: tool.description,
    },
  };
  if (options.structuredOutputs) {
    openaiTool.function.strict = true;
  }
  if (tool.parameters) {
    openaiTool.function.parameters = tool.parameters;
  }
  return openaiTool;
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
 * Maps the audio format to the OpenAI audio format.
 */
const AUDIO_FORMAT_MAP: Record<
  AudioFormat,
  PossibleOpenAIAudioFormat | undefined
> = {
  wav: "wav",
  mp3: "mp3",
  linear16: "pcm16",
  flac: "flac",
  mulaw: "g711_ulaw",
  alaw: "g711_alaw",
  aac: "aac",
  opus: "opus",
};

export function convertToOpenAIResponseFormat(
  responseFormat: NonNullable<LanguageModelInput["response_format"]>,
  options: OpenAIModelOptions,
): NonNullable<
  OpenAI.Chat.Completions.ChatCompletionCreateParams["response_format"]
> {
  switch (responseFormat.type) {
    case "json":
      if (options.structuredOutputs && responseFormat.schema) {
        const schemaDescription = responseFormat.description;
        const json_schema: OpenAI.ResponseFormatJSONSchema["json_schema"] = {
          strict: true,
          name: responseFormat.name,
          schema: responseFormat.schema,
        };
        if (schemaDescription) {
          json_schema.description = schemaDescription;
        }
        return {
          type: "json_schema",
          json_schema,
        };
      }
      return { type: "json_object" };
    case "text":
      return { type: "text" };
    default: {
      const exhaustiveCheck: never = responseFormat;
      throw new InvalidValueError(
        "responseFormat.type",
        (exhaustiveCheck as { type: string }).type,
      );
    }
  }
}

export function mapOpenAIAudioFormat(
  format: PossibleOpenAIAudioFormat,
): AudioFormat {
  switch (format) {
    case "wav":
      return "wav";
    case "mp3":
      return "mp3";
    case "flac":
      return "flac";
    case "opus":
      return "opus";
    case "pcm16":
      return "linear16";
    case "aac":
      return "aac";
    case "g711_ulaw":
      return "mulaw";
    case "g711_alaw":
      return "alaw";
    default: {
      const exhaustiveCheck: never = format;
      throw new NotImplementedError(
        "format",
        (exhaustiveCheck as { type: string }).type,
      );
    }
  }
}

export function mapOpenAIMessage(
  message: OpenAI.Chat.Completions.ChatCompletionMessage,
  options?: Partial<OpenAI.Chat.ChatCompletionCreateParams>,
): AssistantMessage {
  const content: AssistantMessage["content"] = [];

  if (message.content) {
    content.push({
      type: "text",
      text: message.content,
    });
  }

  if (message.audio) {
    const audioPart: AudioPart = { type: "audio", audio_data: "" };
    audioPart.id = message.audio.id;
    audioPart.format = mapOpenAIAudioFormat(options?.audio?.format || "pcm16");
    if (audioPart.format === "linear16") {
      audioPart.sample_rate = OPENAI_AUDIO_SAMPLE_RATE;
      audioPart.channels = OPENAI_AUDIO_CHANNELS;
    }
    audioPart.audio_data = message.audio.data;
    audioPart.transcript = message.audio.transcript;
    content.push(audioPart);
  }

  if (message.tool_calls) {
    message.tool_calls.forEach((toolCall) => {
      const args = JSON.parse(toolCall.function.arguments) as {
        [key: string]: unknown;
      };
      content.push({
        type: "tool-call",
        tool_call_id: toolCall.id,
        tool_name: toolCall.function.name,
        args,
      });
    });
  }

  return {
    role: "assistant",
    content,
  };
}

export function mapOpenAIDelta(
  delta: OpenAI.Chat.Completions.ChatCompletionChunk.Choice.Delta & {
    audio?: { id?: string; data?: string; transcript?: string };
  },
  existingContentDeltas: InternalContentDelta[],
  options: Partial<OpenAI.Chat.ChatCompletionCreateParams>,
): ContentDelta[] {
  const contentDeltas: ContentDelta[] = [];

  if (delta.content) {
    const part: TextPartDelta = {
      type: "text",
      text: delta.content,
    };
    contentDeltas.push({
      index: guessDeltaIndex(part, [
        ...existingContentDeltas,
        ...contentDeltas,
      ]),
      part,
    });
  }
  if (delta.audio) {
    const part: AudioPartDelta = { type: "audio" };
    if (delta.audio.id) {
      part.id = delta.audio.id;
    }
    if (delta.audio.data) {
      part.audio_data = delta.audio.data;
      part.format = mapOpenAIAudioFormat(options.audio?.format || "pcm16");
      part.sample_rate = OPENAI_AUDIO_SAMPLE_RATE;
      part.channels = OPENAI_AUDIO_CHANNELS;
    }
    if (delta.audio.transcript) {
      part.transcript = delta.audio.transcript;
    }
    contentDeltas.push({
      index: guessDeltaIndex(part, [
        ...existingContentDeltas,
        ...contentDeltas,
      ]),
      part,
    });
  }
  if (delta.tool_calls) {
    const allExistingToolCalls = existingContentDeltas.filter(
      (delta) => delta.part.type === "tool-call",
    );
    delta.tool_calls.forEach((toolCall) => {
      const existingDelta = allExistingToolCalls[toolCall.index];

      const part: ToolCallPartDelta = { type: "tool-call" };
      if (toolCall.id) {
        part.tool_call_id = toolCall.id;
      }
      if (toolCall.function?.name) {
        part.tool_name = toolCall.function.name;
      }
      if (toolCall.function?.arguments) {
        part.args = toolCall.function.arguments;
      }
      contentDeltas.push({
        index: guessDeltaIndex(
          part,
          [...existingContentDeltas, ...contentDeltas],
          existingDelta,
        ),
        part,
      });
    });
  }
  return contentDeltas;
}

export function mapOpenAIUsage(
  usage: OpenAI.CompletionUsage,
  input: OpenAILanguageModelInput,
): ModelUsage {
  const result: ModelUsage = {
    input_tokens: usage.prompt_tokens,
    output_tokens: usage.completion_tokens,
  };
  if (usage.prompt_tokens_details) {
    result.input_tokens_details = mapOpenAIPromptTokensDetails(
      usage.prompt_tokens_details as OpenAIPatchedPromptTokensDetails,
      input,
    );
    result.output_tokens_details = mapOpenAICompletionTokenDetails(
      usage.completion_tokens_details as OpenAIPatchedCompletionTokenDetails,
    );
  }
  return result;
}

export function mapOpenAIPromptTokensDetails(
  details: OpenAIPatchedPromptTokensDetails,
  input: OpenAILanguageModelInput,
): ModelTokensDetails {
  const textTokens = details.text_tokens;
  const audioTokens = details.audio_tokens;
  const imageTokens = details.image_tokens;
  const hasTextPart = input.messages.some(
    (s) => s.role === "user" && s.content.some((p) => p.type === "text"),
  );
  const hasAudioPart = input.messages.some(
    (s) => s.role === "user" && s.content.some((p) => p.type === "audio"),
  );
  const cachedTextTokens =
    details.cached_tokens_details?.text_tokens ??
    // Guess that cached tokens are for text if there are text messages
    (hasTextPart ? details.cached_tokens : undefined);
  const cachedAudioTokens =
    details.cached_tokens_details?.audio_tokens ??
    // Guess that cached tokens are for audio if there are audio messages
    (hasAudioPart ? details.cached_tokens : undefined);
  const result: ModelTokensDetails = {};
  if (typeof textTokens === "number") {
    result.text_tokens = textTokens;
  }
  if (typeof audioTokens === "number") {
    result.audio_tokens = audioTokens;
  }
  if (typeof imageTokens === "number") {
    result.image_tokens = imageTokens;
  }
  if (typeof cachedTextTokens === "number") {
    result.cached_text_tokens = cachedTextTokens;
  }
  if (typeof cachedAudioTokens === "number") {
    result.cached_audio_tokens = cachedAudioTokens;
  }
  return result;
}

export function mapOpenAICompletionTokenDetails(
  details: OpenAIPatchedCompletionTokenDetails,
): ModelTokensDetails {
  const result: ModelTokensDetails = {};
  if (details.text_tokens) {
    result.text_tokens = details.text_tokens;
  }
  if (details.audio_tokens) {
    result.audio_tokens = details.audio_tokens;
  }
  return result;
}
