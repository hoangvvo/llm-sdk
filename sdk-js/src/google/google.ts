import type {
  Content,
  FunctionDeclaration,
  FunctionDeclarationSchema,
  FunctionDeclarationsTool,
  GenerateContentRequest,
  GenerationConfig,
  FunctionCallPart as GoogleFunctionCallPart,
  FunctionResponsePart as GoogleFunctionResponsePart,
  InlineDataPart as GoogleInlineDataPart,
  Part as GooglePart,
  ResponseSchema as GoogleResponseSchema,
  TextPart as GoogleTextPart,
  ToolConfig,
  UsageMetadata,
} from "@google/generative-ai";
import {
  FunctionCallingMode,
  GoogleGenerativeAI,
  type GenerativeModel,
} from "@google/generative-ai";
import {
  mapAudioFormatToMimeType,
  mapMimeTypeToAudioFormat,
} from "../audio.utils.js";
import {
  InvariantError,
  NotImplementedError,
  UnsupportedError,
} from "../errors.js";
import type { LanguageModelMetadata } from "../language-model.js";
import { LanguageModel } from "../language-model.js";
import {
  guessDeltaIndex,
  looselyConvertPartToPartDelta,
} from "../stream.utils.js";
import type {
  AudioPart,
  ContentDelta,
  ImagePart,
  LanguageModelInput,
  Message,
  ModelResponse,
  ModelUsage,
  Part,
  PartialModelResponse,
  ResponseFormatOption,
  TextPart,
  Tool,
  ToolCallPart,
  ToolChoiceOption,
  ToolResultPart,
} from "../types.js";
import { calculateCost } from "../usage.utils.js";

export interface GoogleModelOptions {
  apiKey: string;
  modelId: string;
}

export class GoogleModel extends LanguageModel {
  provider: string;
  modelId: string;
  metadata?: LanguageModelMetadata;

  private genModel: GenerativeModel;

  constructor(
    public options: GoogleModelOptions,
    metadata?: LanguageModelMetadata,
  ) {
    super();
    this.provider = "google";
    this.modelId = options.modelId;
    if (metadata) this.metadata = metadata;

    const genAI = new GoogleGenerativeAI(options.apiKey);
    this.genModel = genAI.getGenerativeModel({ model: options.modelId });
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const request = convertToGenerateContentRequest(input);
    const { response } = await this.genModel.generateContent(request);

    const candidate = response.candidates?.[0];
    if (!candidate) {
      throw new InvariantError("No candidate in response");
    }

    const content = mapGoogleContent(candidate.content);
    const result: ModelResponse = { content };
    if (response.usageMetadata) {
      result.usage = mapGoogleUsageMetadata(response.usageMetadata, input);
      if (this.metadata?.pricing) {
        result.cost = calculateCost(result.usage, this.metadata.pricing);
      }
    }

    return result;
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse> {
    const request = convertToGenerateContentRequest(input);
    const { stream } = await this.genModel.generateContentStream(request);

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
          yield { delta };
        }
      }

      if (chunk.usageMetadata) {
        const usage = mapGoogleUsageMetadata(chunk.usageMetadata, input);
        yield { usage };
      }
    }
  }
}

function convertToGenerateContentRequest(
  input: LanguageModelInput,
): GenerateContentRequest {
  const {
    messages,
    system_prompt,
    max_tokens,
    temperature,
    top_p,
    top_k,
    presence_penalty,
    frequency_penalty,
    tools,
    tool_choice,
    response_format,
    extra,
  } = input;

  return {
    contents: messages.map(convertToGoogleContent),
    ...(system_prompt && { systemInstruction: system_prompt }),
    ...(tools && {
      tools: convertToGoogleFunctionDeclarationTools(tools),
    }),
    ...(tool_choice && {
      toolConfig: convertToGoogleToolConfig(tool_choice),
    }),
    generationConfig: {
      ...(typeof max_tokens === "number" && {
        maxOutputTokens: max_tokens,
      }),
      ...(typeof temperature === "number" && {
        temperature,
      }),
      ...(typeof top_p === "number" && { topP: top_p }),
      ...(typeof top_k === "number" && { topK: top_k }),
      ...(typeof presence_penalty === "number" && {
        presencePenalty: presence_penalty,
      }),
      ...(typeof frequency_penalty === "number" && {
        frequencyPenalty: frequency_penalty,
      }),
      ...(response_format &&
        convertResponseFormatToGoogleGenerationConfig(response_format)),
      ...(extra?.["generationConfig"] as GenerationConfig),
    },
    ...extra,
  };
}

