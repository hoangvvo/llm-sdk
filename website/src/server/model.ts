import type { LanguageModel, LanguageModelMetadata } from "@hoangvvo/llm-sdk";
import { AnthropicModel } from "@hoangvvo/llm-sdk/anthropic";
import { CohereModel } from "@hoangvvo/llm-sdk/cohere";
import { GoogleModel } from "@hoangvvo/llm-sdk/google";
import { MistralModel } from "@hoangvvo/llm-sdk/mistral";
import { OpenAIChatModel, OpenAIModel } from "@hoangvvo/llm-sdk/openai";

function requireApiKey(apiKey: string | null): string {
  const value = apiKey?.trim();
  if (!value) {
    throw new Error("An API key is required for the selected provider");
  }
  return value;
}

export function getModel(
  provider: string,
  modelId: string,
  metadata: LanguageModelMetadata | undefined,
  authorization: string | null,
): LanguageModel {
  const apiKey = requireApiKey(authorization);

  switch (provider) {
    case "openai":
      return new OpenAIModel({ apiKey, modelId }, metadata);
    case "openai-chat-completion":
      return new OpenAIChatModel({ apiKey, modelId }, metadata);
    case "anthropic":
      return new AnthropicModel({ apiKey, modelId }, metadata);
    case "google":
      return new GoogleModel({ apiKey, modelId }, metadata);
    case "cohere":
      return new CohereModel({ apiKey, modelId }, metadata);
    case "mistral":
      return new MistralModel({ apiKey, modelId }, metadata);
    default:
      throw new Error(`Unsupported provider: ${provider}`);
  }
}
