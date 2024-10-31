import type {
  Content,
  FunctionDeclaration,
  FunctionDeclarationSchema,
  FunctionDeclarationsTool,
  GenerateContentRequest,
  GenerationConfig,
  GenerativeModel,
  Part,
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
  AudioEncoding,
  ContentDelta,
  LanguageModelInput,
  Message,
  ModelResponse,
  ModelUsage,
  PartialModelResponse,
  Tool,
} from "../schemas/index.js";
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
      content: mapGoogleMessage(candidate.content).content,
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
        const incomingContentDeltas = mapGoogleDelta(candidate.content);
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
      content: accumulator.computeContent(),
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
    const parts = message.content.map((part): Part => {
      switch (part.type) {
        case "text": {
          return {
            text: part.text,
          };
        }
        case "image": {
          return {
            inlineData: {
              data: part.imageData,
              mimeType: part.mimeType,
            },
          };
        }
        case "audio": {
          return {
            inlineData: {
              data: part.audioData,
              mimeType: convertToAudioMimeType(part.encoding),
            },
          };
        }
        case "tool-call": {
          return {
            functionCall: {
              name: part.toolName,
              args: part.args || {},
            },
          };
        }
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
      }
    });

    if (message.role === "assistant") {
      return {
        role: "model",
        parts,
      };
    } else if (message.role === "tool") {
      return {
        role: "function",
        parts,
      };
    } else {
      return {
        role: "user",
        parts,
      };
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
    case "auto": {
      return {
        functionCallingConfig: {
          mode: FunctionCallingMode.AUTO,
        },
      };
    }
    case "required": {
      return {
        functionCallingConfig: {
          mode: FunctionCallingMode.ANY,
        },
      };
    }
    case "none": {
      return {
        functionCallingConfig: {
          mode: FunctionCallingMode.NONE,
        },
      };
    }
    case "tool": {
      return {
        functionCallingConfig: {
          mode: FunctionCallingMode.ANY,
          allowedFunctionNames: [toolChoice.toolName],
        },
      };
    }
  }
}

export function convertToGoogleTools(
  tools: Tool[],
): FunctionDeclarationsTool[] {
  return [
    {
      functionDeclarations: tools.map((tool): FunctionDeclaration => {
        return {
          name: tool.name,
          description: tool.description,
          ...(!!tool.parameters && {
            parameters: tool.parameters as unknown as FunctionDeclarationSchema,
          }),
        };
      }),
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
      generationConfig.responseSchema = responseFormat.schema;
    }
  }
  return undefined;
}

export function convertToAudioMimeType(encoding: AudioEncoding): string {
  switch (encoding) {
    case "linear16":
      return "audio/wav";
    case "flac":
      return "audio/flac";
    case "mp3":
      return "audio/mpeg";
    case "mulaw":
      return "audio/x-wav";
    case "opus":
      return "audio/ogg";
    default:
      throw new Error(`unsupported audio encoding: ${encoding}`);
  }
}

export function mapGoogleMessage(content: Content): AssistantMessage {
  return {
    role: "assistant",
    content: content.parts.map((part): AssistantMessage["content"][number] => {
      if (part.functionCall) {
        return {
          type: "tool-call",
          // IMPORTANT: Gemini does not generate an ID we expect for tool calls
          toolCallId: genidForToolCall(),
          toolName: part.functionCall.name,
          args: (part.functionCall.args as Record<string, unknown>) || {},
        };
      }
      if (part.text) {
        return {
          type: "text",
          text: part.text,
        };
      }
      throw new Error("unknown part type");
    }),
  };
}

export function mapGoogleDelta(content: Content): ContentDelta[] {
  // google does not stream partials for tool calls so it is safe to do this
  const streamingMessage = mapGoogleMessage(content);
  return streamingMessage.content.map((part, index): ContentDelta => {
    if (part.type === "tool-call") {
      return {
        index,
        part: {
          ...part,
          // our ToolCallPartDelta only accepts text
          args: part.args ? JSON.stringify(part.args) : "",
        },
      };
    }
    return {
      index,
      part,
    };
  });
}

function mapGoogleUsage(usage: UsageMetadata): ModelUsage {
  return {
    inputTokens: usage.promptTokenCount,
    outputTokens: usage.candidatesTokenCount,
  };
}

function genidForToolCall() {
  return Math.random().toString(36).substring(2, 15);
}
