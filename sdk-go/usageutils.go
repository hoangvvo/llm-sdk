package llmsdk

import "github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"

// SumModelTokensDetails sums multiple ModelTokensDetails into one
func SumModelTokensDetails(detailsList []ModelTokensDetails) *ModelTokensDetails {
	if len(detailsList) == 0 {
		return nil
	}

	result := &ModelTokensDetails{}

	for _, details := range detailsList {
		if details.TextTokens != nil {
			if result.TextTokens == nil {
				result.TextTokens = ptr.To(0)
			}
			*result.TextTokens += *details.TextTokens
		}
		if details.CachedTextTokens != nil {
			if result.CachedTextTokens == nil {
				result.CachedTextTokens = ptr.To(0)
			}
			*result.CachedTextTokens += *details.CachedTextTokens
		}
		if details.AudioTokens != nil {
			if result.AudioTokens == nil {
				result.AudioTokens = ptr.To(0)
			}
			*result.AudioTokens += *details.AudioTokens
		}
		if details.CachedAudioTokens != nil {
			if result.CachedAudioTokens == nil {
				result.CachedAudioTokens = ptr.To(0)
			}
			*result.CachedAudioTokens += *details.CachedAudioTokens
		}
		if details.ImageTokens != nil {
			if result.ImageTokens == nil {
				result.ImageTokens = ptr.To(0)
			}
			*result.ImageTokens += *details.ImageTokens
		}
		if details.CachedImageTokens != nil {
			if result.CachedImageTokens == nil {
				result.CachedImageTokens = ptr.To(0)
			}
			*result.CachedImageTokens += *details.CachedImageTokens
		}
		if details.CachedTokens != nil {
			if result.CachedTokens == nil {
				result.CachedTokens = ptr.To(0)
			}
			*result.CachedTokens += *details.CachedTokens
		}
		if details.CacheWriteTokens != nil {
			if result.CacheWriteTokens == nil {
				result.CacheWriteTokens = ptr.To(0)
			}
			*result.CacheWriteTokens += *details.CacheWriteTokens
		}
		if details.ExtendedCacheWriteTokens != nil {
			if result.ExtendedCacheWriteTokens == nil {
				result.ExtendedCacheWriteTokens = ptr.To(0)
			}
			*result.ExtendedCacheWriteTokens += *details.ExtendedCacheWriteTokens
		}
		if details.ReasoningTokens != nil {
			if result.ReasoningTokens == nil {
				result.ReasoningTokens = ptr.To(0)
			}
			*result.ReasoningTokens += *details.ReasoningTokens
		}
	}

	return result
}

// SumModelServerToolUsage sums multiple ModelServerToolUsage into one
func SumModelServerToolUsage(usages []ModelServerToolUsage) *ModelServerToolUsage {
	if len(usages) == 0 {
		return nil
	}

	result := &ModelServerToolUsage{}

	for _, usage := range usages {
		if usage.WebSearchRequests != nil {
			if result.WebSearchRequests == nil {
				result.WebSearchRequests = ptr.To(0)
			}
			*result.WebSearchRequests += *usage.WebSearchRequests
		}
	}

	return result
}

// ModelUsageCostOptions specifies counting conventions for unmodified provider usage.
type ModelUsageCostOptions struct {
	// True when InputTokens excludes cache reads and writes.
	InputCacheTokensAreAdditional bool
	// True when OutputTokens excludes reasoning tokens.
	OutputReasoningTokensAreAdditional bool
}

