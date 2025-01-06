# @hoangvvo/llm-sdk

`@hoangvvo/llm-sdk` is a JavaScript library that provides a unified API to access the LLM APIs of various providers.

## Installation

```bash
npm install @hoangvvo/llm-sdk
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

export function getModel(provider: string, modelId: string): LanguageModel {
  switch (provider) {
    case "openai":
      return new OpenAIModel({
        apiKey: process.env["OPENAI_API_KEY"]!,
        modelId,
      });
    case "anthropic":
      return new AnthropicModel({
        apiKey: process.env["ANTHROPIC_API_KEY"]!,
        modelId,
      });
    case "google":
      return new GoogleModel({
        apiKey: process.env["GOOGLE_API_KEY"]!,
        modelId,
      });
    case "cohere":
      return new CohereModel({ apiKey: process.env["CO_API_KEY"]!, modelId });
    case "mistral":
      return new MistralModel({
        apiKey: process.env["MISTRAL_API_KEY"]!,
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

Find examples in the [examples](./examples/) folder to learn how to:

- [Generate text](./examples/generate-text.ts)
- [Stream text](./examples/stream-text.ts)
- [Describe image](./examples/describe-image.ts)
- [Function calling](./examples/tool-use.ts)
- [Generate audio](./examples/generate-audio.ts)
- [Stream audio](./examples/stream-audio.ts)

```bash
node --env-file=../.env examples/generate-text.ts
```

## License

[MIT](https://github.com/hoangvvo/llm-sdk/blob/main/LICENSE)
