import { AnthropicModel } from "../src/anthropic/anthropic.js";
import { CohereModel } from "../src/cohere/cohere.js";
import { GoogleModel } from "../src/google/google.js";
import { MistralModel } from "../src/mistral/mistral.js";
import { OpenAIModel } from "../src/openai/openai.js";

export const openaiModel = new OpenAIModel(
  {
    modelId: "gpt-4o",
    apiKey: process.env["OPENAI_API_KEY"] as string,
  },
  {
    pricing: {
      inputCostPerTextToken: 2.5 / 1_000_000,
      outputCostPerTextToken: 10 / 1_000_000,
    },
  },
);
export const openaiAudioModel = new OpenAIModel(
  {
    modelId: "gpt-4o-audio-preview",
    apiKey: process.env["OPENAI_API_KEY"] as string,
  },
  {
    pricing: {
      inputCostPerTextToken: 2.5 / 1_000_000,
      outputCostPerTextToken: 10 / 1_000_000,
      inputCostPerAudioToken: 100 / 1_000_000,
      outputCostPerAudioToken: 200 / 1_000_000,
    },
  },
);
export const anthropicModel = new AnthropicModel(
  {
    modelId: "claude-3-5-sonnet-20241022",
    apiKey: process.env["ANTHROPIC_API_KEY"] as string,
  },
  {
    pricing: {
      inputCostPerTextToken: 3.0 / 1_000_000,
      outputCostPerTextToken: 15.0 / 1_000_000,
    },
  },
);
export const googleModel = new GoogleModel(
  {
    modelId: "gemini-1.5-pro",
    apiKey: process.env["GOOGLE_API_KEY"] as string,
  },
  {
    pricing: {
      inputCostPerTextToken: 1.25 / 1_000_000,
      outputCostPerTextToken: 5.0 / 1_000_000,
    },
  },
);
export const cohereModel = new CohereModel(
  {
    modelId: "command-r-08-2024",
    apiKey: process.env["CO_API_KEY"] as string,
  },
  {
    pricing: {
      inputCostPerTextToken: 0.16 / 1_000_000,
      outputCostPerTextToken: 0.6 / 1_000_000,
    },
  },
);
export const mistralModel = new MistralModel(
  {
    modelId: "mistral-small-2409",
    apiKey: process.env["MISTRAL_API_KEY"] as string,
  },
  {
    pricing: {
      inputCostPerTextToken: 0.2 / 1_000_000,
      outputCostPerTextToken: 0.6 / 1_000_000,
    },
  },
);