// CalculateCost estimates USD charges using the provider's counting conventions.
func (usage *ModelUsage) CalculateCost(pricing *LanguageModelPricing, options ModelUsageCostOptions) float64 {
	if pricing == nil {
		return 0
	}

	value := func(value *int) int {
		if value == nil || *value < 0 {
			return 0
		}
		return *value
	}
	maxInt := func(values ...int) int {
		result := 0
		for _, value := range values {
			if value > result {
				result = value
			}
		}
		return result
	}
	maxPrice := func(values ...*float64) float64 {
		result := 0.0
		for _, value := range values {
			if value != nil && *value > result {
				result = *value
			}
		}
		return result
	}

	inputDetails := usage.InputTokensDetails
	outputDetails := usage.OutputTokensDetails
	hasModalityBreakdown := func(details *ModelTokensDetails) bool {
		return details != nil && (details.TextTokens != nil || details.AudioTokens != nil || details.ImageTokens != nil)
	}
	modalityTotal := func(details *ModelTokensDetails) int {
		if details == nil {
			return 0
		}
		return value(details.TextTokens) + value(details.AudioTokens) + value(details.ImageTokens)
	}

	inputHasModalityBreakdown := hasModalityBreakdown(inputDetails)
	inputMaxPrice := maxPrice(pricing.InputCostPerTextToken, pricing.InputCostPerAudioToken, pricing.InputCostPerImageToken)
	outputMaxPrice := maxPrice(pricing.OutputCostPerTextToken, pricing.OutputCostPerAudioToken, pricing.OutputCostPerImageToken)
	// Unattributed tokens are assumed to be text. The highest configured rate is
	// only a fallback for models, such as TTS, that do not define a text rate.
	inputBasePrice := inputMaxPrice
	if pricing.InputCostPerTextToken != nil {
		inputBasePrice = *pricing.InputCostPerTextToken
	}
	outputBasePrice := outputMaxPrice
	if pricing.OutputCostPerTextToken != nil {
		outputBasePrice = *pricing.OutputCostPerTextToken
	}

	hasCachedModalities := inputDetails != nil && (inputDetails.CachedTextTokens != nil || inputDetails.CachedAudioTokens != nil || inputDetails.CachedImageTokens != nil)
	cachedReadTokens := 0
	if inputDetails != nil {
		if hasCachedModalities {
			cachedReadTokens = value(inputDetails.CachedTextTokens) + value(inputDetails.CachedAudioTokens) + value(inputDetails.CachedImageTokens)
		} else {
			cachedReadTokens = value(inputDetails.CachedTokens)
		}
	}
	includedCacheTokens := 0
	if !options.InputCacheTokensAreAdditional {
		includedCacheTokens = cachedReadTokens
		if inputDetails != nil {
			includedCacheTokens += value(inputDetails.CacheWriteTokens)
		}
	}
	inputTokens := maxInt(usage.InputTokens, modalityTotal(inputDetails), includedCacheTokens)
	outputDetailTokens := modalityTotal(outputDetails)
	if !options.OutputReasoningTokensAreAdditional && outputDetails != nil {
		outputDetailTokens += value(outputDetails.ReasoningTokens)
	}
	outputTokens := maxInt(usage.OutputTokens, outputDetailTokens)

	// Input and output are tracked separately so long-context multipliers can
	// apply to each side.
	inputCost := float64(inputTokens) * inputBasePrice
	outputCost := float64(outputTokens) * outputBasePrice
	adjustment := func(tokens int, regularPrice, categoryPrice float64) float64 {
		return float64(tokens) * (categoryPrice - regularPrice)
	}

	inputTextPrice := inputBasePrice
	if pricing.InputCostPerTextToken != nil {
		inputTextPrice = *pricing.InputCostPerTextToken
	}
	inputAudioPrice := inputTextPrice
	if pricing.InputCostPerAudioToken != nil {
		inputAudioPrice = *pricing.InputCostPerAudioToken
	}
	inputImagePrice := inputTextPrice
	if pricing.InputCostPerImageToken != nil {
		inputImagePrice = *pricing.InputCostPerImageToken
	}
	outputTextPrice := outputBasePrice
	if pricing.OutputCostPerTextToken != nil {
		outputTextPrice = *pricing.OutputCostPerTextToken
	}
	outputAudioPrice := outputTextPrice
	if pricing.OutputCostPerAudioToken != nil {
		outputAudioPrice = *pricing.OutputCostPerAudioToken
	}
	outputImagePrice := outputTextPrice
	if pricing.OutputCostPerImageToken != nil {
		outputImagePrice = *pricing.OutputCostPerImageToken
	}

	if inputDetails != nil {
		inputCost += adjustment(value(inputDetails.AudioTokens), inputBasePrice, inputAudioPrice)
		inputCost += adjustment(value(inputDetails.ImageTokens), inputBasePrice, inputImagePrice)
	}
	if outputDetails != nil {
		outputCost += adjustment(value(outputDetails.AudioTokens), outputBasePrice, outputAudioPrice)
		outputCost += adjustment(value(outputDetails.ImageTokens), outputBasePrice, outputImagePrice)
	}

	cacheBaseText, cacheBaseAudio, cacheBaseImage := 0.0, 0.0, 0.0
	if !options.InputCacheTokensAreAdditional {
		cacheBaseText, cacheBaseAudio, cacheBaseImage = inputBasePrice, inputBasePrice, inputBasePrice
		if inputHasModalityBreakdown {
			cacheBaseText, cacheBaseAudio, cacheBaseImage = inputTextPrice, inputAudioPrice, inputImagePrice
		}
	}
	if inputDetails != nil {
		if hasCachedModalities {
			cachedTextPrice := inputTextPrice
			if pricing.InputCostPerCachedToken != nil {
				cachedTextPrice = *pricing.InputCostPerCachedToken
			}
			if pricing.InputCostPerCachedTextToken != nil {
				cachedTextPrice = *pricing.InputCostPerCachedTextToken
			}
			cachedAudioPrice := inputAudioPrice
			if pricing.InputCostPerCachedToken != nil {
				cachedAudioPrice = *pricing.InputCostPerCachedToken
			}
			if pricing.InputCostPerCachedAudioToken != nil {
				cachedAudioPrice = *pricing.InputCostPerCachedAudioToken
			}
			cachedImagePrice := inputImagePrice
			if pricing.InputCostPerCachedToken != nil {
				cachedImagePrice = *pricing.InputCostPerCachedToken
			}
			if pricing.InputCostPerCachedImageToken != nil {
				cachedImagePrice = *pricing.InputCostPerCachedImageToken
			}
			inputCost += adjustment(value(inputDetails.CachedTextTokens), cacheBaseText, cachedTextPrice)
			inputCost += adjustment(value(inputDetails.CachedAudioTokens), cacheBaseAudio, cachedAudioPrice)
			inputCost += adjustment(value(inputDetails.CachedImageTokens), cacheBaseImage, cachedImagePrice)
		} else {
			cachedPrice := inputBasePrice
			if pricing.InputCostPerCachedToken != nil {
				cachedPrice = *pricing.InputCostPerCachedToken
			} else if pricing.InputCostPerCachedTextToken != nil || pricing.InputCostPerCachedAudioToken != nil || pricing.InputCostPerCachedImageToken != nil {
				cachedPrice = maxPrice(pricing.InputCostPerCachedTextToken, pricing.InputCostPerCachedAudioToken, pricing.InputCostPerCachedImageToken)
			}
			inputCost += adjustment(value(inputDetails.CachedTokens), cacheBaseText, cachedPrice)
		}

		cacheWritePrice := inputTextPrice
		if pricing.InputCostPerCacheWriteToken != nil {
			cacheWritePrice = *pricing.InputCostPerCacheWriteToken
		}
		inputCost += adjustment(value(inputDetails.CacheWriteTokens), cacheBaseText, cacheWritePrice)

		// One-hour cache writes are a subset of the cache writes, billed at
		// their own rate when the pricing defines one.
		extendedCacheWritePrice := cacheWritePrice
		if pricing.InputCostPerExtendedCacheWriteToken != nil {
			extendedCacheWritePrice = *pricing.InputCostPerExtendedCacheWriteToken
		}
		inputCost += adjustment(value(inputDetails.ExtendedCacheWriteTokens), cacheWritePrice, extendedCacheWritePrice)
	}

	if outputDetails != nil {
		if options.OutputReasoningTokensAreAdditional {
			outputCost += float64(value(outputDetails.ReasoningTokens)) * outputTextPrice
		} else {
			outputCost += adjustment(value(outputDetails.ReasoningTokens), outputBasePrice, outputTextPrice)
		}
	}

	if longContext := pricing.LongContext; longContext != nil && inputTokens > longContext.ThresholdTokens {
		if longContext.InputCostMultiplier != nil {
			inputCost *= *longContext.InputCostMultiplier
		}
		if longContext.OutputCostMultiplier != nil {
			outputCost *= *longContext.OutputCostMultiplier
		}
	}

	cost := inputCost + outputCost
	if usage.ServerToolUse != nil && pricing.CostPerWebSearchRequest != nil {
		cost += float64(value(usage.ServerToolUse.WebSearchRequests)) * *pricing.CostPerWebSearchRequest
	}

	return cost
}

