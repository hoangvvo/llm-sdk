import type {
  LanguageModelPricing,
  ModelTokensDetails,
  ModelUsage,
} from "./types.ts";

export interface ModelUsageCostOptions {
  input_cache_tokens_are_additional: boolean;
  output_reasoning_tokens_are_additional: boolean;
}

export function calculateCost(
  usage: ModelUsage,
  pricing: LanguageModelPricing,
  options: ModelUsageCostOptions,
) {
  const inputDetails = usage.input_tokens_details;
  const outputDetails = usage.output_tokens_details;
  let cost =
    usage.input_tokens * (pricing.input_cost_per_text_token ?? 0) +
    usage.output_tokens * (pricing.output_cost_per_text_token ?? 0);

  const adjustment = (
    tokens: number,
    regularPrice: number | undefined,
    categoryPrice: number | undefined,
  ) =>
    categoryPrice === undefined
      ? 0
      : tokens * (categoryPrice - (regularPrice ?? 0));

  cost += adjustment(
    inputDetails?.audio_tokens ?? 0,
    pricing.input_cost_per_text_token,
    pricing.input_cost_per_audio_token,
  );
  cost += adjustment(
    inputDetails?.image_tokens ?? 0,
    pricing.input_cost_per_text_token,
    pricing.input_cost_per_image_token,
  );
  cost += adjustment(
    outputDetails?.audio_tokens ?? 0,
    pricing.output_cost_per_text_token,
    pricing.output_cost_per_audio_token,
  );
  cost += adjustment(
    outputDetails?.image_tokens ?? 0,
    pricing.output_cost_per_text_token,
    pricing.output_cost_per_image_token,
  );

  const hasCachedModalities =
    inputDetails?.cached_text_tokens !== undefined ||
    inputDetails?.cached_audio_tokens !== undefined ||
    inputDetails?.cached_image_tokens !== undefined;
  const hasCachedModalityPricing =
    pricing.input_cost_per_cached_text_token !== undefined ||
    pricing.input_cost_per_cached_audio_token !== undefined ||
    pricing.input_cost_per_cached_image_token !== undefined;
  const cacheBaseText = options.input_cache_tokens_are_additional
    ? undefined
    : pricing.input_cost_per_text_token;
  const cacheBaseAudio = options.input_cache_tokens_are_additional
    ? undefined
    : pricing.input_cost_per_audio_token;
  const cacheBaseImage = options.input_cache_tokens_are_additional
    ? undefined
    : pricing.input_cost_per_image_token;
  if (hasCachedModalities && hasCachedModalityPricing) {
    cost += adjustment(
      inputDetails.cached_text_tokens ?? 0,
      cacheBaseText,
      pricing.input_cost_per_cached_text_token,
    );
    cost += adjustment(
      inputDetails.cached_audio_tokens ?? 0,
      cacheBaseAudio,
      pricing.input_cost_per_cached_audio_token,
    );
    cost += adjustment(
      inputDetails.cached_image_tokens ?? 0,
      cacheBaseImage,
      pricing.input_cost_per_cached_image_token,
    );
  } else {
    cost += adjustment(
      inputDetails?.cached_tokens ?? 0,
      cacheBaseText,
      pricing.input_cost_per_cached_token,
    );
  }

  cost += adjustment(
    inputDetails?.cache_write_tokens ?? 0,
    cacheBaseText,
    pricing.input_cost_per_cache_write_token,
  );

  if (options.output_reasoning_tokens_are_additional) {
    cost +=
      (usage.output_tokens_details?.reasoning_tokens ?? 0) *
      (pricing.output_cost_per_text_token ?? 0);
  }

  return cost;
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
    "cached_tokens",
    "cache_write_tokens",
    "reasoning_tokens",
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
