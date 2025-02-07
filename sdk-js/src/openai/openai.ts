import OpenAI from "openai";
import {
  InvalidInputError,
  InvariantError,
  RefusalError,
  UnsupportedError,
} from "../errors.ts";
import type { LanguageModel } from "../language-model.ts";
import { type LanguageModelMetadata } from "../language-model.ts";
import { SDKSpan } from "../opentelemetry.ts";
import { getCompatiblePartsWithoutSourceParts } from "../source-part.utils.ts";
import { guessDeltaIndex } from "../stream.utils.ts";
import type {
  AudioFormat,
  AudioPart,
  AudioPartDelta,
  ContentDelta,
  ImagePart,
  LanguageModelInput,
  Message,
  Modality,
  ModelResponse,
  ModelTokensDetails,
  ModelUsage,
  Part,
  PartialModelResponse,
  ResponseFormatOption,
  TextPart,
  TextPartDelta,
  Tool,
  ToolCallPart,
  ToolCallPartDelta,
  ToolChoiceOption,
} from "../types.ts";
import { calculateCost } from "../usage.utils.ts";
import type {
  OpenAIPatchedCompletionTokenDetails,
  OpenAIPatchedPromptTokensDetails,
} from "./types.ts";

const PROVIDER = "openai";

const OPENAI_AUDIO_SAMPLE_RATE = 24_000;
const OPENAI_AUDIO_CHANNELS = 1;

export interface OpenAIModelOptions {
  baseURL?: string;
  apiKey: string;
  modelId: string;
}

export class OpenAIModel implements LanguageModel {
  provider: string;
  modelId: string;
  metadata?: LanguageModelMetadata;

  #openai: OpenAI;

  constructor(options: OpenAIModelOptions, metadata?: LanguageModelMetadata) {
    this.provider = PROVIDER;
    this.modelId = options.modelId;
    if (metadata) this.metadata = metadata;
    this.#openai = new OpenAI({
      baseURL: options.baseURL,
      apiKey: options.apiKey,
    });
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const span = new SDKSpan(this.provider, this.modelId, "generate", input);

    try {
      const createParams = convertToOpenAICreateParams(input, this.modelId);

      const response = await this.#openai.chat.completions.create({
        ...createParams,
        stream: false,
      });

      const choice = response.choices[0];
      if (!choice) {
        throw new InvariantError(PROVIDER, "No choices in response");
      }
      const { message } = choice;

      if (message.refusal) {
        throw new RefusalError(message.refusal);
      }

      const content = mapOpenAIMessage(message, createParams);

      const result: ModelResponse = {
        content,
      };

      if (response.usage) {
        result.usage = mapOpenAIUsage(response.usage, input);
        if (this.metadata?.pricing) {
          result.cost = calculateCost(result.usage, this.metadata.pricing);
        }
      }

      span.onEnd(result);

      return result;
    } catch (error) {
      span.onError(error);
      throw error;
    }
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse> {
    const span = new SDKSpan(this.provider, this.modelId, "stream", input);

    try {
      const createParams = convertToOpenAICreateParams(input, this.modelId);

      const stream = await this.#openai.chat.completions.create({
        ...createParams,
        stream: true,
        stream_options: {
          include_usage: true,
        },
      });

      let refusal = "";

      const allContentDeltas: ContentDelta[] = [];

      for await (const chunk of stream) {
        // It is possible for choices to be empty
        // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
        const choice = chunk.choices?.[0];
        if (choice) {
          if (choice.delta.refusal) {
            refusal += choice.delta.refusal;
          }

          const incomingContentDeltas = mapOpenAIDelta(
            choice.delta,
            allContentDeltas,
            createParams,
          );

          allContentDeltas.push(...incomingContentDeltas);

          for (const delta of incomingContentDeltas) {
            const event: PartialModelResponse = { delta };
            yield event;
            span.onStreamPartial(event);
          }
        }
        if (chunk.usage) {
          const usage = mapOpenAIUsage(chunk.usage, input);
          const event: PartialModelResponse = { usage };
          yield event;
          span.onStreamPartial(event);
        }
      }

      if (refusal) {
        throw new RefusalError(refusal);
      }

      span.onStreamEnd();
    } catch (error) {
      span.onError(error);
      throw error;
    }
  }
}