func (u *ModelUsage) Add(other *ModelUsage) *ModelUsage {
	if u == nil {
		return other
	}
	if other == nil {
		return u
	}

	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens

	tokenDetails := []ModelTokensDetails{}
	if u.InputTokensDetails != nil {
		tokenDetails = append(tokenDetails, *u.InputTokensDetails)
	}
	if other.InputTokensDetails != nil {
		tokenDetails = append(tokenDetails, *other.InputTokensDetails)
	}
	u.InputTokensDetails = SumModelTokensDetails(tokenDetails)

	tokenDetails = []ModelTokensDetails{}
	if u.OutputTokensDetails != nil {
		tokenDetails = append(tokenDetails, *u.OutputTokensDetails)
	}
	if other.OutputTokensDetails != nil {
		tokenDetails = append(tokenDetails, *other.OutputTokensDetails)
	}
	u.OutputTokensDetails = SumModelTokensDetails(tokenDetails)

	serverToolUsages := []ModelServerToolUsage{}
	if u.ServerToolUse != nil {
		serverToolUsages = append(serverToolUsages, *u.ServerToolUse)
	}
	if other.ServerToolUse != nil {
		serverToolUsages = append(serverToolUsages, *other.ServerToolUse)
	}
	u.ServerToolUse = SumModelServerToolUsage(serverToolUsages)

	return u
}

