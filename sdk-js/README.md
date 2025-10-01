# @hoangvvo/llm-sdk

A JavaScript library that enables the development of applications that can interact with different language models through a unified interface.

## Installation

```bash
npm install @hoangvvo/llm-sdk
```

You also need to install the provider-specific packages:

```bash
npm install openai
npm install @anthropic-ai/sdk
npm install @google/genai
npm install cohere-ai
npm install @mistralai/mistralai
```

## Usage

All models implement the `LanguageModel` interface:

```typescript
import type { LanguageModel } from "@hoangvvo/llm-sdk";
import { AnthropicModel } from "@hoangvvo/llm-sdk/anthropic";
import { CohereModel } from "@hoangvvo/llm-sdk/cohere";
import { GoogleModel } from "@hoangvvo/llm-sdk/google";
import { MistralModel } from "@hoangvvo/llm-sdk/mistral";
import { OpenAIModel } from "@hoangvvo/llm-sdk/openai";
import assert from "node:assert";

export function getModel(provider: string, modelId: string): LanguageModel {
  switch (provider) {
    case "openai":
      assert(process.env["OPENAI_API_KEY"]);
      return new OpenAIModel({
        apiKey: process.env["OPENAI_API_KEY"],
        modelId,
      });
    case "anthropic":
      assert(process.env["ANTHROPIC_API_KEY"]);
      return new AnthropicModel({
        apiKey: process.env["ANTHROPIC_API_KEY"],
        modelId,
      });
    case "google":
      assert(process.env["GOOGLE_API_KEY"]);
      return new GoogleModel({
        apiKey: process.env["GOOGLE_API_KEY"],
        modelId,
      });
    case "cohere":
      assert(process.env["CO_API_KEY"]);
      return new CohereModel({ apiKey: process.env["CO_API_KEY"], modelId });
    case "mistral":
      assert(process.env["MISTRAL_API_KEY"]);
      return new MistralModel({
        apiKey: process.env["MISTRAL_API_KEY"],
        modelId,
      });
    default:
      throw new Error(`Unsupported provider: ${provider}`);
  }
}
```

Below is an example to generate text:

```typescript
import { getModel } from "./get-model.ts";

const model = getModel("openai", "gpt-4o");

const response = await model.generate({
  messages: [
    {
      role: "user",
      content: [
        {
          type: "text",
          text: "Tell me a story.",
        },
      ],
    },
    {
      role: "assistant",
      content: [
        {
          type: "text",
          text: "What kind of story would you like to hear?",
        },
      ],
    },
    {
      role: "user",
      content: [
        {
          type: "text",
          text: "A fairy tale.",
        },
      ],
    },
  ],
});

console.dir(response, { depth: null });
```

## Examples

Find examples in the [examples](./examples/) folder to learn how to:

- [`generate-text`: Generate text](./examples/generate-text.ts)
- [`stream-text`: Stream text](./examples/stream-text.ts)
- [`generate-image`: Generate image](./examples/generate-image.ts)
- [`generate-audio`: Generate audio](./examples/generate-audio.ts)
- [`stream-audio`: Stream audio](./examples/stream-audio.ts)
- [`describe-image`: Describe image](./examples/describe-image.ts)
- [`summarize-audio`: Summarize audio](./examples/summarize-audio.ts)
- [`function-calling`: Function calling](./examples/tool-use.ts)
- [`structured-output`: Structured output](./examples/structured-output.ts)
- [`generate-reasoning`: Generate reasoning](./examples/generate-reasoning.ts)
- [`stream-reasoning`: Stream reasoning](./examples/stream-reasoning.ts)
- [`generate-citations`: Generate citations (RAG)](./examples/generate-citations.ts)
- [`stream-citations`: Stream citations (RAG)](./examples/stream-citations.ts)

```bash
node examples/generate-text.ts
```

## Migration

### To 0.4.0

- `image_data` and `audio_data` have been renamed to just `data` in `ImagePart` and `AudioPart`.

### To 0.3.0

- **OpenAI Chat**. The existing Chat completion `OpenAIModel` has been renamed to `OpenAIChatModel`. The Responses API now powers the `OpenAIModel`.
- **OpenAI Strict**. Response format and function calling schema now forces [`strict` mode](https://platform.openai.com/docs/guides/structured-outputs). The option to opt-in to strict mode has been removed.
- **ESM Only**. The library is now ESM-only. CommonJS is no longer supported. You can continue using the library in CommonJS environment by [using the latest Node.js version](https://nodejs.org/api/modules.html#loading-ecmascript-modules-using-require).
- **Tool result content.** Rename `result` to `content`. Tool result content is now an array of `Part` instead of an object to support Anthropic support for multi-modal tool result.

### To 0.2.0

- **All properties now use snake_case.** Initially, the design allowed properties to be transformed to either camelCase or snake_case based on the programming language. However, this flexibility led to database inconsistencies in mixed-language environments. Adopting snake_case aligns with the most common convention.

## License

[MIT](https://github.com/hoangvvo/llm-sdk/blob/main/LICENSE)