function convertToOpenAICreateParams(
  input: LanguageModelInput,
  modelId: string,
): Omit<OpenAI.Chat.ChatCompletionCreateParams, "stream"> {
  const {
    messages,
    system_prompt,
    max_tokens,
    temperature,
    top_p,
    presence_penalty,
    frequency_penalty,
    seed,
    response_format,
    tools,
    tool_choice,
    modalities,
    extra,
  } = input;
  return {
    model: modelId,
    messages: convertToOpenAIMessages(messages, system_prompt),
    max_tokens: max_tokens ?? null,
    temperature: temperature ?? null,
    top_p: top_p ?? null,
    presence_penalty: presence_penalty ?? null,
    frequency_penalty: frequency_penalty ?? null,
    seed: seed ?? null,
    ...(tools && {
      tools: tools.map(convertToOpenAITool),
    }),
    ...(tool_choice && {
      tool_choice: convertToOpenAIToolChoice(tool_choice),
    }),
    ...(response_format && {
      response_format: convertToOpenAIResponseFormat(response_format),
    }),
    modalities: modalities?.map(convertToOpenAIModality) ?? null,
    ...extra,
  };
}

// MARK: To Provider Messages

function convertToOpenAIMessages(
  messages: Message[],
  systemPrompt: string | undefined,
): OpenAI.Chat.ChatCompletionMessageParam[] {
  const openaiMessages: OpenAI.Chat.ChatCompletionMessageParam[] = [];
  if (systemPrompt) {
    openaiMessages.push({
      role: "system",
      content: systemPrompt,
    });
  }
  messages.forEach((message) => {
    switch (message.role) {
      case "user": {
        const messageParts = getCompatiblePartsWithoutSourceParts(
          message.content,
        );

        const openaiMessageParam: OpenAI.Chat.ChatCompletionUserMessageParam = {
          role: "user",
          content: messageParts.map(convertToOpenAIContentPart).flat(),
        };

        openaiMessages.push(openaiMessageParam);
        break;
      }

      case "assistant": {
        const openaiMessageParam: Omit<
          OpenAI.Chat.ChatCompletionAssistantMessageParam,
          "content"
        > & {
          content: OpenAI.Chat.ChatCompletionContentPartText[] | null;
        } = {
          role: "assistant",
          content: null,
        };

        const messageParts = getCompatiblePartsWithoutSourceParts(
          message.content,
        );

        messageParts.forEach((part) => {
          switch (part.type) {
            case "text": {
              openaiMessageParam.content = openaiMessageParam.content ?? [];
              openaiMessageParam.content.push(
                convertToOpenAIContentPartText(part),
              );
              break;
            }
            case "tool-call": {
              openaiMessageParam.tool_calls =
                openaiMessageParam.tool_calls ?? [];
              openaiMessageParam.tool_calls.push(convertToOpenAIToolCall(part));
              break;
            }
            case "audio": {
              openaiMessageParam.audio =
                convertToOpenAIAssistantMessageParamAudio(part);
              break;
            }
            default: {
              throw new UnsupportedError(
                PROVIDER,
                `Cannot convert part to OpenAI assistant message for type ${part.type}`,
              );
            }
          }
        });
        openaiMessages.push(openaiMessageParam);
        break;
      }

      case "tool": {
        message.content.forEach((part) => {
          if (part.type !== "tool-result") {
            throw new InvalidInputError(
              "Tool messages must contain only tool result parts",
            );
          }

          const toolResultPartContent = getCompatiblePartsWithoutSourceParts(
            part.content,
          );

          openaiMessages.push({
            role: "tool",
            tool_call_id: part.tool_call_id,
            content: toolResultPartContent.map(
              convertToOpenAIToolMessageParamContent,
            ),
          });
        });
        break;
      }
    }
  });
  return openaiMessages;
}

