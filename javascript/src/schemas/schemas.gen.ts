// This file is auto-generated by @hey-api/openapi-ts

export const TextPartSchema = {
    type: 'object',
    description: 'A part of the message that contains text.',
    properties: {
        type: {
            type: 'string',
            const: 'text'
        },
        text: {
            type: 'string'
        }
    },
    required: ['type', 'text']
} as const;

export const ImagePartSchema = {
    type: 'object',
    description: 'A part of the message that contains an image.',
    properties: {
        type: {
            type: 'string',
            const: 'image'
        },
        mimeType: {
            type: 'string',
            description: 'The MIME type of the image. E.g. "image/jpeg", "image/png".'
        },
        imageData: {
            type: 'string',
            description: 'The base64-encoded image data.'
        },
        width: {
            type: 'integer',
            description: 'The width of the image in pixels.'
        },
        height: {
            type: 'integer',
            description: 'The height of the image in pixels.'
        }
    },
    required: ['type', 'mimeType', 'imageData']
} as const;

export const AudioEncodingSchema = {
    type: 'string',
    enum: ['linear16', 'flac', 'mulaw', 'mp3', 'opus', 'vorbis'],
    description: 'The encoding of the audio.'
} as const;

export const AudioContainerSchema = {
    type: 'string',
    enum: ['wav', 'ogg', 'flac', 'webm'],
    description: 'The container format of the audio.'
} as const;

export const AudioPartSchema = {
    type: 'object',
    description: 'A part of the message that contains an audio.',
    properties: {
        type: {
            type: 'string',
            const: 'audio'
        },
        container: {
            '$ref': '#/components/schemas/AudioContainer'
        },
        audioData: {
            type: 'string',
            description: 'The base64-encoded audio data.'
        },
        encoding: {
            '$ref': '#/components/schemas/AudioEncoding'
        },
        sampleRate: {
            type: 'integer',
            description: 'The sample rate of the audio. E.g. 44100, 48000.'
        },
        channels: {
            type: 'integer',
            description: 'The number of channels of the audio. E.g. 1, 2.'
        },
        transcript: {
            type: 'string',
            description: 'The transcript of the audio.'
        }
    },
    required: ['type', 'audioData']
} as const;

export const ToolCallPartSchema = {
    type: 'object',
    description: 'A part of the message that represents a call to a tool the model wants to use.',
    properties: {
        type: {
            type: 'string',
            const: 'tool-call'
        },
        toolCallId: {
            type: 'string',
            description: 'The ID of the tool call, used to match the tool result with the tool call.'
        },
        toolName: {
            type: 'string',
            description: 'The name of the tool to call.'
        },
        args: {
            type: 'object',
            additionalProperties: true,
            description: 'The arguments to pass to the tool.',
            nullable: true
        }
    },
    required: ['type', 'toolCallId', 'toolName', 'args']
} as const;

export const ToolResultPartSchema = {
    type: 'object',
    description: 'A part of the message that represents the result of a tool call.',
    properties: {
        type: {
            type: 'string',
            const: 'tool-result'
        },
        toolCallId: {
            type: 'string',
            description: 'The ID of the tool call from previous assistant message.'
        },
        toolName: {
            type: 'string',
            description: 'The name of the tool that was called.'
        },
        result: {
            anyOf: [
                {
                    type: 'object'
                },
                {
                    type: 'array'
                }
            ],
            description: 'The result of the tool call.'
        },
        isError: {
            type: 'boolean',
            description: 'Marks the tool result as an error.'
        }
    },
    required: ['type', 'toolCallId', 'toolName', 'result']
} as const;

export const UserMessageSchema = {
    type: 'object',
    description: 'Represents a message sent by the user.',
    properties: {
        role: {
            type: 'string',
            const: 'user'
        },
        content: {
            type: 'array',
            items: {
                oneOf: [
                    {
                        '$ref': '#/components/schemas/TextPart'
                    },
                    {
                        '$ref': '#/components/schemas/ImagePart'
                    },
                    {
                        '$ref': '#/components/schemas/AudioPart'
                    }
                ]
            }
        }
    },
    required: ['role', 'content']
} as const;

export const AssistantMessageSchema = {
    type: 'object',
    description: 'Represents a message generated by the model.',
    properties: {
        role: {
            type: 'string',
            const: 'assistant'
        },
        content: {
            type: 'array',
            items: {
                oneOf: [
                    {
                        '$ref': '#/components/schemas/TextPart'
                    },
                    {
                        '$ref': '#/components/schemas/ToolCallPart'
                    },
                    {
                        '$ref': '#/components/schemas/AudioPart'
                    }
                ]
            }
        }
    },
    required: ['role', 'content']
} as const;

