import type {
  Content,
  FunctionDeclarationSchema,
  FunctionDeclarationsTool,
  GenerateContentRequest,
  GenerativeModel,
  Part,
  ToolConfig,
} from "@google/generative-ai";
import {
  FunctionCallingMode,
  FunctionDeclarationSchemaType,
  GoogleGenerativeAI,
} from "@google/generative-ai";
import { nanoid } from "nanoid";
import type {
  AssistantMessage,
  LanguageModel,
  LanguageModelInput,
  Message,
  ModelResponse,
  PartialModelResponse,
  Tool,
} from "../models.js";
import type { GoogleModelOptions } from "./types.js";

export class GoogleModel implements LanguageModel {
  provider: string;
  modelId: string;

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
      convertToGoogleParams(this.modelId, input),
    );

    const candidate = result.response.candidates?.[0];
    if (!candidate) {
      throw new Error("google: no candidates in response");
    }

    return {
      message: mapGoogleMessage(candidate.content),
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
      convertToGoogleParams(this.modelId, input),
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
              throw new Error(
                `google: unexpected part ${part.type} at index ${i}`,
              );
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
      message,
    };
  }
}

function convertToGoogleParams(
  modelId: string,
  input: LanguageModelInput,
): GenerateContentRequest {
  // Gemini 1.0 does not support systemInstructions,
  // so we inject it as a user message
  // TODO: this assumes English in prompt, which could affect the model
  const needInjectSystemPromptAsMessage =
    modelId.includes("gemini-1.0") && !!input.systemPrompt;

  let toolConfig: ToolConfig | undefined;
  if (input.toolChoice) {
    if (input.toolChoice.type === "auto") {
      toolConfig = {
        functionCallingConfig: {
          mode: FunctionCallingMode.AUTO,
        },
      };
    } else if (input.toolChoice.type === "required") {
      toolConfig = {
        functionCallingConfig: {
          mode: FunctionCallingMode.ANY,
        },
      };
    } else if (input.toolChoice.type === "none") {
      toolConfig = {
        functionCallingConfig: {
          mode: FunctionCallingMode.NONE,
        },
      };
    } else if (input.toolChoice.type === "tool") {
      toolConfig = {
        functionCallingConfig: {
          mode: FunctionCallingMode.ANY,
          allowedFunctionNames: [input.toolChoice.toolName],
        },
      };
    }
  }

  return {
    ...(!needInjectSystemPromptAsMessage &&
      !!input.systemPrompt && {
        systemInstruction: input.systemPrompt,
      }),
    contents: [
      ...(needInjectSystemPromptAsMessage
        ? ([
            {
              role: "user",
              parts: [{ text: input.systemPrompt }],
            },
            {
              role: "model",
              parts: [
                {
                  text: "Ok, I got it! Please continue in your native language.",
                },
              ],
            },
          ] as Content[])
        : []),
      ...convertToGoogleMessages(input.messages),
    ],
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
      if (part.type === "tool-result") {
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
      } else if (part.type === "tool-call") {
        return {
          functionCall: {
            name: part.toolName,
            args: part.args || {},
          },
        };
      } else if (part.type === "image") {
        if (part.imageData) {
          return {
            inlineData: {
              data: part.imageData,
              mimeType: part.mimeType,
            },
          };
        } else {
          throw new Error("google: image part must have imageData");
        }
      } else {
        return {
          text: part.text,
        };
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
      functionDeclarations: tools.map((tool) => {
        return {
          name: tool.name,
          description: tool.description,
          schema: tool.parameters
            ? convertToFunctionDeclarationSchema(tool.parameters)
            : undefined,
        };
      }),
    },
  ];
}

// Google has their own type values instead of standard JSONSchema types
const GOOGLE_SCHEMA_TYPE_TO_OPENAPI_MAPPING: Record<
  FunctionDeclarationSchemaType,
  string
> = {
  [FunctionDeclarationSchemaType.STRING]: "string",
  [FunctionDeclarationSchemaType.NUMBER]: "number",
  [FunctionDeclarationSchemaType.INTEGER]: "integer",
  [FunctionDeclarationSchemaType.BOOLEAN]: "boolean",
  [FunctionDeclarationSchemaType.ARRAY]: "array",
  [FunctionDeclarationSchemaType.OBJECT]: "object",
};
const OPENAPI_SCHEMA_TYPE_TO_GOOGLE_MAPPING = Object.fromEntries(
  Object.entries(GOOGLE_SCHEMA_TYPE_TO_OPENAPI_MAPPING).map(([key, value]) => [
    value,
    key,
  ]),
) as Record<string, FunctionDeclarationSchemaType>;
function convertToFunctionDeclarationSchema(
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  schema: any,
): FunctionDeclarationSchema {
  const googleType = OPENAPI_SCHEMA_TYPE_TO_GOOGLE_MAPPING[schema.type];
  if (!googleType) {
    throw new Error(`Unsupported schema type: ${schema.type}`);
  }
  return {
    ...schema,
    type: googleType,
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
          args: part.functionCall.args || {},
        };
      }
      if (part.text) {
        return {
          type: "text",
          text: part.text,
        };
      }
      throw new Error("google: unknown part type");
    }),
  };
}

function genidForToolCall() {
  return nanoid();
}
