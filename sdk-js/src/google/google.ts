import type {
  FunctionCallingConfig,
  FunctionDeclaration,
  FunctionResponse,
  FunctionResponsePart,
  GenerateContentConfig,
  GroundingChunk,
  GroundingMetadata,
  GroundingSupport,
  Part as GooglePart,
  ModalityTokenCount,
  PrebuiltVoiceConfig,
  SpeechConfig,
  ThinkingConfig,
  Tool as GoogleTool,
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
} from "../audio-part.utils.ts";
import {
  InvalidInputError,
  InvariantError,
  UnsupportedError,
} from "../errors.ts";
import { generateString } from "../id.utils.ts";
import type {
  LanguageModel,
  LanguageModelCallOptions,
  LanguageModelMetadata,
} from "../language-model.ts";
import { traceLanguageModel } from "../opentelemetry.ts";
import { getCompatiblePartsWithoutSourceParts } from "../source-part.utils.ts";
import {
  guessDeltaIndex,
  looselyConvertPartToPartDelta,
} from "../stream.utils.ts";
import { CANCELLED_TOOL_RESULT_FALLBACK_CONTENT } from "../tool-result.utils.ts";
import type {
  AudioOptions,
  Citation,
  CitationDelta,
  ContentDelta,
  LanguageModelInput,
  Message,
  Modality,
  ModelResponse,
  ModelTokensDetails,
  ModelUsage,
  Part,
  PartialModelResponse,
  ReasoningOptions,
  ReasoningPart,
  ResponseFormatOption,
  TextPart,
  Tool,
  ToolChoiceOption,
  ToolResultStatus,
} from "../types.ts";
import { calculateCost, sumModelTokensDetails } from "../usage.utils.ts";

const PROVIDER = "google";