// MergeModelUsageMax merges cumulative usage snapshots of the same request,
// such as the usage reported by successive stream chunks, keeping the highest
// known counts. The merged result is written into current when it is non-nil.
func MergeModelUsageMax(current, incoming *ModelUsage) *ModelUsage {
	if current == nil {
		return incoming
	}
	if incoming == nil {
		return current
	}
	current.InputTokens = max(current.InputTokens, incoming.InputTokens)
	current.OutputTokens = max(current.OutputTokens, incoming.OutputTokens)
	current.InputTokensDetails = mergeModelTokensDetailsMax(current.InputTokensDetails, incoming.InputTokensDetails)
	current.OutputTokensDetails = mergeModelTokensDetailsMax(current.OutputTokensDetails, incoming.OutputTokensDetails)
	if incoming.ServerToolUse != nil {
		if current.ServerToolUse == nil {
			current.ServerToolUse = &ModelServerToolUsage{}
		}
		mergeCountMax(&current.ServerToolUse.WebSearchRequests, incoming.ServerToolUse.WebSearchRequests)
	}
	return current
}

func mergeCountMax(target **int, value *int) {
	if value == nil {
		return
	}
	if *target == nil {
		*target = ptr.To(*value)
	} else {
		**target = max(**target, *value)
	}
}

func mergeModelTokensDetailsMax(current, incoming *ModelTokensDetails) *ModelTokensDetails {
	if current == nil {
		return incoming
	}
	if incoming == nil {
		return current
	}
	mergeCountMax(&current.TextTokens, incoming.TextTokens)
	mergeCountMax(&current.AudioTokens, incoming.AudioTokens)
	mergeCountMax(&current.ImageTokens, incoming.ImageTokens)
	mergeCountMax(&current.CachedTextTokens, incoming.CachedTextTokens)
	mergeCountMax(&current.CachedAudioTokens, incoming.CachedAudioTokens)
	mergeCountMax(&current.CachedImageTokens, incoming.CachedImageTokens)
	mergeCountMax(&current.CachedTokens, incoming.CachedTokens)
	mergeCountMax(&current.CacheWriteTokens, incoming.CacheWriteTokens)
	mergeCountMax(&current.ExtendedCacheWriteTokens, incoming.ExtendedCacheWriteTokens)
	mergeCountMax(&current.ReasoningTokens, incoming.ReasoningTokens)
	return current
}
