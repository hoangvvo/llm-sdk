import type { LanguageModelPricing, ModelUsage } from "./types.js";

export function calculateCost(
  usage: ModelUsage,
  pricing: LanguageModelPricing,
) {
  const inputTextTokens =
    usage.input_tokens_details?.text_tokens ?? usage.input_tokens;
  const inputAudioTokens = usage.input_tokens_details?.audio_tokens ?? 0;
  const inputImageTokens = usage.input_tokens_details?.image_tokens ?? 0;
  const inputCachedTextTokens =
    usage.input_tokens_details?.cached_text_tokens ?? 0;
  const inputCachedAudioTokens =
    usage.input_tokens_details?.cached_audio_tokens ?? 0;
  const inputCachedImageTokens =
    usage.input_tokens_details?.cached_image_tokens ?? 0;

  const outputTextTokens =
    usage.output_tokens_details?.text_tokens ?? usage.output_tokens;
  const outputAudioTokens = usage.output_tokens_details?.audio_tokens ?? 0;
  const outputImageTokens = usage.output_tokens_details?.image_tokens ?? 0;

  return (
    inputTextTokens * (pricing.input_cost_per_text_token ?? 0) +
    inputAudioTokens * (pricing.input_cost_per_audio_token ?? 0) +
    inputImageTokens * (pricing.input_cost_per_image_token ?? 0) +
    inputCachedTextTokens * (pricing.input_cost_per_text_token ?? 0) +
    inputCachedAudioTokens * (pricing.input_cost_per_audio_token ?? 0) +
    inputCachedImageTokens * (pricing.input_cost_per_image_token ?? 0) +
    outputTextTokens * (pricing.output_cost_per_text_token ?? 0) +
    outputAudioTokens * (pricing.output_cost_per_audio_token ?? 0) +
    outputImageTokens * (pricing.output_cost_per_image_token ?? 0)
  );
}
