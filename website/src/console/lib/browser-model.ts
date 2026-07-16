import type { LanguageModel, LanguageModelMetadata } from "@hoangvvo/llm-sdk";
import { AnthropicModel } from "@hoangvvo/llm-sdk/anthropic";
import { CohereModel } from "@hoangvvo/llm-sdk/cohere";
import { GoogleModel } from "@hoangvvo/llm-sdk/google";
import { MistralModel } from "@hoangvvo/llm-sdk/mistral";
import { OpenAIChatModel, OpenAIModel } from "@hoangvvo/llm-sdk/openai";

interface BrowserModelOptions {
  provider: string;
  modelId: string;
  metadata: LanguageModelMetadata | undefined;
  apiKey: string;
}

export function createBrowserModel({
  provider,
  modelId,
  metadata,
  apiKey,
}: BrowserModelOptions): LanguageModel {
  const normalizedApiKey = apiKey.trim();
  if (!normalizedApiKey) {
    throw new Error("An API key is required for the selected provider");
  }

  switch (provider) {
    case "openai":
      return new OpenAIModel(
        {
          apiKey: normalizedApiKey,
          modelId,
          dangerouslyAllowBrowser: true,
        },
        metadata,
      );
    case "openai-chat-completion":
      return new OpenAIChatModel(
        {
          apiKey: normalizedApiKey,
          modelId,
          dangerouslyAllowBrowser: true,
        },
        metadata,
      );
    case "anthropic":
      return new AnthropicModel(
        {
          apiKey: normalizedApiKey,
          modelId,
          dangerouslyAllowBrowser: true,
        },
        metadata,
      );
    case "google":
      return new GoogleModel({ apiKey: normalizedApiKey, modelId }, metadata);
    case "cohere":
      return new CohereModel({ apiKey: normalizedApiKey, modelId }, metadata);
    case "mistral":
      return new MistralModel({ apiKey: normalizedApiKey, modelId }, metadata);
    default:
      throw new Error(`Unsupported provider: ${provider}`);
  }
}
