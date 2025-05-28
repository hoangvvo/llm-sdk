import type {
  AudioOptions,
  LanguageModel,
  LanguageModelMetadata,
  Modality,
  ReasoningOptions,
} from "@hoangvvo/llm-sdk";
import { AnthropicModel } from "@hoangvvo/llm-sdk/anthropic";
import { CohereModel } from "@hoangvvo/llm-sdk/cohere";
import { GoogleModel } from "@hoangvvo/llm-sdk/google";
import { MistralModel } from "@hoangvvo/llm-sdk/mistral";
import { OpenAIChatModel, OpenAIModel } from "@hoangvvo/llm-sdk/openai";
import dotenv from "dotenv";
import modelList from "../../website/models.json" with { type: "json" };

function assert(condition: unknown, msg: string): asserts condition {
  if (!condition) {
    throw new Error(msg);
  }
}

try {
  dotenv.config({
    path: "../.env",
  });
} catch {
  // ignore
}

export function getModel(
  provider: string,
  modelId: string,
  metadata?: LanguageModelMetadata,
  apiKey?: string,
): LanguageModel {
  switch (provider) {
    case "openai": {
      apiKey = apiKey ?? process.env["OPENAI_API_KEY"];
      assert(apiKey, "OPENAI_API_KEY is not set");
      return new OpenAIModel(
        {
          apiKey,
          modelId,
        },
        metadata,
      );
    }
    case "openai-chat-completion": {
      apiKey = apiKey ?? process.env["OPENAI_API_KEY"];
      assert(apiKey, "OPENAI_API_KEY is not set");
      return new OpenAIChatModel(
        {
          apiKey,
          modelId,
        },
        metadata,
      );
    }
    case "anthropic": {
      apiKey = apiKey ?? process.env["ANTHROPIC_API_KEY"];
      assert(apiKey, "ANTHROPIC_API_KEY is not set");
      return new AnthropicModel(
        {
          apiKey,
          modelId,
        },
        metadata,
      );
    }
    case "google": {
      apiKey = apiKey ?? process.env["GOOGLE_API_KEY"];
      assert(apiKey, "GOOGLE_API_KEY is not set");
      return new GoogleModel(
        {
          apiKey,
          modelId,
        },
        metadata,
      );
    }
    case "cohere": {
      apiKey = apiKey ?? process.env["CO_API_KEY"];
      assert(apiKey, "CO_API_KEY is not set");
      return new CohereModel(
        {
          apiKey,
          modelId,
        },
        metadata,
      );
    }
    case "mistral": {
      apiKey = apiKey ?? process.env["MISTRAL_API_KEY"];
      assert(apiKey, "MISTRAL_API_KEY is not set");
      return new MistralModel(
        {
          apiKey,
          modelId,
        },
        metadata,
      );
    }
    default:
      throw new Error(`Unsupported provider: ${provider}`);
  }
}

export async function getModelList() {
  return modelList as ModelInfo[];
}

export interface ModelInfo {
  provider: string;
  model_id: string;
  metadata: LanguageModelMetadata;
  // some params options
  audio?: AudioOptions;
  reasoning?: ReasoningOptions;
  modalities?: Modality[];
}