// MARK: To Provider Messages

function convertToGoogleContent(message: Message): Content {
  const parts = message.content.map(convertToGooglePart);
  switch (message.role) {
    case "user": {
      return {
        role: "user",
        parts,
      };
    }
    case "assistant": {
      return {
        role: "model",
        parts,
      };
    }
    case "tool": {
      return {
        role: "function",
        parts,
      };
    }
  }
}

function convertToGooglePart(part: Part): GooglePart {
  switch (part.type) {
    case "text":
      return convertToGoogleTextPart(part);
    case "image":
    case "audio":
      return convertToGoogleInlineData(part);
    case "tool-call":
      return convertToGoogleFunctionCallPart(part);
    case "tool-result":
      return convertToGoogleFunctionResponsePart(part);
  }
}

function convertToGoogleTextPart(part: TextPart): GoogleTextPart {
  return { text: part.text };
}

function convertToGoogleInlineData(
  part: ImagePart | AudioPart,
): GoogleInlineDataPart {
  switch (part.type) {
    case "image":
      return {
        inlineData: {
          data: part.image_data,
          mimeType: part.mime_type,
        },
      };
    case "audio": {
      return {
        inlineData: {
          data: part.audio_data,
          mimeType: mapAudioFormatToMimeType(part.format),
        },
      };
    }
  }
}

function convertToGoogleFunctionCallPart(
  part: ToolCallPart,
): GoogleFunctionCallPart {
  return {
    functionCall: {
      name: part.tool_name,
      args: part.args,
    },
  };
}

function convertToGoogleFunctionResponsePart(
  part: ToolResultPart,
): GoogleFunctionResponsePart {
  const textParts = part.content.filter((part) => part.type === "text");

  let response: object;
  const firstTextPart = textParts[0];
  if (!firstTextPart) {
    throw new UnsupportedError(
      "Cannot convert tool result to Google function response without a text part",
    );
  }
  if (textParts.length === 1) {
    response = {
      results: textParts.map((textPart) =>
        tryConvertToGoogleFunctionResponseResponse(textPart.text),
      ),
    };
  } else {
    response = tryConvertToGoogleFunctionResponseResponse(firstTextPart.text);
  }

  return {
    functionResponse: {
      name: part.tool_name,
      response,
    },
  };
}

function tryConvertToGoogleFunctionResponseResponse(
  text: string,
): Record<string, unknown> {
  try {
    const obj = JSON.parse(text) as Record<string, unknown>;
    // Google does not support array in response
    if (Array.isArray(obj)) {
      return {
        result: obj,
      };
    }
    return obj;
  } catch {
    return { result: text };
  }
}

// MARK: To Provider Tools

function convertToGoogleFunctionDeclarationTools(
  tools: Tool[],
): FunctionDeclarationsTool[] {
  return [
    {
      functionDeclarations: tools.map((tool): FunctionDeclaration => {
        const declaration: FunctionDeclaration = {
          name: tool.name,
          description: tool.description,
          parameters: tool.parameters as unknown as FunctionDeclarationSchema,
        };

        return declaration;
      }),
    },
  ];
}

