---
title: Overview
---

We provide SDKs to interact with various LLM providers in the following programming languages:

- [JavaScript](https://github.com/hoangvvo/llm-sdk/tree/main/sdk-js)
- [Rust](https://github.com/hoangvvo/llm-sdk/tree/main/sdk-rust)
- [Go](https://github.com/hoangvvo/llm-sdk/tree/main/sdk-go)

## Supported Providers

| Provider                     | Sampling Params                                                   | Function Calling | Structured Output | Text Input | Image Input | Audio Input | Citation [^source-as-text] | Text Output | Image Output | Audio Output | Reasoning |
| ---------------------------- | ----------------------------------------------------------------- | ---------------- | ----------------- | ---------- | ----------- | ----------- | -------------------------- | ----------- | ------------ | ------------ | --------- |
| **OpenAI (Responses)**       | ✅ except `top_k`,`frequency_penalty`, `presence_penalty`, `seed` | ✅               | ✅                | ✅         | ✅          | ✅          | ➖                         | ✅          | ✅           | ➖           | ✅        |
| **OpenAI (Chat Completion)** | ✅ except `top_k`                                                 | ✅               | ✅                | ✅         | ✅          | ✅          | ➖                         | ✅          | ➖           | ✅           | ➖        |
| **Anthropic**                | ✅ except `frequency_penalty`, `presence_penalty`, `seed`         | ✅               | ➖                | ✅         | ✅          | ➖          | ✅                         | ✅          | ➖           | ➖           | ✅        |
| **Google**                   | ✅                                                                | ✅               | ✅                | ✅         | ✅          | ✅          | ➖                         | ✅          | ✅           | ✅           | ✅        |
| **Cohere**                   | ✅                                                                | ✅               | ✅                | ✅         | ✅          | ➖          | ✅                         | ✅          | ➖           | ➖           | ✅        |
| **Mistral**                  | ✅ except `top_k`                                                 | ✅               | ✅                | ✅         | ✅          | ✅          | 🚧                         | ✅          | ➖           | ➖           | ✅        |

Keys:

- ✅: Supported
- 🚧: Not yet implemented
- ➖: Not available from provider

[^source-as-text]: Source Input (citation) is not supported by all providers and may be converted to compatible inputs instead.
