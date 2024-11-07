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
import type {
  LanguageModel,
  LanguageModelMetadata,
} from "../models/language-model.js";
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
} from "../schemas/index.js";
import { mapAudioFormatToMimeType } from "../utils/audio.utils.js";
import type { InternalContentDelta } from "../utils/stream.utils.js";
import { ContentDeltaAccumulator } from "../utils/stream.utils.js";
import { calculateCost } from "../utils/usage.utils.js";
import type { GoogleModelOptions } from "./types.js";

export type GoogleLanguageModelInput = LanguageModelInput & {
  extra?: Partial<GenerateContentRequest>;
};

export class GoogleModel implements LanguageModel {
  provider: string;
  modelId: string;
  metadata?: LanguageModelMetadata;

  private genModel: GenerativeModel;

  constructor(options: GoogleModelOptions, metadata?: LanguageModelMetadata) {
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
      convertToGoogleParams(input),
    );

    const candidate = result.response.candidates?.[0];
    if (!candidate) {
      throw new Error("no candidates in response");
    }

    const usage: ModelUsage | undefined = result.response.usageMetadata
      ? mapGoogleUsage(result.response.usageMetadata)
      : undefined;

    return {
      content: mapGoogleMessage(candidate).content,
      ...(usage && { usage }),
      ...(this.metadata?.pricing &&
        usage && { cost: calculateCost(usage, this.metadata.pricing) }),
    };
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse, ModelResponse> {
    const { stream } = await this.genModel.generateContentStream(
      convertToGoogleParams(input),
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
        usage = mapGoogleUsage(chunk.usageMetadata);
      }
    }

    return {
      content: accumulator.computeContent().filter((part) => {
        // It is observed that Google sometimes sends empty text parts
        if (part.type === "text" && part.text === "") {
          return false;
        }
        return true;
      }),
      ...(usage && { usage }),
    };
  }
}

export function convertToGoogleParams(
  input: GoogleLanguageModelInput,
): GenerateContentRequest {
  const toolConfig = convertToGoogleToolConfig(input.toolChoice);

  const samplingParams = convertToGoogleSamplingParams(input);

  return {
    ...(!!input.systemPrompt && {
      systemInstruction: input.systemPrompt,
    }),
    contents: convertToGoogleMessages(input.messages),
    ...(input.tools && {
      tools: convertToGoogleTools(input.tools),
    }),
    ...(toolConfig && { toolConfig }),
    generationConfig: {
      ...samplingParams,
      ...input.extra?.["generationConfig"],
      ...(convertToGoogleResponseFormat(input.responseFormat) || {}),
      // TODO: this does not consider input tokens
    },
    ...input.extra,
  };
}

export function convertToGoogleMessages(messages: Message[]): Content[] {
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
              data: part.imageData,
              mimeType: part.mimeType,
            },
          };
        case "audio":
          return {
            inlineData: {
              data: part.audioData,
              mimeType: mapAudioFormatToMimeType(part),
            },
          };
        case "tool-call":
          return {
            functionCall: {
              name: part.toolName,
              args: part.args || {},
            },
          };
        case "tool-result": {
          let response = part.result as object;

          if (Array.isArray(response)) {
            // NOTE: Gemini does not work with Array
            response = { result: response };
          }

          // NOTE: Gemini does not accept a tool call id
          return {
            functionResponse: {
              name: part.toolName,
              response,
            },
          };
        }
        default: {
          const exhaustiveCheck: never = part;
          throw new Error(
            `Unsupported part type: ${(exhaustiveCheck as { type: string }).type}`,
          );
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
        throw new Error(
          `Unsupported message role: ${(exhaustiveCheck as { role: string }).role}`,
        );
      }
    }
  });
}

export function convertToGoogleSamplingParams(
  input: Partial<LanguageModelInput>,
) {
  return {
    ...(typeof input.maxTokens === "number" && {
      maxOutputTokens: input.maxTokens,
    }),
    ...(typeof input.temperature === "number" && {
      temperature: input.temperature,
    }),
    ...(typeof input.topP === "number" && {
      topP: input.topP,
    }),
    ...(typeof input.topK === "number" && {
      topK: input.topK,
    }),
  } satisfies GenerationConfig;
}

export function convertToGoogleToolConfig(
  toolChoice: LanguageModelInput["toolChoice"],
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
          allowedFunctionNames: [toolChoice.toolName],
        },
      };
    default: {
      const exhaustiveCheck: never = toolChoice;
      throw new Error(
        `Unsupported tool choice type: ${(exhaustiveCheck as { type: string }).type}`,
      );
    }
  }
}

export function convertToGoogleTools(
  tools: Tool[],
): FunctionDeclarationsTool[] {
  return [
    {
      functionDeclarations: tools.map(
        (tool): FunctionDeclaration => ({
          name: tool.name,
          description: tool.description,
          ...(!!tool.parameters && {
            parameters: convertToGoogleSchema(
              tool.parameters,
            ) as FunctionDeclarationSchema,
          }),
        }),
      ),
    },
  ];
}

