import type { LanguageModelPricing, ModelUsage } from "../types.js";

export function calculateCost(
  usage: ModelUsage,
  pricing: LanguageModelPricing,
) {
  let cost = 0;
  const inputTextTokens =
    usage.inputTokensDetail?.textTokens ?? usage.inputTokens;
  const inputAudioTokens = usage.inputTokensDetail?.audioTokens ?? 0;
  const inputImageTokens = usage.inputTokensDetail?.imageTokens ?? 0;
  const inputCachedTextTokens = usage.inputTokensDetail?.cachedTextTokens ?? 0;
  const inputCachedAudioTokens =
    usage.inputTokensDetail?.cachedAudioTokens ?? 0;
  const inputCachedImageTokens =
    usage.inputTokensDetail?.cachedImageTokens ?? 0;

  const outputTextTokens =
    usage.outputTokensDetail?.textTokens ?? usage.outputTokens;
  const outputAudioTokens = usage.outputTokensDetail?.audioTokens ?? 0;
  const outputImageTokens = usage.outputTokensDetail?.imageTokens ?? 0;

  cost +=
    inputTextTokens * (pricing.inputCostPerTextToken ?? 0) +
    inputAudioTokens * (pricing.inputCostPerAudioToken ?? 0) +
    inputImageTokens * (pricing.inputCostPerImageToken ?? 0) +
    inputCachedTextTokens * (pricing.inputCostPerTextToken ?? 0) +
    inputCachedAudioTokens * (pricing.inputCostPerAudioToken ?? 0) +
    inputCachedImageTokens * (pricing.inputCostPerImageToken ?? 0) +
    outputTextTokens * (pricing.outputCostPerTextToken ?? 0) +
    outputAudioTokens * (pricing.outputCostPerAudioToken ?? 0) +
    outputImageTokens * (pricing.outputCostPerImageToken ?? 0);

  return cost;
}
