/* eslint-disable @typescript-eslint/no-non-null-assertion */
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
