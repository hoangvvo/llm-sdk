import type { LanguageModelPricing } from "../models/language-model.js";
import type { ModelUsage } from "../schemas/types.gen.js";

export function calculateCost(
  usage: ModelUsage,
  pricing: LanguageModelPricing,
) {
  return (
    usage.inputTokens * pricing.inputTokensCost +
    usage.outputTokens * pricing.outputTokensCost
  );
}
