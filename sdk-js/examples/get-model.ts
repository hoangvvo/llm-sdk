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
