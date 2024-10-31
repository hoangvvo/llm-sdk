import type { LanguageModelPricing } from "../models/language-model.js";
import type { ModelUsage } from "../schemas/types.gen.js";

export function calculateCost(
  usage: ModelUsage,
  pricing: LanguageModelPricing,
) {
  let cost = 0;
  if (usage.inputTokensDetail) {
    // use detailed token counts if available
    const textTokens =
      (usage.inputTokensDetail.textTokens ?? usage.inputTokens) || 0;
    const audioTokens = usage.inputTokensDetail.audioTokens || 0;
    const imageTokens = usage.inputTokensDetail.imageTokens || 0;

    cost +=
      textTokens * (pricing.inputCostPerTextToken || 0) +
      audioTokens * (pricing.inputCostPerAudioToken || 0) +
      imageTokens * (pricing.inputCostPerImageToken || 0);
  } else {
    // fallback to usage.inputTokens and assume all tokens are text tokens
    cost += usage.inputTokens * (pricing.inputCostPerTextToken || 0);
  }

  if (usage.outputTokensDetail) {
    // use detailed token counts if available
    const textTokens =
      (usage.outputTokensDetail.textTokens ?? usage.outputTokens) || 0;
    const audioTokens = usage.outputTokensDetail.audioTokens || 0;
    const imageTokens = usage.outputTokensDetail.imageTokens || 0;
    cost +=
      textTokens * (pricing.outputCostPerTextToken || 0) +
      audioTokens * (pricing.outputCostPerAudioToken || 0) +
      imageTokens * (pricing.outputCostPerImageToken || 0);
  } else {
    // fallback to usage.outputTokens and assume all tokens are text tokens
    cost += usage.outputTokens * (pricing.outputCostPerTextToken || 0);
  }

  return cost;
}
