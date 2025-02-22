# @hoangvvo/llm-sdk in JavaScript

A unified LLM API.

## Installation

```bash
npm install @hoangvvo/llm-sdk
```

## Usage

Create a model instance:

```javascript
import { OpenAIModel } from "@hoangvvo/llm-sdk/openai";
// or
import { GoogleModel } from "@hoangvvo/llm-sdk/google";
// or
import { AnthropicModel } from "@hoangvvo/llm-sdk/anthropic";
// or
import { CohereModel } from "@hoangvvo/llm-sdk/cohere";
// or
import { MistralModel } from "@hoangvvo/llm-sdk/mistral";

const model = new OpenAIModel({
  apiKey: "openai-api-key",
  modelId: "gpt-3.5-turbo",
});
```

See [examples](./examples/) or [test cases](./test/) for more details.

## Migration

### To 0.2.0

- **All properties now use snake_case.** Initially, the design allowed properties to be transformed to either camelCase or snake_case based on the programming language. However, this flexibility led to database inconsistencies in mixed-language environments. Adopting snake_case aligns with the most common convention.

## License

MIT
