import type {
  Content,
  FunctionDeclaration,
  FunctionDeclarationSchema,
  FunctionDeclarationsTool,
  GenerateContentCandidate,
  GenerateContentRequest,
  GenerationConfig,
  GenerativeModel,
  Part as GooglePart,
  Schema,
  SchemaType,
  ToolConfig,
  UsageMetadata,
} from "@google/generative-ai";
import { FunctionCallingMode, GoogleGenerativeAI } from "@google/generative-ai";
import { InvalidValueError, NotImplementedError } from "../errors/errors.js";
import type { LanguageModelMetadata } from "../models/language-model.js";
import { LanguageModel } from "../models/language-model.js";
import type {
  AssistantMessage,
  ContentDelta,
  LanguageModelInput,
  Message,
  ModelResponse,
  ModelUsage,
  Part,
  PartialModelResponse,
  Tool,
  ToolResultPart,
} from "../types.js";
import { mapAudioFormatToMimeType } from "../utils/audio.utils.js";
import { convertAudioPartsToTextParts } from "../utils/message.utils.js";
import type { InternalContentDelta } from "../utils/stream.utils.js";
import {
  ContentDeltaAccumulator,
  guessDeltaIndex,
} from "../utils/stream.utils.js";
import { calculateCost } from "../utils/usage.utils.js";
import type { GoogleModelOptions } from "./types.js";

export type GoogleLanguageModelInput = LanguageModelInput & {
  extra?: Partial<GenerateContentRequest>;
};

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
    this.genModel = genAI.getGenerativeModel({
      model: options.modelId,
    });
  }

  async generate(input: GoogleLanguageModelInput): Promise<ModelResponse> {
    const result = await this.genModel.generateContent(
      convertToGoogleParams(input, this.options),
    );

    const candidate = result.response.candidates?.[0];
    if (!candidate) {
      throw new Error("no candidates in response");
    }

    const usage: ModelUsage | undefined = result.response.usageMetadata
      ? mapGoogleUsage(result.response.usageMetadata, input)
      : undefined;

    const response: ModelResponse = {
      content: mapGoogleMessage(candidate).content,
    };
    if (usage) {
      response.usage = usage;
      if (this.metadata?.pricing) {
        response.cost = calculateCost(usage, this.metadata.pricing);
      }
    }
    return response;
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse, ModelResponse> {
    const { stream } = await this.genModel.generateContentStream(
      convertToGoogleParams(input, this.options),
    );

    let usage: ModelUsage | undefined;

    const accumulator = new ContentDeltaAccumulator();

    for await (const chunk of stream) {
      const candidate = chunk.candidates?.[0];

      if (candidate?.content) {
        const incomingContentDeltas = mapGoogleDelta(
          candidate.content,
          accumulator.deltas,
        );

        accumulator.addChunks(incomingContentDeltas);

        for (const delta of incomingContentDeltas) {
          yield { delta };
        }
      }

      if (chunk.usageMetadata) {
        usage = mapGoogleUsage(chunk.usageMetadata, input);
      }
    }

    const response: ModelResponse = {
      content: accumulator.computeContent(),
    };
    if (usage) {
      response.usage = usage;
    }
    return response;
  }
}

export function convertToGoogleParams(
  input: GoogleLanguageModelInput,
  options: GoogleModelOptions,
): GenerateContentRequest {
  const toolConfig = convertToGoogleToolConfig(input.tool_choice);
  const samplingParams = convertToGoogleSamplingParams(input);

  const params: GenerateContentRequest = {
    contents: convertToGoogleMessages(input.messages, options),
    generationConfig: {
      ...samplingParams,
      ...input.extra?.["generationConfig"],
    },
    ...input.extra,
  };

  if (input.system_prompt) {
    params.systemInstruction = input.system_prompt;
  }
  if (input.tools) {
    params.tools = convertToGoogleTools(input.tools);
  }
  if (toolConfig) {
    params.toolConfig = toolConfig;
  }
  const responseFormatConfig = convertToGoogleResponseFormat(
    input.response_format,
  );
  if (responseFormatConfig) {
    params.generationConfig = {
      ...params.generationConfig,
      ...responseFormatConfig,
    };
  }
  return params;
}