function convertToOpenAIContentPart(
  part: Part,
): OpenAI.Chat.ChatCompletionContentPart {
  switch (part.type) {
    case "text":
      return convertToOpenAIContentPartText(part);
    case "image":
      return convertToOpenAIContentPartImage(part);
    case "audio":
      return convertToOpenAIContentPartInputAudio(part);
    default:
      throw new UnsupportedError(
        PROVIDER,
        `Cannot convert part to OpenAI content part for type ${part.type}`,
      );
  }
}

function convertToOpenAIContentPartText(
  part: TextPart,
): OpenAI.Chat.ChatCompletionContentPartText {
  return {
    type: "text",
    text: part.text,
  };
}

function convertToOpenAIContentPartImage(
  part: ImagePart,
): OpenAI.Chat.ChatCompletionContentPartImage {
  return {
    type: "image_url",
    image_url: {
      url: `data:${part.mime_type};base64,${part.image_data}`,
    },
  };
}

function convertToOpenAIContentPartInputAudio(
  part: AudioPart,
): OpenAI.Chat.ChatCompletionContentPartInputAudio {
  let format: OpenAI.Chat.ChatCompletionContentPartInputAudio.InputAudio["format"];
  switch (part.format) {
    case "mp3":
      format = "mp3";
      break;
    case "wav":
      format = "wav";
      break;
    default:
      throw new UnsupportedError(
        PROVIDER,
        `Cannot convert audio format to OpenAI InputAudio format for format ${part.format}`,
      );
  }
  return {
    type: "input_audio",
    input_audio: {
      data: part.audio_data,
      format,
    },
  };
}

function convertToOpenAIAssistantMessageParamAudio(
  part: AudioPart,
): OpenAI.Chat.Completions.ChatCompletionAssistantMessageParam.Audio {
  if (!part.audio_id) {
    throw new UnsupportedError(
      PROVIDER,
      "Cannot convert audio part to OpenAI assistant message without an ID",
    );
  }
  return {
    id: part.audio_id,
  };
}

function convertToOpenAIToolCall(
  part: ToolCallPart,
): OpenAI.Chat.Completions.ChatCompletionMessageToolCall {
  return {
    type: "function",
    id: part.tool_call_id,
    function: {
      name: part.tool_name,
      arguments: JSON.stringify(part.args),
    },
  };
}

function convertToOpenAIToolMessageParamContent(
  part: Part,
): Extract<
  OpenAI.ChatCompletionToolMessageParam["content"],
  unknown[]
>[number] {
  switch (part.type) {
    case "text":
      return convertToOpenAIContentPartText(part);
    default:
      throw new UnsupportedError(
        PROVIDER,
        `Cannot convert part to OpenAI tool message for type ${part.type}`,
      );
  }
}

// MARK: To Provider Tools

function convertToOpenAITool(
  tool: Tool,
): OpenAI.Chat.Completions.ChatCompletionTool {
  return {
    type: "function",
    function: {
      name: tool.name,
      description: tool.description,
      parameters: tool.parameters,
      strict: true,
    },
  };
}

function convertToOpenAIToolChoice(
  toolChoice: ToolChoiceOption,
): OpenAI.Chat.Completions.ChatCompletionToolChoiceOption {
  switch (toolChoice.type) {
    case "tool": {
      return {
        type: "function",
        function: {
          name: toolChoice.tool_name,
        },
      };
    }
    case "auto": {
      return "auto";
    }
    case "none": {
      return "none";
    }
    case "required": {
      return "required";
    }
  }
}

// MARK: To Provider Response Format

function convertToOpenAIResponseFormat(
  responseFormat: ResponseFormatOption,
): NonNullable<
  OpenAI.Chat.Completions.ChatCompletionCreateParams["response_format"]
> {
  switch (responseFormat.type) {
    case "json": {
      if (responseFormat.schema) {
        return {
          type: "json_schema",
          json_schema: {
            name: responseFormat.name,
            ...(responseFormat.description && {
              description: responseFormat.description,
            }),
            schema: responseFormat.schema,
            strict: true,
          },
        };
      }
      return {
        type: "json_object",
      };
    }
    case "text": {
      return { type: "text" };
    }
  }
}

