import type {
  LanguageModelPricing,
  ModelServerToolUsage,
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
  "extended_cache_write_tokens",
  "reasoning_tokens",
] as const;

const SERVER_TOOL_USAGE_KEYS = ["web_search_requests"] as const;

/**
 * Counting conventions needed to price unmodified provider usage.
 */
export interface ModelUsageCostOptions {
  /**
   * True when `input_tokens` excludes cache reads and writes.
   */
  input_cache_tokens_are_additional: boolean;
  /**
   * True when `output_tokens` excludes reasoning tokens.
   */
  output_reasoning_tokens_are_additional: boolean;
}

/** Estimates USD charges using the provider's counting conventions. */
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

  let inputCost = inputTokens * inputBasePrice;
  let outputCost = outputTokens * outputBasePrice;

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

  inputCost += adjustment(
    value(inputDetails?.audio_tokens),
    inputBasePrice,
    inputAudioPrice,
  );
  inputCost += adjustment(
    value(inputDetails?.image_tokens),
    inputBasePrice,
    inputImagePrice,
  );
  outputCost += adjustment(
    value(outputDetails?.audio_tokens),
    outputBasePrice,
    outputAudioPrice,
  );
  outputCost += adjustment(
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
    inputCost += adjustment(
      value(inputDetails.cached_text_tokens),
      cacheBaseText,
      pricing.input_cost_per_cached_text_token ??
        pricing.input_cost_per_cached_token ??
        inputTextPrice,
    );
    inputCost += adjustment(
      value(inputDetails.cached_audio_tokens),
      cacheBaseAudio,
      pricing.input_cost_per_cached_audio_token ??
        pricing.input_cost_per_cached_token ??
        inputAudioPrice,
    );
    inputCost += adjustment(
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
    inputCost += adjustment(
      value(inputDetails?.cached_tokens),
      cacheBaseText,
      cachedPrice,
    );
  }

  const cacheWritePrice =
    pricing.input_cost_per_cache_write_token ?? inputTextPrice;
  inputCost += adjustment(
    value(inputDetails?.cache_write_tokens),
    cacheBaseText,
    cacheWritePrice,
  );
  // Extended-retention cache writes are a subset of the cache writes charged above.
  inputCost += adjustment(
    value(inputDetails?.extended_cache_write_tokens),
    cacheWritePrice,
    pricing.input_cost_per_extended_cache_write_token ?? cacheWritePrice,
  );

  if (options.output_reasoning_tokens_are_additional) {
    outputCost += value(outputDetails?.reasoning_tokens) * outputTextPrice;
  } else {
    outputCost += adjustment(
      value(outputDetails?.reasoning_tokens),
      outputBasePrice,
      outputTextPrice,
    );
  }

  const longContext = pricing.long_context;
  if (longContext && inputTokens > longContext.threshold_tokens) {
    inputCost *= longContext.input_cost_multiplier ?? 1;
    outputCost *= longContext.output_cost_multiplier ?? 1;
  }

  return (
    inputCost +
    outputCost +
    value(usage.server_tool_use?.web_search_requests) *
      value(pricing.cost_per_web_search_request)
  );
}

export function sumModelUsage(usages: ModelUsage[]): ModelUsage {
  const result: ModelUsage = { input_tokens: 0, output_tokens: 0 };
  const inputDetails: ModelTokensDetails[] = [];
  const outputDetails: ModelTokensDetails[] = [];
  const serverToolUsages: ModelServerToolUsage[] = [];
  for (const usage of usages) {
    result.input_tokens += usage.input_tokens;
    result.output_tokens += usage.output_tokens;
    if (usage.input_tokens_details) {
      inputDetails.push(usage.input_tokens_details);
    }
    if (usage.output_tokens_details) {
      outputDetails.push(usage.output_tokens_details);
    }
    if (usage.server_tool_use) {
      serverToolUsages.push(usage.server_tool_use);
    }
  }
  if (inputDetails.length > 0) {
    result.input_tokens_details = sumModelTokensDetails(inputDetails);
  }
  if (outputDetails.length > 0) {
    result.output_tokens_details = sumModelTokensDetails(outputDetails);
  }
  if (serverToolUsages.length > 0) {
    result.server_tool_use = sumModelServerToolUsage(serverToolUsages);
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

export function sumModelServerToolUsage(
  usages: ModelServerToolUsage[],
): ModelServerToolUsage {
  const result: ModelServerToolUsage = {};
  for (const usage of usages) {
    for (const key of SERVER_TOOL_USAGE_KEYS) {
      const value = usage[key];
      if (value !== undefined) {
        result[key] = (result[key] ?? 0) + value;
      }
    }
  }
  return result;
}

/**
 * Merges cumulative usage snapshots of the same request, such as the usage
 * reported by successive stream chunks, keeping the highest known counts.
 */
export function mergeModelUsageMax(
  current: ModelUsage | undefined,
  incoming: ModelUsage,
): ModelUsage {
  if (!current) return incoming;

  const mergeCounts = <K extends string>(
    current: Partial<Record<K, number>> | undefined,
    incoming: Partial<Record<K, number>> | undefined,
    keys: readonly K[],
  ): Partial<Record<K, number>> | undefined => {
    if (!current && !incoming) return undefined;
    const result: Partial<Record<K, number>> = {};
    for (const key of keys) {
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
  const inputDetails = mergeCounts(
    current.input_tokens_details,
    incoming.input_tokens_details,
    MODEL_TOKEN_DETAIL_KEYS,
  );
  if (inputDetails) result.input_tokens_details = inputDetails;
  const outputDetails = mergeCounts(
    current.output_tokens_details,
    incoming.output_tokens_details,
    MODEL_TOKEN_DETAIL_KEYS,
  );
  if (outputDetails) result.output_tokens_details = outputDetails;
  const serverToolUse = mergeCounts(
    current.server_tool_use,
    incoming.server_tool_use,
    SERVER_TOOL_USAGE_KEYS,
  );
  if (serverToolUse) result.server_tool_use = serverToolUse;
  return result;
}
