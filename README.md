# llm-sdk

A suite of library to interact with various Large Language Model (LLM) providers through a unified API and build agentic AI applications.

Two libraries are provided:

- [LLM SDK](#llm-sdks): Unified SDKs to interact with various LLM providers.
- [LLM Agent](#llm-agent): An abstraction to build agentic AI applications using the LLM SDK.

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

### Language Model

A language model instance satisfies the `LanguageModel` interface, which includes the following:

- `provider`: The LLM provider name.
- `model_id`: The model identifier.
- `metadata`: Metadata about the model, such as pricing information or capabilities.
- `generate(LanguageModelInput) -> ModelResponse`: Generate a non-streaming response from the model.
- `stream(LanguageModelInput) -> AsyncIterable<PartialModelResponse>`: Generate a streaming response from the model.

### Language Model Input

`LanguageModelInput` is a unified format to represent the input for generating responses from the language model, applicable to both non-streaming and streaming requests. The library converts these inputs into corresponding properties for each LLM provider, if applicable. This allows specifying:

- The conversation history, which includes `UserMessage`, `AssistantMessage`, and `ToolMessage`.
- Sampling parameters: `max_tokens`, `temperature`, `top_p`, `top_k`, `presence_penalty`, `frequency_penalty`, and `seed`.
- Tool definitions and tool selection.
- The response format to enforce the model to return structured objects instead of plain text.

See [LanguageModelInput](https://github.com/hoangvvo/llm-sdk/blob/main/schema/sdk.ts#L366).

### Message

`Message`s are primitives that make up the conversation history, and `Part`s are the building blocks of each message. The library converts them into a format suitable for the underlying LLM provider and maps those from different providers to the unified format.

Three message types are defined in the SDK: `UserMessage`, `AssistantMessage`, and `ToolMessage`.

See [Message](https://github.com/hoangvvo/llm-sdk/blob/main/schema/sdk.ts#L29).

### Part

> [!NOTE]
> Tool calls are implemented as a `Part` instead of being a property of the `AssistantMessage`.

> [!NOTE]
> The `ToolResultPart` content is an array of `Part` instead of a string or an object. This enables non-text results to be returned for LLM providers that support them (e.g., Anthropic Function Calling supports images in tool results).

The following `Part` types are implemented in the SDK: `TextPart`, `ImagePart`, `AudioPart`, `ToolCallPart`, and `ToolResultPart`.

See [`Part`](https://github.com/hoangvvo/llm-sdk/blob/main/schema/sdk.ts#L16).

For streaming calls, there are also corresponding [`PartDelta`](https://github.com/hoangvvo/llm-sdk/blob/main/schema/sdk.ts#L25) types.

### Model Response

The response from the language model is represented as a `ModelResponse` that includes:

- `content`: An array of `Part` that represents the generated content, which usually comes from the `AssistantMessage`.
- `usage`: Token usage information, if available.
- `cost`: The estimated cost of the request, if the model's pricing information is provided.

For streaming calls, the response is represented as a series of `PartialModelResponse` objects that include:

- `delta`: A `PartDelta` and its index in the eventual `content` array.
- `usage`: Token usage information, if available.

All SDKs provide the `StreamAccumulator` utility to help build the final `ModelResponse` from a stream of `PartialModelResponse`.

## LLM Agent

Agents enable the development of agentic AI applications that can generate responses and execute tasks autonomously. Agents utilize the LLM SDK to interact with different language models and allow definitions of instructions, tools, and other language model parameters.

We provide Agent implementations in the following programming languages:

- [JavaScript](./agent-js)
- [Rust](./agent-rust)
- [Go](./agent-go)

### Agent Definition

The agent is constructed with the following parameters:

- `name`: The identifier of the agent.
- `model`: The language model instance from the LLM SDK.
- `instructions`: A list of instructions to be injected into the system prompt to guide the agent's behavior.
- `tools`: A list of _executable_ tools that the agent can call during its execution.
- `response_format`: The expected response format from the agent. While the default is plain text, it can be customized to return structured output.
- `max_turns`: The maximum number of turns the agent can take to complete a request.

In addition, the agent is defined with a `context` type that can be accessed in the instructions (for dynamic instructions) and tools.

### Agent Tools

An agent tool is defined with the following properties:

- `name`: The identifier of the tool.
- `description`: A description of the tool to instruct the model how and when to use it.
- `parameters`: The JSON schema of the parameters that the tool accepts. The type must be "object".
- `execute(args, context, state)`: The function that will be called to execute the tool with given parameters and context.

The `execute` function must always return an `AgentToolResult`, which includes:

- `content`: The content generated by the tool, which is an array of `Part`, allowing multi-modal outputs for language models that support them.
- `is_error`: A boolean indicating whether the tool execution resulted in an error. Some language models utilize this property to guide its behavior.

### Agent Run

An agent run is initiated by calling the `run` method with an `AgentRequest`, which includes the following:

- `messages`: The conversation history
- `context`: A user-provided value that can be accessed in instructions and tools.

> [!NOTE]
> Each agent run is stateless, so it is recommended to implement a strategy to persist the conversation history if needed.

Each run will continuously generate LLM completions, parse responses to check for tool calls, execute any tools, and feed the tool results back to the model until one of the following conditions is met:

- The model generates a final response (i.e., no tool call).
- The maximum number of turns is reached.

`AgentResponse` includes the final response with the following properties:

- `items`: A list of `RunItem`, such as `ToolMessage` and `AssistantMessage`, that were generated during the run. This can be used to append to the conversation history for the next run.
- `content`: The final content generated by the agent, which is usually the content of the last `AssistantMessage`.

The library also provides a streaming interface, similar to streaming LLM completions, to stream the agent run progress, including part deltas (e.g., text deltas, audio deltas) and intermediate tool calls. Each event can either be:

- `AgentStreamEventPartial`: Contains the `PartialModelResponse` as generated by the LLM SDK, which includes part deltas.
- `AgentStreamEventMessage`: Contains a full `Message` (e.g., `ToolMessage` or `AssistantMessage`) that was generated during the run.
- `AgentStreamEventResponse`: The final response of the agent run, which includes the `AgentResponse`.

### Agent Patterns

Unlike other agent frameworks, this library does not hardcode any hidden prompt templates or parsing logic. This design choice avoids unnecessary constraints, reduces complexity, and prevents the loss of control that comes with excessive abstraction (see this [blog post](https://octomind.dev/blog/why-we-no-longer-use-langchain-for-building-our-ai-agents) for a deeper discussion).

This prevents the library from implementing specific agent patterns, such as hand-off, memory, and others. The examples folder in each agent library contains implementations of various agent patterns to serve as references.

## License

[MIT](LICENSE)