function convertToGoogleToolConfig(toolChoice: ToolChoiceOption): ToolConfig {
  switch (toolChoice.type) {
    case "auto":
      return {
        functionCallingConfig: {
          mode: FunctionCallingMode.AUTO,
        },
      };
    case "required":
      return {
        functionCallingConfig: {
          mode: FunctionCallingMode.ANY,
        },
      };
    case "none":
      return {
        functionCallingConfig: {
          mode: FunctionCallingMode.NONE,
        },
      };
    case "tool":
      return {
        functionCallingConfig: {
          mode: FunctionCallingMode.ANY,
          allowedFunctionNames: [toolChoice.tool_name],
        },
      };
  }
}

// MARK: To Provider Response Format

function convertResponseFormatToGoogleGenerationConfig(
  responseFormat: ResponseFormatOption,
): GenerationConfig {
  switch (responseFormat.type) {
    case "json": {
      return {
        responseMimeType: "application/json",
        ...(responseFormat.schema && {
          responseSchema:
            responseFormat.schema as unknown as GoogleResponseSchema,
        }),
      };
    }
    case "text": {
      return {};
    }
  }
}

// MARK: To SDK Message

function mapGoogleContent(content: Content): Part[] {
  return content.parts.map(mapGooglePart);
}

function mapGooglePart(part: GooglePart): Part {
  if (part.text) return mapGoogleTextPart(part);
  if (part.inlineData) return mapGoogleInlineData(part);
  if (part.functionCall) return mapGoogleFunctionCall(part);
  throw new NotImplementedError(
    `Cannot map Google part to SDK part for type ${Object.keys(part)
      .filter((key) => !!part[key as keyof GooglePart])
      .join(", ")}`,
  );
}

function mapGoogleTextPart(part: GoogleTextPart): TextPart {
  return {
    type: "text",
    text: part.text,
  };
}

function mapGoogleInlineData(
  part: GoogleInlineDataPart,
): ImagePart | AudioPart {
  if (part.inlineData.mimeType.startsWith("image/")) {
    return {
      type: "image",
      image_data: part.inlineData.data,
      mime_type: part.inlineData.mimeType,
    };
  }
  if (part.inlineData.mimeType.startsWith("audio/")) {
    return {
      type: "audio",
      format: mapMimeTypeToAudioFormat(part.inlineData.mimeType),
      audio_data: part.inlineData.data,
    };
  }
  throw new NotImplementedError(
    `Cannot map Google inline data part to SDK part for mime type: ${part.inlineData.mimeType}`,
  );
}

function mapGoogleFunctionCall(part: GoogleFunctionCallPart): ToolCallPart {
  return {
    type: "tool-call",
    tool_call_id: genidForToolCall(),
    tool_name: part.functionCall.name,
    args: part.functionCall.args as Record<string, unknown>,
  };
}

/**
 * Google function calls do not have ids so we need to generate ones
 */
function genidForToolCall() {
  return Math.random().toString(36).substring(2, 15);
}

// MARK: To SDK Delta

function mapGoogleUsageMetadata(
  usage: UsageMetadata,
  input: LanguageModelInput,
): ModelUsage {
  const cachedContentTokenCount = usage.cachedContentTokenCount;
  const hasAudioPart = input.messages.some(
    (s) => s.role === "user" && s.content.some((p) => p.type === "audio"),
  );
  const result: ModelUsage = {
    input_tokens: usage.promptTokenCount,
    output_tokens: usage.candidatesTokenCount,
  };
  if (typeof cachedContentTokenCount === "number") {
    result.input_tokens_details = {
      [hasAudioPart ? "cachedAudioTokens" : "cachedTextTokens"]:
        cachedContentTokenCount,
    };
  }
  return result;
}

// MARK: To SDK Usage

function mapGoogleContentToDelta(
  content: Content,
  existingContentDeltas: ContentDelta[],
): ContentDelta[] {
  const contentDeltas: ContentDelta[] = [];

  content.parts.forEach((googlePart) => {
    const part = looselyConvertPartToPartDelta(mapGooglePart(googlePart));
    const index = guessDeltaIndex(part, [
      ...existingContentDeltas,
      ...contentDeltas,
    ]);
    contentDeltas.push({
      index,
      part,
    });
  });

  return contentDeltas;
}
