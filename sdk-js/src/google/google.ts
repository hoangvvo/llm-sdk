import type {
  FunctionCallingConfig,
  FunctionDeclaration,
  Part as GooglePart,
  ModalityTokenCount,
  UsageMetadata,
} from "@google/genai";
import {
  FunctionCallingConfigMode,
  GoogleGenAI,
  MediaModality,
  type Content,
  type GenerateContentParameters,
} from "@google/genai";
import {
  mapAudioFormatToMimeType,
  mapMimeTypeToAudioFormat,
} from "../audio.utils.ts";
import {
  InvalidInputError,
  InvariantError,
  UnsupportedError,
} from "../errors.ts";
import { generateString } from "../id.utils.ts";
import type {
  LanguageModel,
  LanguageModelMetadata,
} from "../language-model.ts";
import { traceLanguageModel } from "../opentelemetry.ts";
import { getCompatiblePartsWithoutSourceParts } from "../source-part.utils.ts";
import {
  guessDeltaIndex,
  looselyConvertPartToPartDelta,
} from "../stream.utils.ts";
import type {
  ContentDelta,
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
  Tool,
  ToolChoiceOption,
} from "../types.ts";
import { calculateCost, sumModelTokensDetails } from "../usage.utils.ts";

const PROVIDER = "google";

export interface GoogleModelOptions {
  apiKey: string;
  modelId: string;
}

export class GoogleModel implements LanguageModel {
  provider: string;
  modelId: string;
  metadata?: LanguageModelMetadata;

  #ai: GoogleGenAI;

  constructor(options: GoogleModelOptions, metadata?: LanguageModelMetadata) {
    this.provider = PROVIDER;
    this.modelId = options.modelId;
    if (metadata) this.metadata = metadata;

    this.#ai = new GoogleGenAI({
      apiKey: options.apiKey,
    });

    traceLanguageModel(this);
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const params = convertToGenerateContentParameters(input, this.modelId);

    const response = await this.#ai.models.generateContent(params);

    const candidate = response.candidates?.[0];
    if (!candidate) {
      throw new InvariantError(PROVIDER, "No candidate in response");
    }

    const content = mapGoogleContent(candidate.content?.parts ?? []);
    const result: ModelResponse = { content };
    if (response.usageMetadata) {
      result.usage = mapGoogleUsageMetadata(response.usageMetadata);
      if (this.metadata?.pricing) {
        result.cost = calculateCost(result.usage, this.metadata.pricing);
      }
    }

    return result;
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse> {
    const params = convertToGenerateContentParameters(input, this.modelId);

    const stream = await this.#ai.models.generateContentStream(params);

    const allContentDeltas: ContentDelta[] = [];

    for await (const chunk of stream) {
      const candidate = chunk.candidates?.[0];

      if (candidate?.content) {
        const incomingContentDeltas = mapGoogleContentToDelta(
          candidate.content,
          allContentDeltas,
        );

        allContentDeltas.push(...incomingContentDeltas);

        for (const delta of incomingContentDeltas) {
          const event: PartialModelResponse = { delta };
          yield event;
        }
      }

      if (chunk.usageMetadata) {
        const usage = mapGoogleUsageMetadata(chunk.usageMetadata);
        const partial: PartialModelResponse = { usage };
        yield partial;
      }
    }
  }
}

function convertToGenerateContentParameters(
  input: LanguageModelInput,
  modelId: string,
): GenerateContentParameters {
  const {
    system_prompt,
    messages,
    tools,
    tool_choice,
    response_format,
    max_tokens,
    temperature,
    top_p,
    top_k,
    presence_penalty,
    frequency_penalty,
    seed,
    modalities,
    extra,
  } = input;

  const responseSchemaTuple = response_format
    ? convertToGoogleResponseSchema(response_format)
    : null;

  const params: GenerateContentParameters = {
    contents: convertToGoogleContents(messages),
    model: modelId,
    ...extra,
    config: {
      ...(system_prompt && { systemInstruction: system_prompt }),
      ...(temperature && { temperature }),
      ...(top_p && { topP: top_p }),
      ...(top_k && { topK: top_k }),
      ...(presence_penalty && { presencePenalty: presence_penalty }),
      ...(frequency_penalty && { frequencyPenalty: frequency_penalty }),
      ...(seed && { seed }),
      ...(max_tokens && { maxOutputTokens: max_tokens }),
      ...(tools && {
        tools: [{ functionDeclarations: tools.map(convertToGoogleTool) }],
      }),
      ...(responseSchemaTuple && {
        responseMimeType: responseSchemaTuple[0],
        responseJsonSchema: responseSchemaTuple[1],
      }),
      ...(tool_choice && {
        toolConfig: {
          functionCallingConfig:
            convertToGoogleFunctionCallingConfig(tool_choice),
        },
      }),
      ...(modalities && {
        responseModalities: modalities.map(convertToGoogleModality),
      }),
      thinkingConfig: {
        includeThoughts: true,
      },
      ...(extra?.["config"] as object),
    },
  };

  return params;
}

