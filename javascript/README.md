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

const model = new OpenAIModel({
  apiKey: "openai-api-key",
  modelId: "gpt-3.5-turbo",
});
```

Generate text:

```javascript
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

Stream text:

```javascript
const response = await model.stream({
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

let current = await response.next();
while (!current.done) {
  console.dir(current.value, { depth: null });
  current = await response.next();
}

console.dir(current.value, { depth: null });
```

Tool use:

```javascript
const response = await model.generate({
  messages: [
    {
      role: "user",
      content: [
        {
          type: "text",
          text: "I would like to buy 50 NVDA stocks.",
        },
      ],
    },
    {
      role: "assistant",
      content: [
        {
          type: "tool-call",
          toolName: "trade",
          args: {
            action: "buy",
            quantity: 50,
            symbol: "NVDA",
          },
          toolCallId: "1",
        },
      ],
    },
    {
      role: "tool",
      content: [
        {
          type: "tool-result",
          toolCallId: "1",
          result: {
            status: "success",
          },
          toolName: "trade",
        },
      ],
    },
  ],
  tools: [
    {
      name: "trade",
      description: "Trade stocks",
      parameters: {
        type: "object",
        properties: {
          action: {
            type: "string",
            description: "The action to perform",
          },
          quantity: {
            type: "number",
            description: "The number of stocks to trade",
          },
          symbol: {
            type: "string",
            description: "The stock symbol",
          },
        },
      },
    },
  ],
});

console.dir(response, { depth: null });
```

## License

MIT
