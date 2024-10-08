import type {
  Content,
  FunctionDeclaration,
  FunctionDeclarationSchema,
  FunctionDeclarationsTool,
  GenerateContentRequest,
  GenerationConfig,
  GenerativeModel,
  Part,
  Schema,
  ToolConfig,
} from "@google/generative-ai";
import { FunctionCallingMode, GoogleGenerativeAI } from "@google/generative-ai";
import type {
  LanguageModel,
  LanguageModelCapability,
} from "../models/language-model.js";
import type {
  AssistantMessage,
  ContentDelta,
  LanguageModelInput,
  Message,
  ModelResponse,
  ModelUsage,
  PartialModelResponse,
  Tool,
} from "../schemas/index.js";
import { mapContentDeltas, mergeContentDeltas } from "../utils/stream.utils.js";
import type { GoogleModelOptions } from "./types.js";

export class GoogleModel implements LanguageModel {
  provider: string;
  modelId: string;
  capabilities: LanguageModelCapability[] = [
    "streaming",
    "tool",
    "response-format-json",
  ];

  private genModel: GenerativeModel;

  constructor(private options: GoogleModelOptions) {
    this.provider = "google";
    this.modelId = options.modelId;
    const genAI = new GoogleGenerativeAI(options.apiKey);
    this.genModel = genAI.getGenerativeModel({
      model: options.modelId,
    });
  }

  async generate(input: LanguageModelInput): Promise<ModelResponse> {
    const result = await this.genModel.generateContent(
      convertToGoogleParams(input),
    );

    const candidate = result.response.candidates?.[0];
    if (!candidate) {
      throw new Error("no candidates in response");
    }

    const usage: ModelUsage | undefined = result.response.usageMetadata
      ? {
          inputTokens: result.response.usageMetadata.promptTokenCount,
          outputTokens: result.response.usageMetadata.candidatesTokenCount,
        }
      : undefined;

    return {
      content: mapGoogleMessage(candidate.content).content,
      ...(usage && { usage }),
      ...(this.options.pricing &&
        usage && { cost: calculateGoogleCost(usage, this.options.pricing) }),
    };
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse, ModelResponse> {
    const { stream } = await this.genModel.generateContentStream(
      convertToGoogleParams(input),
    );

    let contentDeltas: ContentDelta[] = [];

    for await (const chunk of stream) {
      const candidate = chunk.candidates?.[0];

      if (candidate?.content) {
        const incomingContentDeltas = mapGoogleDelta(candidate.content);
        contentDeltas = mergeContentDeltas(
          contentDeltas,
          incomingContentDeltas,
        );

        for (const delta of incomingContentDeltas) {
          yield { delta };
        }
      }
    }

    return {
      content: mapContentDeltas(contentDeltas),
    };
  }
}

export function convertToGoogleParams(
  input: LanguageModelInput,
): GenerateContentRequest {
  const toolConfig = convertToGoogleToolConfig(input.toolChoice);

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
      ...(convertToGoogleResponseFormat(input.responseFormat) || {}),
      // TODO: this does not consider input tokens
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
    },
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
              mimeType: part.mimeType,
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
            parameters: convertToFunctionDeclarationSchema(tool.parameters),
          }),
        };
      }),
    },
  ];
}

function convertToFunctionDeclarationSchema(
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  schema: any,
): FunctionDeclarationSchema {
  return {
    ...schema,
    // TODO: gemini throws error if format is provided
    format: undefined,
    ...(schema.properties && {
      properties: Object.entries(schema.properties).reduce(
        (acc, [key, value]) => ({
          ...acc,
          [key]: convertToFunctionDeclarationSchema(value),
        }),
        {} as Record<string, FunctionDeclarationSchema>,
      ),
    }),
    ...(schema.items && {
      items: convertToFunctionDeclarationSchema(schema.items),
    }),
  };
}

export function convertToGoogleResponseFormat(
  responseFormat: LanguageModelInput["responseFormat"],
): GenerationConfig | undefined {
  if (responseFormat?.type === "json") {
    const generationConfig: GenerationConfig = {
      responseMimeType: "application/json",
    };
    if (responseFormat.schema) {
      generationConfig.responseSchema = convertToFunctionDeclarationSchema(
        responseFormat.schema,
      ) as Schema;
    }
  }
  return undefined;
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

function genidForToolCall() {
  return Math.random().toString(36).substring(2, 15);
}

function calculateGoogleCost(
  usage: ModelUsage,
  pricing: NonNullable<GoogleModelOptions["pricing"]>,
): number {
  return (
    usage.inputTokens * pricing.inputTokensCost +
    usage.outputTokens * pricing.outputTokensCost
  );
}