export const TextPartDeltaSchema = {
    type: 'object',
    properties: {
        type: {
            type: 'string',
            const: 'text'
        },
        text: {
            type: 'string'
        }
    },
    required: ['type', 'text']
} as const;

export const ToolCallPartDeltaSchema = {
    type: 'object',
    properties: {
        type: {
            type: 'string',
            const: 'tool-call'
        },
        toolCallId: {
            type: 'string',
            description: 'The ID of the tool call, used to match the tool result with the tool call.'
        },
        toolName: {
            type: 'string',
            description: 'The name of the tool to call.'
        },
        args: {
            type: 'string',
            description: 'The partial JSON string of the arguments to pass to the tool.'
        }
    }
} as const;

export const AudioPartDeltaSchema = {
    type: 'object',
    properties: {
        type: {
            type: 'string',
            const: 'audio'
        },
        audioData: {
            type: 'string',
            description: 'The base64-encoded audio data.'
        },
        container: {
            '$ref': '#/components/schemas/AudioContainer'
        },
        encoding: {
            '$ref': '#/components/schemas/AudioEncoding'
        },
        sampleRate: {
            type: 'integer',
            description: 'The sample rate of the audio. E.g. 44100, 48000.'
        },
        channels: {
            type: 'integer',
            description: 'The number of channels of the audio. E.g. 1, 2.'
        },
        transcript: {
            type: 'string',
            description: 'The transcript of the audio.'
        }
    },
    required: ['type']
} as const;

export const ContentDeltaSchema = {
    type: 'object',
    properties: {
        index: {
            type: 'integer'
        },
        part: {
            oneOf: [
                {
                    '$ref': '#/components/schemas/TextPartDelta'
                },
                {
                    '$ref': '#/components/schemas/ToolCallPartDelta'
                },
                {
                    '$ref': '#/components/schemas/AudioPartDelta'
                }
            ]
        }
    },
    required: ['index', 'part']
} as const;

export const ToolSchema = {
    type: 'object',
    description: 'Represents a tool that can be used by the model.',
    properties: {
        name: {
            type: 'string',
            description: 'The name of the tool.'
        },
        description: {
            type: 'string',
            description: 'A description of the tool.'
        },
        parameters: {
            type: 'object',
            description: 'The JSON schema of the parameters that the tool accepts. The type must be "object".',
            nullable: true
        }
    },
    required: ['name', 'description', 'parameters']
} as const;

export const ToolMessageSchema = {
    type: 'object',
    description: 'Represents tool result in the message history.',
    properties: {
        role: {
            type: 'string',
            const: 'tool'
        },
        content: {
            type: 'array',
            items: {
                '$ref': '#/components/schemas/ToolResultPart'
            }
        }
    },
    required: ['role', 'content']
} as const;

export const MessageSchema = {
    oneOf: [
        {
            '$ref': '#/components/schemas/UserMessage'
        },
        {
            '$ref': '#/components/schemas/AssistantMessage'
        },
        {
            '$ref': '#/components/schemas/ToolMessage'
        }
    ]
} as const;

export const ModelTokensDetailSchema = {
    type: 'object',
    description: 'Represents the token usage of the model.',
    properties: {
        textTokens: {
            type: 'integer'
        },
        audioTokens: {
            type: 'integer'
        },
        imageTokens: {
            type: 'integer'
        }
    }
} as const;

export const ModelUsageSchema = {
    type: 'object',
    description: 'Represents the token usage of the model.',
    properties: {
        inputTokens: {
            type: 'integer'
        },
        outputTokens: {
            type: 'integer'
        },
        inputTokensDetail: {
            '$ref': '#/components/schemas/ModelTokensDetail'
        },
        outputTokensDetail: {
            '$ref': '#/components/schemas/ModelTokensDetail'
        }
    },
    required: ['inputTokens', 'outputTokens']
} as const;

export const ModelResponseSchema = {
    type: 'object',
    description: 'Represents the response generated by the model.',
    properties: {
        content: {
            type: 'array',
            items: {
                oneOf: [
                    {
                        '$ref': '#/components/schemas/TextPart'
                    },
                    {
                        '$ref': '#/components/schemas/ToolCallPart'
                    },
                    {
                        '$ref': '#/components/schemas/AudioPart'
                    }
                ]
            }
        },
        usage: {
            '$ref': '#/components/schemas/ModelUsage'
        },
        cost: {
            type: 'number',
            description: 'The cost of the response.'
        }
    },
    required: ['content']
} as const;