export function convertToGoogleResponseFormat(
  responseFormat: LanguageModelInput["responseFormat"],
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

  return {
    ...(!!schema["type"] && { type: schema["type"] as SchemaType }),
    ...(typeof schema["format"] === "string" &&
      allowedFormatValues.includes(schema["format"]) && {
        format: schema["format"],
      }),
    ...(!!schema["description"] && {
      description: schema["description"] as string,
    }),
    ...(!!enumValue && { enum: enumValue }),
    ...(!!schema["maxItems"] && { maxItems: schema["maxItems"] as number }),
    ...(!!schema["minItems"] && { minItems: schema["minItems"] as number }),
    ...(!!schema["properties"] && {
      properties: Object.fromEntries(
        Object.entries(schema["properties"]).map(([key, value]) => [
          key,
          convertToGoogleSchema(value as Record<string, unknown>),
        ]),
      ),
    }),
    ...(!!schema["required"] && { required: schema["required"] as string[] }),
    ...(!!schema["items"] && {
      items: convertToGoogleSchema(schema["items"] as Record<string, unknown>),
    }),
    ...(!!schema["allOf"] && {
      allOf: (schema["allOf"] as Record<string, unknown>[]).map((value) =>
        convertToGoogleSchema(value),
      ),
    }),
    ...(!!schema["anyOf"] && {
      anyOf: (schema["anyOf"] as Record<string, unknown>[]).map((value) =>
        convertToGoogleSchema(value),
      ),
    }),
    ...(!!schema["oneOf"] && {
      oneOf: (schema["oneOf"] as Record<string, unknown>[]).map((value) =>
        convertToGoogleSchema(value),
      ),
    }),
  };
}

export function mapGoogleMessage(
  candidate: GenerateContentCandidate,
): AssistantMessage {
  return {
    role: "assistant",
    content: candidate.content.parts.map(
      mapGooglePart,
    ) as AssistantMessage["content"],
  };
}

export function mapGooglePart(part: GooglePart): Part {
  if (typeof part.text === "string") {
    return {
      type: "text",
      text: part.text,
    };
  }
  if (part.inlineData) {
    if (part.inlineData.mimeType.startsWith("image/")) {
      return {
        type: "image",
        imageData: part.inlineData.data,
        mimeType: part.inlineData.mimeType,
      };
    }
    if (part.inlineData.mimeType.startsWith("audio/")) {
      return {
        type: "audio",
        audioData: part.inlineData.data,
      };
    }
    throw new Error(
      "unsupported inlineData mimeType: " + part.inlineData.mimeType,
    );
  }
  if (part.functionCall) {
    return {
      type: "tool-call",
      toolCallId: genidForToolCall(),
      toolName: part.functionCall.name,
      // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
      args: (part.functionCall.args as Record<string, unknown>) || null,
    };
  }
  if (part.codeExecutionResult) {
    throw new Error("codeExecutionResult is not supported");
  }
  if (part.functionResponse) {
    return {
      type: "tool-result",
      toolCallId: genidForToolCall(),
      toolName: part.functionResponse.name,
      result: part.functionResponse.response as ToolResultPart["result"],
    };
  }
  if (part.fileData) {
    throw new Error("fileData is not supported");
  }
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
  if (part.executableCode) {
    throw new Error("executableCode is not supported");
  }
  const exhaustiveCheck: never = part;
  throw new Error(
    "unknown part properties: " + Object.keys(exhaustiveCheck).join(", "),
  );
}

export function mapGoogleDelta(
  content: Content,
  existingDeltas: InternalContentDelta[],
): ContentDelta[] {
  // google does not stream partials for tool calls so it is safe to do this
  return content.parts.map(mapGooglePart).map((part): ContentDelta => {
    const partOfSameTypeIndex = existingDeltas.findIndex(
      (delta) => delta.part.type === part.type,
    );
    switch (part.type) {
      case "tool-call": {
        let index: number;
        if (partOfSameTypeIndex === -1) {
          index = existingDeltas.length;
        } else {
          const sameToolNameIndex = existingDeltas.findIndex(
            (delta) =>
              delta.part.type === "tool-call" &&
              delta.part.toolName === part.toolName,
          );
          index =
            sameToolNameIndex === -1
              ? existingDeltas.length
              : sameToolNameIndex;
        }
        return {
          index,
          part: {
            ...part,
            args: part.args ? JSON.stringify(part.args) : "",
          },
        };
      }
      case "text":
      case "audio":
        return {
          index:
            partOfSameTypeIndex === -1
              ? existingDeltas.length
              : partOfSameTypeIndex,
          part,
        };
      case "image":
      case "tool-result":
        throw new Error(`Unsupported part type for delta: ${part.type}`);
      default: {
        const exhaustiveCheck: never = part;
        throw new Error(
          `Unsupported part type: ${(exhaustiveCheck as { type: string }).type}`,
        );
      }
    }
  });
}

export function mapGoogleUsage(usage: UsageMetadata): ModelUsage {
  return {
    inputTokens: usage.promptTokenCount,
    outputTokens: usage.candidatesTokenCount,
  };
}

function genidForToolCall() {
  return Math.random().toString(36).substring(2, 15);
}