export function convertToGoogleMessages(
  messages: Message[],
  options: GoogleModelOptions,
): Content[] {
  if (options.convertAudioPartsToTextParts) {
    messages = messages.map(convertAudioPartsToTextParts);
  }
  return messages.map((message): Content => {
    const parts = message.content.map((part): GooglePart => {
      switch (part.type) {
        case "text":
          return {
            text: part.text,
          };
        case "image":
          return {
            inlineData: {
              data: part.image_data,
              mimeType: part.mime_type,
            },
          };
        case "audio":
          return {
            inlineData: {
              data: part.audio_data,
              mimeType: mapAudioFormatToMimeType(part),
            },
          };
        case "tool-call":
          return {
            functionCall: {
              name: part.tool_name,
              args: part.args || {},
            },
          };
        case "tool-result": {
          let responseObj = part.result as object;
          if (Array.isArray(responseObj)) {
            responseObj = { result: responseObj };
          }
          return {
            functionResponse: {
              name: part.tool_name,
              response: responseObj,
            },
          };
        }
        default: {
          const exhaustiveCheck: never = part;
          throw new InvalidValueError("part.type", exhaustiveCheck);
        }
      }
    });

    switch (message.role) {
      case "assistant":
        return {
          role: "model",
          parts,
        };
      case "tool":
        return {
          role: "function",
          parts,
        };
      case "user":
        return {
          role: "user",
          parts,
        };
      default: {
        const exhaustiveCheck: never = message;
        throw new InvalidValueError("message.role", exhaustiveCheck);
      }
    }
  });
}

export function convertToGoogleSamplingParams(
  input: Partial<LanguageModelInput>,
): GenerationConfig {
  const config: GenerationConfig = {};
  if (typeof input.max_tokens === "number") {
    config.maxOutputTokens = input.max_tokens;
  }
  if (typeof input.temperature === "number") {
    config.temperature = input.temperature;
  }
  if (typeof input.top_p === "number") {
    config.topP = input.top_p;
  }
  if (typeof input.top_k === "number") {
    config.topK = input.top_k;
  }
  return config;
}

export function convertToGoogleToolConfig(
  toolChoice: LanguageModelInput["tool_choice"],
): ToolConfig | undefined {
  if (!toolChoice) {
    return undefined;
  }

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
    default: {
      const exhaustiveCheck: never = toolChoice;
      throw new InvalidValueError("toolChoice.type", exhaustiveCheck);
    }
  }
}

export function convertToGoogleTools(
  tools: Tool[],
): FunctionDeclarationsTool[] {
  return [
    {
      functionDeclarations: tools.map((tool): FunctionDeclaration => {
        const declaration: FunctionDeclaration = {
          name: tool.name,
          description: tool.description,
        };
        if (tool.parameters) {
          declaration.parameters = convertToGoogleSchema(
            tool.parameters,
          ) as FunctionDeclarationSchema;
        }
        return declaration;
      }),
    },
  ];
}

export function convertToGoogleResponseFormat(
  responseFormat: LanguageModelInput["response_format"],
): GenerationConfig | undefined {
  if (responseFormat?.type === "json") {
    const generationConfig: GenerationConfig = {
      responseMimeType: "application/json",
    };
    if (responseFormat.schema) {
      generationConfig.responseSchema = convertToGoogleSchema(
        responseFormat.schema,
      );
    }
    return generationConfig;
  }
  return undefined;
}