export const PartialModelResponseSchema = {
    type: 'object',
    properties: {
        delta: {
            '$ref': '#/components/schemas/ContentDelta'
        }
    },
    required: ['delta']
} as const;

export const ToolChoiceAutoSchema = {
    type: 'object',
    description: 'The model will automatically choose the tool to use or not use any tools.',
    properties: {
        type: {
            type: 'string',
            const: 'auto'
        }
    },
    required: ['type']
} as const;

export const ToolChoiceNoneSchema = {
    type: 'object',
    description: 'The model will not use any tools.',
    properties: {
        type: {
            type: 'string',
            const: 'none'
        }
    },
    required: ['type']
} as const;

export const ToolChoiceRequiredSchema = {
    type: 'object',
    description: 'The model will be forced to use a tool.',
    properties: {
        type: {
            type: 'string',
            const: 'required'
        }
    },
    required: ['type']
} as const;

export const ToolChoiceToolSchema = {
    type: 'object',
    description: 'The model will use the specified tool.',
    properties: {
        type: {
            type: 'string',
            const: 'tool'
        },
        toolName: {
            type: 'string'
        }
    },
    required: ['type', 'toolName']
} as const;

export const ResponseFormatTextSchema = {
    type: 'object',
    properties: {
        type: {
            type: 'string',
            const: 'text'
        }
    },
    required: ['type']
} as const;

export const ResponseFormatJsonSchema = {
    type: 'object',
    properties: {
        type: {
            type: 'string',
            const: 'json'
        },
        schema: {
            type: 'object',
            description: 'The JSON schema of the response that the model must output. Not all models support this option.'
        }
    },
    required: ['type']
} as const;

export const ModalitySchema = {
    type: 'string',
    enum: ['text', 'audio']
} as const;

export const LanguageModelInputSchema = {
    type: 'object',
    properties: {
        systemPrompt: {
            type: 'string',
            description: 'A system prompt is a way of providing context and instructions to the model'
        },
        messages: {
            type: 'array',
            items: {
                '$ref': '#/components/schemas/Message'
            },
            description: 'A list of messages comprising the conversation so far.'
        },
        tools: {
            type: 'array',
            items: {
                '$ref': '#/components/schemas/Tool'
            },
            description: 'Definitions of tools that the model may use.'
        },
        toolChoice: {
            oneOf: [
                {
                    '$ref': '#/components/schemas/ToolChoiceAuto'
                },
                {
                    '$ref': '#/components/schemas/ToolChoiceNone'
                },
                {
                    '$ref': '#/components/schemas/ToolChoiceRequired'
                },
                {
                    '$ref': '#/components/schemas/ToolChoiceTool'
                }
            ],
            description: 'Determines how the model should choose which tool to use. "auto" - The model will automatically choose the tool to use or not use any tools. "none" - The model will not use any tools. "required" - The model will be forced to use a tool. { type: "tool", toolName: "toolName" } - The model will use the specified tool.'
        },
        responseFormat: {
            oneOf: [
                {
                    '$ref': '#/components/schemas/ResponseFormatJson'
                },
                {
                    '$ref': '#/components/schemas/ResponseFormatText'
                }
            ],
            description: 'The format that the model must output'
        },
        maxTokens: {
            type: 'integer',
            description: 'The maximum number of tokens that can be generated in the chat completion.'
        },
        temperature: {
            type: 'number',
            description: 'Amount of randomness injected into the response. Ranges from 0.0 to 1.0'
        },
        topP: {
            type: 'number',
            description: 'An alternative to sampling with temperature, called nucleus sampling, where the model considers the results of the tokens with top_p probability mass Ranges from 0.0 to 1.0'
        },
        topK: {
            type: 'number',
            description: 'Only sample from the top K options for each subsequent token. Used to remove "long tail" low probability responses. Ranges from 0.0 to 1.0'
        },
        presencePenalty: {
            type: 'number',
            description: "Positive values penalize new tokens based on whether they appear in the text so far, increasing the model's likelihood to talk about new topics."
        },
        frequencyPenalty: {
            type: 'number',
            description: "Positive values penalize new tokens based on their existing frequency in the text so far, decreasing the model's likelihood to repeat the same line verbatim."
        },
        seed: {
            type: 'integer',
            description: 'The seed (integer), if set and supported by the model, to enable deterministic results.'
        },
        modalities: {
            type: 'array',
            items: {
                '$ref': '#/components/schemas/Modality'
            },
            description: 'The modalities that the model should support.'
        },
        extra: {
            type: 'object',
            description: 'Extra options that the model may support.'
        }
    },
    required: ['messages']
} as const;