export interface GoogleModelOptions {
  apiKey: string;
  modelId: string;
  baseURL?: string;
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
      ...(options.baseURL ? { httpOptions: { baseUrl: options.baseURL } } : {}),
    });

    traceLanguageModel(this);
  }

  async generate(
    input: LanguageModelInput,
    options?: LanguageModelCallOptions,
  ): Promise<ModelResponse> {
    const params = convertToGenerateContentParameters(input, this.modelId);
    if (options?.signal) {
      params.config = { ...params.config, abortSignal: options.signal };
    }

    const response = await this.#ai.models.generateContent(params);

    const candidate = response.candidates?.[0];
    if (!candidate) {
      throw new InvariantError(PROVIDER, "No candidate in response");
    }

    const content = mapGoogleContent(
      candidate.content?.parts ?? [],
      candidate.groundingMetadata,
    );
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
    options?: LanguageModelCallOptions,
  ): AsyncGenerator<PartialModelResponse> {
    const params = convertToGenerateContentParameters(input, this.modelId);
    if (options?.signal) {
      params.config = { ...params.config, abortSignal: options.signal };
    }

    const stream = await this.#ai.models.generateContentStream(params);

    const allContentDeltas: ContentDelta[] = [];
    // Streaming support indices refer to grounding chunks accumulated across
    // all chunks, rather than only the current response chunk.
    const groundingChunks: GroundingChunk[] = [];
    const webSearchQueries = new Set<string>();
    const streamTextPartMappings = new Map<number, number>();

    for await (const chunk of stream) {
      const candidate = chunk.candidates?.[0];
      const incomingContentDeltas: ContentDelta[] = [];

      if (candidate?.content) {
        incomingContentDeltas.push(
          ...mapGoogleContentToDelta(
            candidate.content,
            allContentDeltas,
            streamTextPartMappings,
          ),
        );
      }

      const metadata = candidate?.groundingMetadata;
      if (metadata) {
        for (const query of metadata.webSearchQueries ?? []) {
          webSearchQueries.add(query);
        }
        if (metadata.groundingChunks) {
          groundingChunks.push(...metadata.groundingChunks);
        }
        for (const support of metadata.groundingSupports ?? []) {
          const sdkPartIndex = streamTextPartMappings.get(
            support.segment?.partIndex ?? 0,
          );
          if (sdkPartIndex === undefined) continue;
          for (const citation of mapGoogleGroundingCitations(
            support,
            groundingChunks,
          )) {
            const citationDelta: CitationDelta = {
              type: "citation",
              ...citation,
            };
            incomingContentDeltas.push({
              index: sdkPartIndex,
              part: { type: "text", text: "", citation: citationDelta },
            });
          }
        }
      }

      if (incomingContentDeltas.length > 0) {
        allContentDeltas.push(...incomingContentDeltas);

        for (const delta of incomingContentDeltas) {
          const event: PartialModelResponse = { delta };
          yield event;
        }
      }

      if (chunk.usageMetadata) {
        const usage = mapGoogleUsageMetadata(chunk.usageMetadata);
        const partial: PartialModelResponse = { usage };
        if (this.metadata?.pricing) {
          partial.cost = calculateCost(usage, this.metadata.pricing);
        }
        yield partial;
      }
    }

    const sources = groundingChunks.flatMap((chunk) =>
      chunk.web?.uri
        ? [
            {
              url: chunk.web.uri,
              ...(chunk.web.title ? { title: chunk.web.title } : {}),
            },
          ]
        : [],
    );
    if (webSearchQueries.size > 0 || sources.length > 0) {
      const callIndex =
        Math.max(...allContentDeltas.map(({ index }) => index), -1) + 1;
      const toolCallId = generateString(10);
      yield {
        delta: {
          index: callIndex,
          part: {
            type: "tool-call",
            tool_call_id: toolCallId,
            call: {
              type: "web_search",
              status: "completed",
              ...(webSearchQueries.size > 0
                ? {
                    action: {
                      type: "search" as const,
                      queries: [...webSearchQueries],
                    },
                  }
                : {}),
            },
          },
        },
      };
      yield {
        delta: {
          index: callIndex + 1,
          part: {
            type: "tool-result",
            tool_call_id: toolCallId,
            result: { type: "web_search", sources },
            status: "completed",
          },
        },
      };
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
    audio,
    reasoning,
  } = input;

  const params: GenerateContentParameters = {
    contents: convertToGoogleContents(messages),
    model: modelId,
  };
  const config: GenerateContentConfig = {};
  if (system_prompt) {
    config.systemInstruction = system_prompt;
  }
  if (temperature) {
    config.temperature = temperature;
  }
  if (typeof top_p === "number") {
    config.topP = top_p;
  }
  if (typeof top_k === "number") {
    config.topK = top_k;
  }
  if (presence_penalty) {
    config.presencePenalty = presence_penalty;
  }
  if (frequency_penalty) {
    config.frequencyPenalty = frequency_penalty;
  }
  if (seed) {
    config.seed = seed;
  }
  if (max_tokens) {
    config.maxOutputTokens = max_tokens;
  }
  if (tools) {
    config.tools = convertToGoogleTools(tools);
  }
  if (tool_choice) {
    config.toolConfig = {
      functionCallingConfig: convertToGoogleFunctionCallingConfig(tool_choice),
    };
  }
  if (response_format) {
    const [responseMimeType, responseJsonSchema] =
      convertToGoogleResponseSchema(response_format);
    config.responseMimeType = responseMimeType;
    config.responseJsonSchema = responseJsonSchema;
  }
  if (modalities) {
    config.responseModalities = modalities.map(convertToGoogleModality);
  }
  if (audio) {
    config.speechConfig = convertToGoogleSpeechConfig(audio);
  }
  if (reasoning) {
    config.thinkingConfig = convertToGoogleThinkingConfig(reasoning);
  }

  return {
    ...params,
    config,
  };
}

function convertToGoogleContents(messages: Message[]): Content[] {
  return messages.flatMap((message): Content[] => {
    const parts = message.content.map(convertToGoogleParts).flat();
    // Google hosted-tool metadata has no request part to replay.
    if (parts.length === 0) return [];

    switch (message.role) {
      case "user":
        return [
          {
            role: "user",
            parts,
          },
        ];
      case "assistant":
        return [
          {
            role: "model",
            parts,
          },
        ];
      case "tool":
        return [
          {
            role: "user",
            parts,
          },
        ];
    }
  });
}

