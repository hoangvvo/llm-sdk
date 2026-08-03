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
		if details.ReasoningTokens != nil {
			if result.ReasoningTokens == nil {
				result.ReasoningTokens = ptr.To(0)
			}
			*result.ReasoningTokens += *details.ReasoningTokens
		}
	}

	return result
}

type ModelUsageCostOptions struct {
	InputCacheTokensAreAdditional      bool
	OutputReasoningTokensAreAdditional bool
}

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

	cost := float64(inputTokens)*inputBasePrice + float64(outputTokens)*outputBasePrice
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
		cost += adjustment(value(inputDetails.AudioTokens), inputBasePrice, inputAudioPrice)
		cost += adjustment(value(inputDetails.ImageTokens), inputBasePrice, inputImagePrice)
	}
	if outputDetails != nil {
		cost += adjustment(value(outputDetails.AudioTokens), outputBasePrice, outputAudioPrice)
		cost += adjustment(value(outputDetails.ImageTokens), outputBasePrice, outputImagePrice)
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
			cost += adjustment(value(inputDetails.CachedTextTokens), cacheBaseText, cachedTextPrice)
			cost += adjustment(value(inputDetails.CachedAudioTokens), cacheBaseAudio, cachedAudioPrice)
			cost += adjustment(value(inputDetails.CachedImageTokens), cacheBaseImage, cachedImagePrice)
		} else {
			cachedPrice := inputBasePrice
			if pricing.InputCostPerCachedToken != nil {
				cachedPrice = *pricing.InputCostPerCachedToken
			} else if pricing.InputCostPerCachedTextToken != nil || pricing.InputCostPerCachedAudioToken != nil || pricing.InputCostPerCachedImageToken != nil {
				cachedPrice = maxPrice(pricing.InputCostPerCachedTextToken, pricing.InputCostPerCachedAudioToken, pricing.InputCostPerCachedImageToken)
			}
			cost += adjustment(value(inputDetails.CachedTokens), cacheBaseText, cachedPrice)
		}

		cacheWritePrice := inputTextPrice
		if pricing.InputCostPerCacheWriteToken != nil {
			cacheWritePrice = *pricing.InputCostPerCacheWriteToken
		}
		cost += adjustment(value(inputDetails.CacheWriteTokens), cacheBaseText, cacheWritePrice)
	}

	if outputDetails != nil {
		if options.OutputReasoningTokensAreAdditional {
			cost += float64(value(outputDetails.ReasoningTokens)) * outputTextPrice
		} else {
			cost += adjustment(value(outputDetails.ReasoningTokens), outputBasePrice, outputTextPrice)
		}
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

	return u
}