// MARK: To Provider Modality

function convertToOpenAIModality(
  modality: Modality,
): NonNullable<OpenAI.Chat.ChatCompletionCreateParams["modalities"]>[number] {
  switch (modality) {
    case "text":
      return "text";
    case "audio":
      return "audio";
  }
}

// MARK: To SDK Message

function mapOpenAIMessage(
  message: OpenAI.Chat.Completions.ChatCompletionMessage,
  createParams: OpenAI.Chat.Completions.ChatCompletionCreateParams,
): Part[] {
  const parts: Part[] = [];

  if (message.content) {
    parts.push({
      type: "text",
      text: message.content,
    });
  }

  if (message.audio) {
    if (!createParams.audio) {
      throw new InvariantError(
        PROVIDER,
        "Audio returned from OpenAI API but no audio parameter was provided",
      );
    }
    const audioPart: AudioPart = {
      type: "audio",
      audio_data: message.audio.data,
      format: mapOpenAIAudioFormat(createParams.audio.format),
      audio_id: message.audio.id,
      transcript: message.audio.transcript,
    };
    if (audioPart.format == "linear16") {
      audioPart.sample_rate = OPENAI_AUDIO_SAMPLE_RATE;
      audioPart.channels = OPENAI_AUDIO_CHANNELS;
    }
    parts.push(audioPart);
  }

  if (message.tool_calls) {
    message.tool_calls.forEach((toolCall) => {
      parts.push(mapOpenAIFunctionToolCall(toolCall));
    });
  }

  return parts;
}

function mapOpenAIAudioFormat(
  format: OpenAI.Chat.Completions.ChatCompletionAudioParam["format"],
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
  }
}

function mapOpenAIFunctionToolCall(
  messageToolCall: OpenAI.Chat.Completions.ChatCompletionMessageToolCall,
): ToolCallPart {
  return {
    type: "tool-call",
    tool_call_id: messageToolCall.id,
    tool_name: messageToolCall.function.name,
    args: JSON.parse(messageToolCall.function.arguments) as Record<
      string,
      unknown
    >,
  };
}

// MARK: To SDK Delta

function mapOpenAIDelta(
  delta: OpenAI.Chat.Completions.ChatCompletionChunk.Choice.Delta & {
    audio?: { id?: string; data?: string; transcript?: string };
  },
  existingContentDeltas: ContentDelta[],
  createParams: Partial<OpenAI.Chat.ChatCompletionCreateParams>,
): ContentDelta[] {
  const contentDeltas: ContentDelta[] = [];

  if (delta.content) {
    const part: TextPartDelta = {
      type: "text",
      text: delta.content,
    };
    const index = guessDeltaIndex(part, [
      ...existingContentDeltas,
      ...contentDeltas,
    ]);
    contentDeltas.push({
      part,
      index,
    });
  }

  if (delta.audio) {
    const part: AudioPartDelta = { type: "audio" };
    if (delta.audio.id) {
      part.audio_id = delta.audio.id;
    }
    if (delta.audio.data) {
      part.audio_data = delta.audio.data;
      if (createParams.audio?.format) {
        part.format = mapOpenAIAudioFormat(createParams.audio.format);
      }
    }
    if (delta.audio.transcript) {
      part.transcript = delta.audio.transcript;
    }
    contentDeltas.push({
      part,
      index: guessDeltaIndex(part, [
        ...existingContentDeltas,
        ...contentDeltas,
      ]),
    });
  }

  if (delta.tool_calls) {
    delta.tool_calls.forEach((toolCall) => {
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
          toolCall.index,
        ),
        part,
      });
    });
  }

  return contentDeltas;
}

// MARK: To SDK Usage

function mapOpenAIUsage(
  usage: OpenAI.CompletionUsage,
  input: LanguageModelInput,
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

function mapOpenAIPromptTokensDetails(
  details: OpenAIPatchedPromptTokensDetails,
  input: LanguageModelInput,
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

function mapOpenAICompletionTokenDetails(
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
