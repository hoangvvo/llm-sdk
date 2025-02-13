---
title: Overview
---

We provide SDKs to interact with various LLM providers in the following programming languages:

- [JavaScript](https://github.com/hoangvvo/llm-sdk/tree/main/sdk-js)
- [Rust](https://github.com/hoangvvo/llm-sdk/tree/main/sdk-rust)
- [Go](https://github.com/hoangvvo/llm-sdk/tree/main/sdk-go)

## Supported Providers

|                   | OpenAI               | Anthropic                                                 | Google               | Cohere | Mistral              |
| ----------------- | -------------------- | --------------------------------------------------------- | -------------------- | ------ | -------------------- |
| Sampling Params   | ✅ except `top_k`    | ✅ except `frequency_penalty`, `presence_penalty`, `seed` | ✅                   | ✅     | ✅ except `top_k`    |
| Function Calling  | ✅                   | ✅                                                        | ✅                   | ✅     | ✅                   |
| Structured Output | ✅                   | ➖                                                        | ✅                   | ✅     | ✅                   |
| Text Input        | ✅                   | ✅                                                        | ✅                   | ✅     | ✅                   |
| Image Input       | ✅                   | ✅                                                        | ✅                   | ✅     | ✅                   |
| Audio Input       | ✅                   | ➖                                                        | ✅                   | ➖     | ➖                   |
| Citation          | ➖ [^source-as-text] | ✅                                                        | ➖ [^source-as-text] | ✅     | 🚧 [^source-as-text] |
| Text Output       | ✅                   | ✅                                                        | ✅                   | ✅     | ✅                   |
| Image Output      | 🚧                   | ➖                                                        | ✅                   | ➖     | ➖                   |
| Audio Output      | ✅                   | ➖                                                        | ➖                   | ➖     | ➖                   |

Keys:

- ✅: Supported
- 🚧: Not yet implemented
- ➖: Not available from provider

[^source-as-text]: Source Input (citation) is not supported by all providers and may be converted to compatible inputs instead.
