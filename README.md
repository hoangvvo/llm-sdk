# llm-sdk

Access the LLM API of different providers using a unified SDK.

## Features

- Supports multiple LLM providers with a unified API.
- Handles multiple modalities: Text, Image, and Audio, even in streaming requests.
- Enables function calling.
- Supports streaming responses and includes utilities for building final output from streamed data, including streaming audio.
- Reports token usage and calculates the cost of a request when provided with the model's pricing information.
- Offers consistent serialization and deserialization for data storage across different programming languages.

## Specification

The specification serves as the foundation for implementing the unified LLM SDK in various programming languages. It is expressed using TypeScript in [schemas/sdk.ts](./schema/sdk.ts).

Implementations in different programming languages must strictly adhere to this specification. Specifically, the properties in data structures should retain the same names and types, even if this conflicts with the conventions of the target language, such as camelCase vs snake_case (we follow the latter).

Each implementation may provide additional features.

## LLM SDKs

We provide SDKs to interact with various LLM providers in the following programming languages:

- [JavaScript](./sdk-js)
- [Rust](./sdk-rust)
- [Go](./sdk-go)

### Supported Providers

|                   | OpenAI            | Anthropic                                                 | Google | Cohere | Mistral           |
| ----------------- | ----------------- | --------------------------------------------------------- | ------ | ------ | ----------------- |
| Sampling Params   | ✅ except `top_k` | ✅ except `frequency_penalty`, `presence_penalty`, `seed` | ✅     | ✅     | ✅ except `top_k` |
| Function Calling  | ✅                | ✅                                                        | ✅     | ✅     | ✅                |
| Structured Output | ✅                | ➖                                                        | ✅     | ✅     | ✅                |
| Text Input        | ✅                | ✅                                                        | ✅     | ✅     | ✅                |
| Image Input       | ✅                | ✅                                                        | ✅     | ✅     | ✅                |
| Audio Input       | ✅                | ➖                                                        | ✅     | ➖     | ➖                |
| Text Output       | ✅                | ✅                                                        | ✅     | ✅     | ✅                |
| Image Output      | 🚧                | ➖                                                        | ✅     | ➖     | ➖                |
| Audio Output      | ✅                | ➖                                                        | ➖     | ➖     | ➖                |

Keys:

- ✅: Supported
- 🚧: Not yet implemented
- ➖: Not available from provider

### Language Model Input

A unified format to represent the input for generating responses from the language model, applicable to both non-streaming and streaming requests. The library converts these inputs into corresponding properties for each LLM provider, if applicable. This allows specifying:

- The conversation history, which includes `UserMessage`, `AssistantMessage`, and `ToolMessage`.
- Sampling parameters: `max_tokens`, `temperature`, `top_p`, `top_k`, `presence_penalty`, `frequency_penalty`, and `seed`.
- Tool definitions and tool selection.
- The response format to enforce the model to return structured objects instead of plain text.

See [LanguageModelInput](https://github.com/hoangvvo/llm-sdk/blob/main/schema/sdk.ts#L366).

### Message

`messages` are primitives that make up the conversation history, and `parts` are the building blocks of each message. The library converts them into a format suitable for the underlying LLM provider and maps those from different providers to the unified format.

Three message types are defined in the SDK: `UserMessage`, `AssistantMessage`, and `ToolMessage`.

See [Message](https://github.com/hoangvvo/llm-sdk/blob/main/schema/sdk.ts#L29).

### Part

> [!NOTE]
> Tool calls are implemented as a `Part` instead of being a property of the `AssistantMessage`.

> [!NOTE]
> The `ToolResultPart` content is an array of `Part` instead of a string or an object. This enables non-text results to be returned for LLM providers that support them (e.g., Anthropic Function Calling supports images in tool results).

The following `Part` types are implemented in the SDK: `TextPart`, `ImagePart`, `AudioPart`, `ToolCallPart`, and `ToolResultPart`.

See [Part](https://github.com/hoangvvo/llm-sdk/blob/main/schema/sdk.ts#L16).

For streaming calls, there are also corresponding `XXXPartDelta` types.

## License

[MIT](LICENSE)
