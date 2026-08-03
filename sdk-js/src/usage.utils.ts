import type {
  LanguageModelPricing,
  ModelTokensDetails,
  ModelUsage,
} from "./types.ts";

const MODEL_TOKEN_DETAIL_KEYS = [
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

  const value = (value: number | undefined) => Math.max(0, value ?? 0);
  const maxPrice = (...prices: (number | undefined)[]) =>
    Math.max(0, ...prices.filter((price) => price !== undefined));
  const hasModalityBreakdown = (details: ModelTokensDetails | undefined) =>
    details?.text_tokens !== undefined ||
    details?.audio_tokens !== undefined ||
    details?.image_tokens !== undefined;
  const modalityTotal = (details: ModelTokensDetails | undefined) =>
    value(details?.text_tokens) +
    value(details?.audio_tokens) +
    value(details?.image_tokens);

  const inputHasModalityBreakdown = hasModalityBreakdown(inputDetails);
  const inputMaxPrice = maxPrice(
    pricing.input_cost_per_text_token,
    pricing.input_cost_per_audio_token,
    pricing.input_cost_per_image_token,
  );
  const outputMaxPrice = maxPrice(
    pricing.output_cost_per_text_token,
    pricing.output_cost_per_audio_token,
    pricing.output_cost_per_image_token,
  );
  // Unattributed tokens are assumed to be text. The highest configured rate is
  // only a fallback for models, such as TTS, that do not define a text rate.
  const inputBasePrice = pricing.input_cost_per_text_token ?? inputMaxPrice;
  const outputBasePrice = pricing.output_cost_per_text_token ?? outputMaxPrice;

  const hasCachedModalities =
    inputDetails?.cached_text_tokens !== undefined ||
    inputDetails?.cached_audio_tokens !== undefined ||
    inputDetails?.cached_image_tokens !== undefined;
  const cachedReadTokens = hasCachedModalities
    ? value(inputDetails.cached_text_tokens) +
      value(inputDetails.cached_audio_tokens) +
      value(inputDetails.cached_image_tokens)
    : value(inputDetails?.cached_tokens);
  const includedCacheTokens = options.input_cache_tokens_are_additional
    ? 0
    : cachedReadTokens + value(inputDetails?.cache_write_tokens);
  const inputTokens = Math.max(
    value(usage.input_tokens),
    modalityTotal(inputDetails),
    includedCacheTokens,
  );
  const outputTokens = Math.max(
    value(usage.output_tokens),
    modalityTotal(outputDetails) +
      (options.output_reasoning_tokens_are_additional
        ? 0
        : value(outputDetails?.reasoning_tokens)),
  );

  let cost = inputTokens * inputBasePrice + outputTokens * outputBasePrice;

  const adjustment = (
    tokens: number,
    regularPrice: number,
    categoryPrice: number,
  ) => tokens * (categoryPrice - regularPrice);

  const inputTextPrice = pricing.input_cost_per_text_token ?? inputBasePrice;
  const inputAudioPrice = pricing.input_cost_per_audio_token ?? inputTextPrice;
  const inputImagePrice = pricing.input_cost_per_image_token ?? inputTextPrice;
  const outputTextPrice = pricing.output_cost_per_text_token ?? outputBasePrice;
  const outputAudioPrice =
    pricing.output_cost_per_audio_token ?? outputTextPrice;
  const outputImagePrice =
    pricing.output_cost_per_image_token ?? outputTextPrice;

  cost += adjustment(
    value(inputDetails?.audio_tokens),
    inputBasePrice,
    inputAudioPrice,
  );
  cost += adjustment(
    value(inputDetails?.image_tokens),
    inputBasePrice,
    inputImagePrice,
  );
  cost += adjustment(
    value(outputDetails?.audio_tokens),
    outputBasePrice,
    outputAudioPrice,
  );
  cost += adjustment(
    value(outputDetails?.image_tokens),
    outputBasePrice,
    outputImagePrice,
  );

  const cacheBaseText = options.input_cache_tokens_are_additional
    ? 0
    : inputHasModalityBreakdown
      ? inputTextPrice
      : inputBasePrice;
  const cacheBaseAudio = options.input_cache_tokens_are_additional
    ? 0
    : inputHasModalityBreakdown
      ? inputAudioPrice
      : inputBasePrice;
  const cacheBaseImage = options.input_cache_tokens_are_additional
    ? 0
    : inputHasModalityBreakdown
      ? inputImagePrice
      : inputBasePrice;
  if (hasCachedModalities) {
    cost += adjustment(
      value(inputDetails.cached_text_tokens),
      cacheBaseText,
      pricing.input_cost_per_cached_text_token ??
        pricing.input_cost_per_cached_token ??
        inputTextPrice,
    );
    cost += adjustment(
      value(inputDetails.cached_audio_tokens),
      cacheBaseAudio,
      pricing.input_cost_per_cached_audio_token ??
        pricing.input_cost_per_cached_token ??
        inputAudioPrice,
    );
    cost += adjustment(
      value(inputDetails.cached_image_tokens),
      cacheBaseImage,
      pricing.input_cost_per_cached_image_token ??
        pricing.input_cost_per_cached_token ??
        inputImagePrice,
    );
  } else {
    const hasCachedModalityPrice =
      pricing.input_cost_per_cached_text_token !== undefined ||
      pricing.input_cost_per_cached_audio_token !== undefined ||
      pricing.input_cost_per_cached_image_token !== undefined;
    const cachedModalityPrice = maxPrice(
      pricing.input_cost_per_cached_text_token,
      pricing.input_cost_per_cached_audio_token,
      pricing.input_cost_per_cached_image_token,
    );
    const cachedPrice =
      pricing.input_cost_per_cached_token ??
      (hasCachedModalityPrice ? cachedModalityPrice : inputBasePrice);
    cost += adjustment(
      value(inputDetails?.cached_tokens),
      cacheBaseText,
      cachedPrice,
    );
  }

  cost += adjustment(
    value(inputDetails?.cache_write_tokens),
    cacheBaseText,
    pricing.input_cost_per_cache_write_token ?? inputTextPrice,
  );

  if (options.output_reasoning_tokens_are_additional) {
    cost += value(outputDetails?.reasoning_tokens) * outputTextPrice;
  } else {
    cost += adjustment(
      value(outputDetails?.reasoning_tokens),
      outputBasePrice,
      outputTextPrice,
    );
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
  for (const details of detailsArr) {
    for (const key of MODEL_TOKEN_DETAIL_KEYS) {
      const value = details[key];
      if (value !== undefined) {
        result[key] = (result[key] ?? 0) + value;
      }
    }
  }
  return result;
}

export function mergeModelUsageMax(
  current: ModelUsage | undefined,
  incoming: ModelUsage,
): ModelUsage {
  if (!current) return incoming;

  const mergeDetails = (
    current: ModelTokensDetails | undefined,
    incoming: ModelTokensDetails | undefined,
  ) => {
    if (!current && !incoming) return undefined;
    const result: ModelTokensDetails = {};
    for (const key of MODEL_TOKEN_DETAIL_KEYS) {
      const currentValue = current?.[key];
      const incomingValue = incoming?.[key];
      if (currentValue !== undefined || incomingValue !== undefined) {
        result[key] = Math.max(currentValue ?? 0, incomingValue ?? 0);
      }
    }
    return result;
  };

  const result: ModelUsage = {
    input_tokens: Math.max(current.input_tokens, incoming.input_tokens),
    output_tokens: Math.max(current.output_tokens, incoming.output_tokens),
  };
  const inputDetails = mergeDetails(
    current.input_tokens_details,
    incoming.input_tokens_details,
  );
  if (inputDetails) result.input_tokens_details = inputDetails;
  const outputDetails = mergeDetails(
    current.output_tokens_details,
    incoming.output_tokens_details,
  );
  if (outputDetails) result.output_tokens_details = outputDetails;
  return result;
}
