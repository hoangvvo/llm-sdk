# llm-sdk

Access the LLM API of different providers using a unified SDK.

## Features

- Supports multiple LLM providers: OpenAI, Anthropic, Google Gemini, Cohere, Mistral
- Supports multi modalities (text, image, audio), including OpenAI newest audio generation capabilities.
- Supports reporting the token usage and calculating cost of a request if given the model's pricing information.

## Specification

The specification serves as the basis to implement the unified LLM SDK in different programming languages. The specification is expressed using JSON schema and can be found in [schema.json](./schema/schema.json).

Implementations in different programming languages should adhere to this specification but may adapt it to the idioms of the respective language or provide additional functionalities.

## Implementations

- [JavaScript](./sdk-js/README.md)
