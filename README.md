# llm-sdk

`llm-sdk` is an open-source suite for building production LLM applications. It ships two libraries:

- **LLM SDK** – cross-language clients (JavaScript, Rust, Go) that talk to multiple LLM providers through one `LanguageModel` interface.
- **LLM Agent** – a minimal, transparent agent library that orchestrates model generations and tool executions using the SDK under the hood.

| Package                         | Language              | Version                                                                                                                                       | Link                                                             |
| ------------------------------- | --------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| `@hoangvvo/llm-sdk`             | JavaScript/TypeScript | [![npm version](https://img.shields.io/npm/v/@hoangvvo/llm-sdk?style=flat-square)](https://www.npmjs.com/package/@hoangvvo/llm-sdk)           | [NPM](https://www.npmjs.com/package/@hoangvvo/llm-sdk)           |
| `llm-sdk-rs`                    | Rust                  | [![crates.io](https://img.shields.io/crates/v/llm-sdk-rs?style=flat-square)](https://crates.io/crates/llm-sdk-rs)                             | [crates.io](https://crates.io/crates/llm-sdk-rs)                 |
| `github.com/hoangvvo/llm-sdk`   | Go                    | [![Go Reference](https://pkg.go.dev/badge/github.com/hoangvvo/llm-sdk/sdk-go.svg)](https://pkg.go.dev/github.com/hoangvvo/llm-sdk/sdk-go)     | [GitHub](https://github.com/hoangvvo/llm-sdk/tree/main/sdk-go)   |
| `@hoangvvo/llm-agent`           | JavaScript/TypeScript | [![npm version](https://img.shields.io/npm/v/@hoangvvo/llm-agent?style=flat-square)](https://www.npmjs.com/package/@hoangvvo/llm-agent)       | [NPM](https://www.npmjs.com/package/@hoangvvo/llm-agent)         |
| `llm-agent`                     | Rust                  | [![crates.io](https://img.shields.io/crates/v/llm-agent?style=flat-square)](https://crates.io/crates/llm-agent)                               | [crates.io](https://crates.io/crates/llm-agent)                  |
| `github.com/hoangvvo/llm-agent` | Go                    | [![Go Reference](https://pkg.go.dev/badge/github.com/hoangvvo/llm-sdk/agent-go.svg)](https://pkg.go.dev/github.com/hoangvvo/llm-sdk/agent-go) | [GitHub](https://github.com/hoangvvo/llm-sdk/tree/main/agent-go) |

The accompanying [Console app](https://llm-sdk.hoangvvo.com/console/chat/) demonstrates the libraries end-to-end.

![Console Chat Application screenshot](./website/assets/console-chat.png)

> **Status**: both libraries are currently `v0`. The SDK library APIs are largely stable; the Agent library APIs may evolve. Feedback and contributions are welcome.

## Why use llm-sdk

- Supports multiple LLM providers with a unified API.
- Handles multiple modalities: Text, Image, and Audio. Supports streaming.
- Supports multi-modality function calling (image/audio returned from tools).
- Supports citations (RAG) and reasoning for supported models.
- Reports token usage and calculates the cost of a request when provided with the model’s pricing information.
- Unified serialization across programming languages (systems in different languages can work together).
- Integrates OpenTelemetry for tracing.
- _Zero abstraction_: the agent library is a thin for-loop around the SDK. No overcomplex abstractions like chains, graphs, or hidden prompt templates.

## LLM SDKs

Choose the language that fits your service and get the same capabilities:

- [JavaScript](./sdk-js)
- [Rust](./sdk-rust)
- [Go](./sdk-go)

Each implements the TypeScript reference specification in [`schema/sdk.ts`](./schema/sdk.ts). Request/response payloads (`LanguageModelInput`, `ModelResponse`, tool events, etc.) keep identical field names when serialized to JSON so services can interoperate across languages.

### Supported providers

| Provider                     | Sampling Params                                                   | Function Calling | Structured Output | Reasoning | Citation [^source-as-text]                                                              | Text Input | Image Input | Audio Input | Text Output | Image Output | Audio Output |
| ---------------------------- | ----------------------------------------------------------------- | ---------------- | ----------------- | --------- | --------------------------------------------------------------------------------------- | ---------- | ----------- | ----------- | ----------- | ------------ | ------------ |
| **OpenAI (Responses)**       | ✅ except `top_k`,`frequency_penalty`, `presence_penalty`, `seed` | ✅               | ✅                | ✅        | ➖                                                                                      | ✅         | ✅          | ✅          | ✅          | ✅           | ➖           |
| **OpenAI (Chat Completion)** | ✅ except `top_k`                                                 | ✅               | ✅                | ➖        | ➖                                                                                      | ✅         | ✅          | ✅          | ✅          | ➖           | ✅           |
| **Anthropic**                | ✅ except `frequency_penalty`, `presence_penalty`, `seed`         | ✅               | ➖                | ✅        | ✅ ([Search results](https://docs.claude.com/en/docs/build-with-claude/search-results)) | ✅         | ✅          | ➖          | ✅          | ➖           | ➖           |
| **Google**                   | ✅                                                                | ✅               | ✅                | ✅        | ➖                                                                                      | ✅         | ✅          | ✅          | ✅          | ✅           | ✅           |
| **Cohere**                   | ✅                                                                | ✅               | ✅                | ✅        | ✅ ([Document](https://docs.cohere.com/v2/docs/retrieval-augmented-generation-rag))     | ✅         | ✅          | ➖          | ✅          | ➖           | ➖           |
| **Mistral**                  | ✅ except `top_k`                                                 | ✅               | ✅                | ✅        | 🚧                                                                                      | ✅         | ✅          | ✅          | ✅          | ➖           | ➖           |

Keys: ✅ supported · 🚧 planned · ➖ not available from provider.

[^source-as-text]: Source Input (citation) is not supported by all providers and may be converted to compatible inputs instead.

### Core interfaces

- `LanguageModel`: supplies provider metadata plus `generate` and `stream` methods that accept a `LanguageModelInput` and return unified responses.
- `LanguageModelInput`: captures conversation history, sampling parameters, tool definitions, response-format hints, and modality toggles. The SDK adapts this shape to each provider’s API.
- `ModelResponse` / `PartialModelResponse`: normalized outputs (with usage/cost when available) that you can forward directly to other services.
- `Message`: building blocks for conversations. Messages represent user, assistant, or tool turns, with a list of parts, each representing a chunk of content in a specific modality:
  - `Part`: `TextPart`, `ImagePart`, `AudioPart`, `SourcePart` (for citation), `ToolCallPart`, `ToolResultPart`, and `ReasoningPart`.
- Tool semantics: function calling and tool-result envelopes share the same schema across providers. The SDK normalizes call IDs, arguments, and error flags so agent runtimes can hydrate rich tool events without per-provider branching.

## LLM Agent

`llm-agent` wraps the SDK to provide a lightweight agent runtime:

- **Agent** objects are stateless blueprints that declare instructions, tools, toolkits, and default model settings.
- **Run sessions** bind an agent to a specific context value. Sessions resolve dynamic instructions once, initialize toolkit state, and stream model/tool events back to you.
- **Agent items** capture every turn: user/assistant messages, model responses (with usage metadata), and rich tool-call records. Append the output list to the next run’s input to continue a conversation.
- **Streaming** mirrors non-streaming responses but emits partial deltas and tool events for real-time UX.

## Getting started

Read the full documentation on [llm-sdk.hoangvvo.com](https://llm-sdk.hoangvvo.com) or start from these guides:

- [SDK Overview](https://llm-sdk.hoangvvo.com/sdk)
- [Language Model basics](https://llm-sdk.hoangvvo.com/sdk/language-model)
- [Agent concepts](https://llm-sdk.hoangvvo.com/agent/agent)
- [Agent run](https://llm-sdk.hoangvvo.com/agent/run)

Also check out some agent patterns, including:

- [Delegation (Agent-as-tools)](https://llm-sdk.hoangvvo.com/agent/delegation)
- [Memory](https://llm-sdk.hoangvvo.com/agent/memory)
- [Artifacts (Canvas)](https://llm-sdk.hoangvvo.com/agent/artifacts)
- [Planner–Executor](https://llm-sdk.hoangvvo.com/agent/planner-executor)
- [Human-in-the-loop](https://llm-sdk.hoangvvo.com/agent/human-in-the-loop)

**Note**: To run examples, create an `.env` file in the root folder (folder containing this README) with your API keys.

## Agent Patterns

This agent **library** (not _framework_) is designed for transparency and control.
Unlike many “agentic” frameworks, it ships with no hidden prompt templates or secret parsing rules, and that’s on purpose:

- Nothing hidden – What you write is what runs. No secret prompts or “special sauce” behind the scenes, so your instructions aren’t quietly overridden.
- Works in any settings – Many frameworks bake in English-only prompts. Here, the model sees only your words, in whichever language or format.
- Easy to tweak – Change prompts, parsing, or flow without fighting built-in defaults.
- Less to debug – Fewer layers mean you can trace exactly where things break.
- No complex abstraction – Don't waste time learning new concepts or APIs (e.g., “chains”, “graphs”, syntax with special meanings, etc.). Just plain functions and data structures.

LLM in the past was not as powerful as today, so frameworks had to do a lot of heavy lifting to get decent results.
But with modern LLMs, much of that complexity is no longer necessary.

Because we keep the core minimal (**only 500 LOC!**) and do not want to introduce such hidden magic, the library doesn’t bundle heavy agent patterns like hand-off, memory, or planners.
Instead, the `examples/` folders shows clean, working references you can copy or adapt to see that it can still be used to build complex use cases.

This philosophy is inspired by this [blog post](https://hamel.dev/blog/posts/prompt/).

## Comparison with other libraries

The initial version of `llm-sdk` was developed internally at my company, prior to the existence or knowledge of similar libraries like the [Vercel AI SDK](https://github.com/vercel/ai) or [OpenAI Swarm](https://github.com/openai/swarm). As a result, it was never intended to compete with or address the limitations of those libraries. As these other libraries matured, `llm-sdk` continued to evolve independently, focusing on its unique features and use cases, which were designed to be sufficient for its intended applications.

This section is designed to outline the differences for those considering migration to or from `llm-sdk` or to assert compatibility.

TBD.

## License

[MIT](LICENSE)
