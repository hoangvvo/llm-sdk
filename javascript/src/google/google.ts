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
  LanguageModelInput,
  Message,
  ModelResponse,
  PartialModelResponse,
  Tool,
} from "../schemas/index.js";
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

  constructor(options: GoogleModelOptions) {
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

    return {
      content: mapGoogleMessage(candidate.content).content,
      ...(result.response.usageMetadata && {
        usage: {
          inputTokens: result.response.usageMetadata.promptTokenCount,
          outputTokens: result.response.usageMetadata.candidatesTokenCount,
        },
      }),
    };
  }

  async *stream(
    input: LanguageModelInput,
  ): AsyncGenerator<PartialModelResponse, ModelResponse> {
    const { stream } = await this.genModel.generateContentStream(
      convertToGoogleParams(input),
    );

    const message: AssistantMessage = {
      role: "assistant",
      content: [],
    };

    for await (const chunk of stream) {
      const candidate = chunk.candidates?.[0];

      if (candidate?.content) {
        const streamingMessage = mapGoogleMessage(candidate.content);

        message.content.length = Math.max(
          message.content.length,
          streamingMessage.content.length,
        );

        for (let i = 0; i < streamingMessage.content.length; i++) {
          const streamingPart = streamingMessage.content[i]!;

          if (streamingPart.type === "text") {
            let part = message.content[i];
            if (!part) {
              part = message.content[i] = {
                type: "text",
                text: "",
              };
            }
            if (part.type !== "text") {
              throw new Error(`unexpected part ${part.type} at index ${i}`);
            }
            part.text += streamingPart.text;
          } else if (streamingPart.type === "tool-call") {
            message.content[i] = streamingPart;
          }

          yield {
            delta: {
              index: i,
              part: streamingPart,
            },
          };
        }
      }
    }

    return {
      content: message.content,
    };
  }
}

function convertToGoogleParams(
  input: LanguageModelInput,
): GenerateContentRequest {
  let toolConfig: ToolConfig | undefined;
  if (input.toolChoice) {
    switch (input.toolChoice.type) {
      case "auto": {
        toolConfig = {
          functionCallingConfig: {
            mode: FunctionCallingMode.AUTO,
          },
        };
        break;
      }
      case "required": {
        toolConfig = {
          functionCallingConfig: {
            mode: FunctionCallingMode.ANY,
          },
        };
        break;
      }
      case "none": {
        toolConfig = {
          functionCallingConfig: {
            mode: FunctionCallingMode.NONE,
          },
        };
        break;
      }
      case "tool": {
        toolConfig = {
          functionCallingConfig: {
            mode: FunctionCallingMode.ANY,
            allowedFunctionNames: [input.toolChoice.toolName],
          },
        };
        break;
      }
    }
  }

  const generationConfig: GenerationConfig = {};
  if (typeof input.maxTokens === "number") {
    generationConfig.maxOutputTokens = input.maxTokens;
  }
  if (typeof input.temperature === "number") {
    generationConfig.temperature = input.temperature;
  }
  if (typeof input.topP === "number") {
    generationConfig.topP = input.topP;
  }
  if (typeof input.topK === "number") {
    generationConfig.topK = input.topK;
  }
  if (input.responseFormat?.type === "json") {
    generationConfig.responseMimeType = "application/json";
    if (input.responseFormat.schema) {
      generationConfig.responseSchema = convertToFunctionDeclarationSchema(
        input.responseFormat.schema,
      ) as Schema;
    }
  }

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

function convertToGoogleMessages(messages: Message[]): Content[] {
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

function convertToGoogleTools(tools: Tool[]): FunctionDeclarationsTool[] {
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

function mapGoogleMessage(content: Content): AssistantMessage {
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

function genidForToolCall() {
  return Math.random().toString(36).substring(2, 15);
}