function convertToGoogleContents(messages: Message[]): Content[] {
  return messages.map((message): Content => {
    switch (message.role) {
      case "user":
        return {
          role: "user",
          parts: message.content.map(convertToGoogleParts).flat(),
        };
      case "assistant":
        return {
          role: "model",
          parts: message.content.map(convertToGoogleParts).flat(),
        };
      case "tool":
        return {
          role: "user",
          parts: message.content.map(convertToGoogleParts).flat(),
        };
    }
  });
}

function convertToGoogleParts(part: Part): GooglePart[] {
  switch (part.type) {
    case "text":
      return [
        {
          text: part.text,
        },
      ];
    case "image":
      return [
        {
          inlineData: {
            data: part.image_data,
            mimeType: part.mime_type,
          },
        },
      ];
    case "audio":
      return [
        {
          inlineData: {
            data: part.audio_data,
            mimeType: mapAudioFormatToMimeType(part.format),
          },
        },
      ];
    case "reasoning":
      return [
        {
          text: part.text,
          thought: true,
          ...(part.signature && { thoughtSignature: part.signature }),
        },
      ];
    case "source":
      return part.content.map(convertToGoogleParts).flat();
    case "tool-call":
      return [
        {
          functionCall: {
            name: part.tool_name,
            args: part.args,
            id: part.tool_call_id,
          },
        },
      ];
    case "tool-result":
      return [
        {
          functionResponse: {
            id: part.tool_call_id,
            name: part.tool_name,
            response: convertToGoogleFunctionResponseResponse(
              part.content,
              Boolean(part.is_error),
            ),
          },
        },
      ];
  }
}

function convertToGoogleFunctionResponseResponse(
  parts: Part[],
  isError: boolean,
): Record<string, unknown> {
  const compatibleParts = getCompatiblePartsWithoutSourceParts(parts);
  const textParts: TextPart[] = [];
  for (const part of compatibleParts) {
    if (part.type === "text") {
      textParts.push(part);
    } else {
      throw new UnsupportedError(
        PROVIDER,
        `Google model tool result only supports text parts, got ${part.type}`,
      );
    }
  }

  const responses = textParts.map((part) => maybeParseJSON(part.text));
  if (responses.length === 0) {
    throw new InvalidInputError(
      "Google model tool result must have at least one text part",
    );
  }

  // Use "output" key to specify function output and "error" key to specify error details
  if (isError) {
    return { error: responses.length === 1 ? responses[0] : responses };
  } else {
    return { output: responses.length === 1 ? responses[0] : responses };
  }
}

function maybeParseJSON(text: string) {
  try {
    return JSON.parse(text) as Record<string, unknown>;
  } catch {
    return { data: text };
  }
}

function convertToGoogleTool(tool: Tool): FunctionDeclaration {
  return {
    name: tool.name,
    description: tool.description,
    parametersJsonSchema: tool.parameters,
  };
}

function convertToGoogleFunctionCallingConfig(
  toolChoice: ToolChoiceOption,
): FunctionCallingConfig {
  switch (toolChoice.type) {
    case "auto":
      return { mode: FunctionCallingConfigMode.AUTO };
    case "tool":
      return {
        mode: FunctionCallingConfigMode.ANY,
        allowedFunctionNames: [toolChoice.tool_name],
      };
    case "required":
      return { mode: FunctionCallingConfigMode.ANY };
    case "none":
      return { mode: FunctionCallingConfigMode.NONE };
  }
}