function convertToGoogleParts(part: Part): GooglePart[] {
  switch (part.type) {
    case "text":
      return [
        part.signature
          ? { text: part.text, thoughtSignature: part.signature }
          : { text: part.text },
      ];
    case "image":
      return [
        {
          inlineData: {
            data: part.data,
            mimeType: part.mime_type,
          },
        },
      ];
    case "audio":
      return [
        {
          inlineData: {
            data: part.data,
            mimeType: mapAudioFormatToMimeType(part.format),
          },
        },
      ];
    case "reasoning": {
      const googleReasoningPart: GooglePart = {
        text: part.text,
        thought: true,
      };
      if (part.signature) {
        googleReasoningPart.thoughtSignature = part.signature;
      }
      return [googleReasoningPart];
    }
    case "source":
      return part.content.map(convertToGoogleParts).flat();
    case "tool-call": {
      if (part.call.type === "web_search") return [];
      const googleToolCallPart: GooglePart = {
        functionCall: {
          name: part.call.name,
          args: part.call.args,
          id: part.tool_call_id,
        },
      };
      if (part.signature) {
        googleToolCallPart.thoughtSignature = part.signature;
      }
      return [googleToolCallPart];
    }
    case "tool-result": {
      if (part.result.type === "web_search") return [];
      const functionResponse = convertToGoogleFunctionResponse(
        part.result.content,
        part.status,
      );
      const googleFunctionResponse: FunctionResponse = {
        id: part.tool_call_id,
        name: part.result.name,
        response: functionResponse.response,
      };
      if (functionResponse.parts) {
        googleFunctionResponse.parts = functionResponse.parts;
      }
      return [
        {
          functionResponse: googleFunctionResponse,
        },
      ];
    }
  }
}

function convertToGoogleFunctionResponse(
  parts: Part[],
  status: ToolResultStatus,
): {
  response: Record<string, unknown>;
  parts?: FunctionResponsePart[];
} {
  const compatibleParts = getCompatiblePartsWithoutSourceParts(parts);
  const textParts: TextPart[] = [];
  const functionResponseParts: FunctionResponsePart[] = [];
  for (const part of compatibleParts) {
    switch (part.type) {
      case "text":
        textParts.push(part);
        break;
      case "image":
      case "audio":
        functionResponseParts.push({
          inlineData: {
            data: part.data,
            mimeType:
              part.type === "image"
                ? part.mime_type
                : mapAudioFormatToMimeType(part.format),
          },
        });
        break;
      default:
        throw new InvalidInputError(
          `Google model tool result does not support part type ${part.type}`,
        );
    }
  }

  const responses = textParts.map((part) => maybeParseJSON(part.text));
  // Use "output" key to specify function output and "error" key to specify error details,
  // as per Google API specification
  const key = status === "completed" ? "output" : "error";
  const functionResponse: {
    response: Record<string, unknown>;
    parts?: FunctionResponsePart[];
  } = {
    response: {
      [key]:
        responses.length === 0
          ? status === "cancelled"
            ? CANCELLED_TOOL_RESULT_FALLBACK_CONTENT
            : {}
          : responses.length === 1
            ? responses[0]
            : responses,
    },
  };
  if (functionResponseParts.length > 0) {
    functionResponse.parts = functionResponseParts;
  }
  return functionResponse;
}

function maybeParseJSON(text: string) {
  try {
    return JSON.parse(text) as Record<string, unknown>;
  } catch {
    return { data: text };
  }
}

