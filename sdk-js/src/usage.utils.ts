import type {
  LanguageModelPricing,
  ModelTokensDetails,
  ModelUsage,
} from "./types.ts";

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

export function sumModelUsage(usages: ModelUsage[]): ModelUsage {
  const result = usages.reduce<ModelUsage>(
    (acc, curr) => ({
      input_tokens: acc.input_tokens + curr.input_tokens,
      output_tokens: acc.output_tokens + curr.output_tokens,
    }),
    { input_tokens: 0, output_tokens: 0 },
  );
  const inputDetails = usages.flatMap((usage) =>
    usage.input_tokens_details ? [usage.input_tokens_details] : [],
  );
  const outputDetails = usages.flatMap((usage) =>
    usage.output_tokens_details ? [usage.output_tokens_details] : [],
  );
  if (inputDetails.length > 0) {
    result.input_tokens_details = sumModelTokensDetails(inputDetails);
  }
  if (outputDetails.length > 0) {
    result.output_tokens_details = sumModelTokensDetails(outputDetails);
  }
  return result;
}

export function sumModelTokensDetails(
  detailsArr: ModelTokensDetails[],
): ModelTokensDetails {
  const result: ModelTokensDetails = {};
  const keys = [
    "text_tokens",
    "audio_tokens",
    "image_tokens",
    "cached_text_tokens",
    "cached_audio_tokens",
    "cached_image_tokens",
  ] as const;
  for (const details of detailsArr) {
    for (const key of keys) {
      const value = details[key];
      if (value !== undefined) {
        result[key] = (result[key] ?? 0) + value;
      }
    }
  }
  return result;
}