function convertToGoogleResponseSchema(
  responseFormat: ResponseFormatOption,
): [string, object | null] {
  if (responseFormat.type === "text") {
    return ["text/plain", null];
  }
  if (!responseFormat.schema) {
    return ["application/json", null];
  }
  return ["application/json", responseFormat.schema];
}

function convertToGoogleModality(modality: Modality): string {
  // https://ai.google.dev/api/generate-content#Modality
  switch (modality) {
    case "text":
      return "TEXT";
    case "image":
      return "IMAGE";
    case "audio":
      return "AUDIO";
  }
}

function mapGoogleContent(parts: GooglePart[]): Part[] {
  return parts
    .map((googlePart): Part | null => {
      if (googlePart.thought) {
        return {
          type: "reasoning",
          text: googlePart.text ?? "",
          ...(googlePart.thoughtSignature && {
            signature: googlePart.thoughtSignature,
          }),
        };
      }
      if (googlePart.text) {
        return {
          type: "text",
          text: googlePart.text,
        };
      }
      if (googlePart.inlineData) {
        if (googlePart.inlineData.mimeType?.startsWith("image/")) {
          if (!googlePart.inlineData.data) {
            throw new InvariantError(PROVIDER, "Image data is empty");
          }
          return {
            type: "image",
            image_data: googlePart.inlineData.data,
            mime_type: googlePart.inlineData.mimeType,
          };
        }
      }
      if (googlePart.inlineData?.mimeType?.startsWith("audio/")) {
        if (!googlePart.inlineData.data) {
          throw new InvariantError(PROVIDER, "Audio data is empty");
        }
        return {
          type: "audio",
          format: mapMimeTypeToAudioFormat(googlePart.inlineData.mimeType),
          audio_data: googlePart.inlineData.data,
        };
      }
      if (googlePart.functionCall) {
        if (!googlePart.functionCall.name) {
          throw new InvariantError(PROVIDER, "Function call name is missing");
        }
        return {
          type: "tool-call",
          tool_call_id: googlePart.functionCall.id ?? generateString(10),
          tool_name: googlePart.functionCall.name,
          args: googlePart.functionCall.args ?? {},
        };
      }
      return null;
    })
    .filter((part) => !!part);
}

function mapGoogleContentToDelta(
  content: Content,
  existingContentDeltas: ContentDelta[],
): ContentDelta[] {
  if (!content.parts) return [];
  const contentDeltas: ContentDelta[] = [];

  const parts = mapGoogleContent(content.parts);

  for (const part of parts) {
    const partDelta = looselyConvertPartToPartDelta(part);
    const index = guessDeltaIndex(partDelta, [
      ...existingContentDeltas,
      ...contentDeltas,
    ]);
    contentDeltas.push({
      index,
      part: partDelta,
    });
  }

  return contentDeltas;
}

function mapGoogleUsageMetadata(usageMetadata: UsageMetadata): ModelUsage {
  const usage: ModelUsage = {
    input_tokens: usageMetadata.promptTokenCount ?? 0,
    output_tokens: usageMetadata.responseTokenCount ?? 0,
  };
  if (usageMetadata.promptTokensDetails) {
    usage.input_tokens_details = sumModelTokensDetails(
      mapGoogleModalityTokenCountToUsageDetails(
        usageMetadata.promptTokensDetails,
      ),
    );
  }
  if (usageMetadata.responseTokensDetails) {
    usage.output_tokens_details = sumModelTokensDetails(
      mapGoogleModalityTokenCountToUsageDetails(
        usageMetadata.responseTokensDetails,
      ),
    );
  }

  return usage;
}

function mapGoogleModalityTokenCountToUsageDetails(
  modalityTokenCounts: ModalityTokenCount[],
): ModelTokensDetails[] {
  return modalityTokenCounts.map((modalityTokenCount): ModelTokensDetails => {
    if (modalityTokenCount.tokenCount === undefined) {
      return {};
    }
    switch (modalityTokenCount.modality) {
      case MediaModality.TEXT:
        return {
          text_tokens: modalityTokenCount.tokenCount,
        };
      case MediaModality.IMAGE:
        return {
          image_tokens: modalityTokenCount.tokenCount,
        };
      case MediaModality.AUDIO:
        return {
          audio_tokens: modalityTokenCount.tokenCount,
        };
      default:
        return {};
    }
  });
}