export function convertToGoogleSchema(schema: Record<string, unknown>): Schema {
  const allowedFormatValues = ["float", "double", "int32", "int64", "enum"];
  let enumValue = schema["enum"] as string[] | undefined;
  if (typeof schema["const"] === "string") {
    enumValue = [...(enumValue || []), schema["const"]];
  }
  const result: Schema = {};
  if (schema["type"]) {
    result.type = schema["type"] as SchemaType;
  }
  if (
    typeof schema["format"] === "string" &&
    allowedFormatValues.includes(schema["format"])
  ) {
    result.format = schema["format"];
  }
  if (schema["description"]) {
    result.description = schema["description"] as string;
  }
  if (enumValue) {
    result.enum = enumValue;
  }
  // if (schema["maxItems"]) {
  //   result.maxItems = schema["maxItems"] as number;
  // }
  // if (schema["minItems"]) {
  //   result.minItems = schema["minItems"] as number;
  // }
  if (schema["properties"]) {
    result.properties = Object.fromEntries(
      Object.entries(schema["properties"]).map(([key, value]) => [
        key,
        convertToGoogleSchema(value as Record<string, unknown>),
      ]),
    );
  }
  if (schema["required"]) {
    result.required = schema["required"] as string[];
  }
  if (schema["items"]) {
    result.items = convertToGoogleSchema(
      schema["items"] as Record<string, unknown>,
    );
  }
  // if (schema["allOf"]) {
  //   result.allOf = (schema["allOf"] as Record<string, unknown>[]).map((value) =>
  //     convertToGoogleSchema(value),
  //   );
  // }
  // if (schema["anyOf"]) {
  //   result.anyOf = (schema["anyOf"] as Record<string, unknown>[]).map((value) =>
  //     convertToGoogleSchema(value),
  //   );
  // }
  // if (schema["oneOf"]) {
  //   result.oneOf = (schema["oneOf"] as Record<string, unknown>[]).map((value) =>
  //     convertToGoogleSchema(value),
  //   );
  // }
  return result;
}

export function mapGoogleMessage(
  candidate: GenerateContentCandidate,
): AssistantMessage {
  return {
    role: "assistant",
    content: candidate.content.parts
      .map(mapGooglePart)
      .filter((part): part is AssistantMessage["content"][number] => !!part),
  };
}

export function mapGooglePart(part: GooglePart): Part | undefined {
  if (typeof part.text === "string" && part.text) {
    return {
      type: "text",
      text: part.text,
    };
  }
  if (part.inlineData) {
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
        audio_data: part.inlineData.data,
      };
    }
    throw new NotImplementedError(
      "inlineData.mimeType",
      part.inlineData.mimeType,
    );
  }
  if (part.functionCall) {
    return {
      type: "tool-call",
      tool_call_id: genidForToolCall(),
      tool_name: part.functionCall.name,
      args: part.functionCall.args as Record<string, unknown>,
    };
  }
  if (part.codeExecutionResult) {
    throw new NotImplementedError(
      "part.codeExecutionResult",
      part.codeExecutionResult,
    );
  }
  if (part.functionResponse) {
    return {
      type: "tool-result",
      tool_call_id: genidForToolCall(),
      tool_name: part.functionResponse.name,
      result: part.functionResponse.response as ToolResultPart["result"],
    };
  }
  if (part.fileData) {
    throw new NotImplementedError("part.fileData", part.fileData);
  }
  if (part.executableCode) {
    throw new NotImplementedError("part.executableCode", part.executableCode);
  }
  return undefined;
}

export function mapGoogleDelta(
  content: Content,
  existingDeltas: InternalContentDelta[],
): ContentDelta[] {
  const contentDeltas: ContentDelta[] = [];

  content.parts.forEach((googlePart) => {
    const part = mapGooglePart(googlePart);
    if (!part) {
      return;
    }
    switch (part.type) {
      case "tool-result":
      case "image":
        throw new Error(`Unexpected part type for delta: ${part.type}`);
      case "text":
      case "audio": {
        contentDeltas.push({
          index: guessDeltaIndex(part, [...existingDeltas, ...contentDeltas]),
          part,
        });
        break;
      }
      case "tool-call": {
        contentDeltas.push({
          index: guessDeltaIndex(part, [...existingDeltas, ...contentDeltas]),
          part: {
            ...part,
            args: part.args ? JSON.stringify(part.args) : "",
          },
        });
        break;
      }
      default: {
        const exhaustiveCheck: never = part;
        throw new NotImplementedError(
          "part.type",
          (exhaustiveCheck as { type: string }).type,
        );
      }
    }
  });

  return contentDeltas;
}

export function mapGoogleUsage(
  usage: UsageMetadata,
  input: GoogleLanguageModelInput,
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

function genidForToolCall() {
  return Math.random().toString(36).substring(2, 15);
}