function convertToGoogleTools(tools: Tool[]): GoogleTool[] {
  const functionDeclarations: FunctionDeclaration[] = [];
  const googleTools: GoogleTool[] = [];

  for (const tool of tools) {
    if (tool.type === "function") {
      functionDeclarations.push({
        name: tool.name,
        description: tool.description,
        parametersJsonSchema: tool.parameters,
      });
      continue;
    }

    if (
      (tool.allowed_domains && tool.allowed_domains.length > 0) ||
      tool.user_location
    ) {
      // GoogleSearch has no equivalent request fields. Rejecting these avoids
      // silently broadening a domain-restricted or localized search.
      throw new UnsupportedError(
        PROVIDER,
        "Google Search does not support allowed_domains or user_location",
      );
    }
    googleTools.push({ googleSearch: {} });
  }

  if (functionDeclarations.length > 0) {
    googleTools.unshift({ functionDeclarations });
  }

  return googleTools;
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

function convertToGoogleSpeechConfig(audio: AudioOptions): SpeechConfig {
  const prebuiltVoiceConfig: PrebuiltVoiceConfig = {};
  if (audio.voice) {
    prebuiltVoiceConfig.voiceName = audio.voice;
  }
  const speechConfig: SpeechConfig = {
    voiceConfig: { prebuiltVoiceConfig },
  };
  if (audio.language) {
    speechConfig.languageCode = audio.language;
  }
  return speechConfig;
}

function convertToGoogleThinkingConfig(
  reasoning: ReasoningOptions,
): ThinkingConfig {
  const thinkingConfig: ThinkingConfig = {
    includeThoughts: reasoning.enabled,
  };
  if (typeof reasoning.budget_tokens === "number") {
    thinkingConfig.thinkingBudget = reasoning.budget_tokens;
  }
  return thinkingConfig;
}

function mapGoogleContent(
  parts: GooglePart[],
  groundingMetadata?: GroundingMetadata,
): Part[] {
  const mappedParts = parts.map(mapGooglePart);

  for (const support of groundingMetadata?.groundingSupports ?? []) {
    const part = mappedParts[support.segment?.partIndex ?? 0];
    if (part?.type !== "text") continue;
    for (const citation of mapGoogleGroundingCitations(
      support,
      groundingMetadata?.groundingChunks ?? [],
    )) {
      part.citations = part.citations ?? [];
      part.citations.push(citation);
    }
  }

  const content = mappedParts.filter((part) => part !== null);
  const queries = groundingMetadata?.webSearchQueries ?? [];
  const sources = (groundingMetadata?.groundingChunks ?? []).flatMap((chunk) =>
    chunk.web?.uri
      ? [
          {
            url: chunk.web.uri,
            ...(chunk.web.title ? { title: chunk.web.title } : {}),
          },
        ]
      : [],
  );
  if (queries.length > 0 || sources.length > 0) {
    const toolCallId = generateString(10);
    content.push({
      type: "tool-call",
      tool_call_id: toolCallId,
      call: {
        type: "web_search",
        status: "completed",
        ...(queries.length > 0
          ? { action: { type: "search" as const, queries } }
          : {}),
      },
    });
    content.push({
      type: "tool-result",
      tool_call_id: toolCallId,
      result: { type: "web_search", sources },
      status: "completed",
    });
  }
  return content;
}

function mapGooglePart(googlePart: GooglePart): Part | null {
  if (googlePart.thought) {
    const reasoningPart: ReasoningPart = {
      type: "reasoning",
      text: googlePart.text ?? "",
    };
    if (googlePart.thoughtSignature) {
      reasoningPart.signature = googlePart.thoughtSignature;
    }
    return reasoningPart;
  }
  if (googlePart.text) {
    const textPart: TextPart = {
      type: "text",
      text: googlePart.text,
    };
    if (googlePart.thoughtSignature) {
      textPart.signature = googlePart.thoughtSignature;
    }
    return textPart;
  }
  if (googlePart.inlineData?.mimeType?.startsWith("image/")) {
    if (!googlePart.inlineData.data) {
      throw new InvariantError(PROVIDER, "Image data is empty");
    }
    return {
      type: "image",
      data: googlePart.inlineData.data,
      mime_type: googlePart.inlineData.mimeType,
    };
  }
  if (googlePart.inlineData?.mimeType?.startsWith("audio/")) {
    if (!googlePart.inlineData.data) {
      throw new InvariantError(PROVIDER, "Audio data is empty");
    }
    return {
      type: "audio",
      format: mapMimeTypeToAudioFormat(googlePart.inlineData.mimeType),
      data: googlePart.inlineData.data,
    };
  }
  if (googlePart.functionCall) {
    if (!googlePart.functionCall.name) {
      throw new InvariantError(PROVIDER, "Function call name is missing");
    }
    const toolCallPart: Extract<Part, { type: "tool-call" }> = {
      type: "tool-call",
      tool_call_id: googlePart.functionCall.id ?? generateString(10),
      call: {
        type: "function",
        name: googlePart.functionCall.name,
        args: googlePart.functionCall.args ?? {},
      },
    };
    if (googlePart.thoughtSignature) {
      toolCallPart.signature = googlePart.thoughtSignature;
    }
    return toolCallPart;
  }
  return null;
}

function mapGoogleGroundingCitations(
  support: GroundingSupport,
  chunks: GroundingChunk[],
): Citation[] {
  const citations: Citation[] = [];
  for (const chunkIndex of support.groundingChunkIndices ?? []) {
    const web = chunks[chunkIndex]?.web;
    if (!web?.uri) continue;
    const citation: Citation = { source: web.uri };
    if (web.title) citation.title = web.title;
    if (support.segment?.text) citation.cited_text = support.segment.text;
    // Google reports byte offsets within the referenced content part. Keep
    // those provider offsets intact in the common citation shape.
    if (support.segment?.startIndex !== undefined) {
      citation.start_index = support.segment.startIndex;
    }
    if (support.segment?.endIndex !== undefined) {
      citation.end_index = support.segment.endIndex;
    }
    citations.push(citation);
  }
  return citations;
}

function mapGoogleContentToDelta(
  content: Content,
  existingContentDeltas: ContentDelta[],
  streamTextPartMappings: Map<number, number>,
): ContentDelta[] {
  if (!content.parts) return [];
  const contentDeltas: ContentDelta[] = [];

  for (const [providerPartIndex, googlePart] of content.parts.entries()) {
    const part = mapGooglePart(googlePart);
    if (!part) continue;
    const partDelta = looselyConvertPartToPartDelta(part);
    let index: number;
    if (partDelta.type === "text") {
      // Google's citation partIndex addresses the provider's parts array. Keep
      // a text-only mapping because provider slots are not stable for separate
      // tool calls, which must retain the existing index-matching behavior.
      const mappedIndex = streamTextPartMappings.get(providerPartIndex);
      if (mappedIndex !== undefined) {
        index = mappedIndex;
      } else if (contentDeltas.some((delta) => delta.part.type === "text")) {
        // Multiple text parts in one chunk are distinct provider parts.
        index = nextGoogleDeltaIndex(existingContentDeltas, contentDeltas);
      } else {
        // Part indexes are local to an incremental chunk. Reuse the existing
        // text stream when a later chunk starts again at provider index zero.
        index = guessDeltaIndex(partDelta, [
          ...existingContentDeltas,
          ...contentDeltas,
        ]);
      }
      streamTextPartMappings.set(providerPartIndex, index);
    } else {
      index = guessDeltaIndex(partDelta, [
        ...existingContentDeltas,
        ...contentDeltas,
      ]);
    }
    contentDeltas.push({
      index,
      part: partDelta,
    });
  }

  return contentDeltas;
}

function nextGoogleDeltaIndex(
  existingContentDeltas: ContentDelta[],
  incomingContentDeltas: ContentDelta[],
): number {
  return (
    Math.max(
      ...existingContentDeltas.map((delta) => delta.index),
      ...incomingContentDeltas.map((delta) => delta.index),
      -1,
    ) + 1
  );
}

function mapGoogleUsageMetadata(usageMetadata: UsageMetadata): ModelUsage {
  const usage: ModelUsage = {
    input_tokens: usageMetadata.promptTokenCount ?? 0,
    output_tokens: usageMetadata.responseTokenCount ?? 0,
  };
  if (usageMetadata.promptTokensDetails) {
    usage.input_tokens_details = mapGoogleModalityTokenCountToUsageDetails(
      usageMetadata.promptTokensDetails,
      usageMetadata.cacheTokensDetails,
    );
  }
  if (usageMetadata.responseTokensDetails) {
    usage.output_tokens_details = mapGoogleModalityTokenCountToUsageDetails(
      usageMetadata.responseTokensDetails,
    );
  }

  return usage;
}

function mapGoogleModalityTokenCountToUsageDetails(
  modalityTokenCounts: ModalityTokenCount[],
  cachedTokensDetails?: ModalityTokenCount[],
): ModelTokensDetails {
  let tokensDetails: ModelTokensDetails = {};
  for (const detail of modalityTokenCounts) {
    if (detail.tokenCount === undefined) {
      continue;
    }
    switch (detail.modality) {
      case MediaModality.TEXT:
        tokensDetails = sumModelTokensDetails([
          tokensDetails,
          { text_tokens: detail.tokenCount },
        ]);
        break;
      case MediaModality.IMAGE:
        tokensDetails = sumModelTokensDetails([
          tokensDetails,
          { image_tokens: detail.tokenCount },
        ]);
        break;
      case MediaModality.AUDIO:
        tokensDetails = sumModelTokensDetails([
          tokensDetails,
          { audio_tokens: detail.tokenCount },
        ]);
        break;
      default:
        break;
    }
  }
  for (const detail of cachedTokensDetails ?? []) {
    if (detail.tokenCount === undefined) {
      continue;
    }
    switch (detail.modality) {
      case MediaModality.TEXT:
        tokensDetails = sumModelTokensDetails([
          tokensDetails,
          { cached_text_tokens: detail.tokenCount },
        ]);
        break;
      case MediaModality.IMAGE:
        tokensDetails = sumModelTokensDetails([
          tokensDetails,
          { cached_image_tokens: detail.tokenCount },
        ]);
        break;
      case MediaModality.AUDIO:
        tokensDetails = sumModelTokensDetails([
          tokensDetails,
          { cached_audio_tokens: detail.tokenCount },
        ]);
        break;
      default:
        break;
    }
  }
  return tokensDetails;
}
