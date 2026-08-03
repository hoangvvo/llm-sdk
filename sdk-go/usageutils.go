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
		if value == nil {
			return 0
		}
		return *value
	}
	price := func(value *float64) float64 {
		if value == nil {
			return 0
		}
		return *value
	}

	inputDetails := usage.InputTokensDetails
	outputDetails := usage.OutputTokensDetails
	cost := float64(usage.InputTokens)*price(pricing.InputCostPerTextToken) +
		float64(usage.OutputTokens)*price(pricing.OutputCostPerTextToken)

	adjustment := func(tokens int, regularPrice, categoryPrice *float64) float64 {
		if categoryPrice == nil {
			return 0
		}
		return float64(tokens) * (*categoryPrice - price(regularPrice))
	}

	if inputDetails != nil {
		cost += adjustment(value(inputDetails.AudioTokens), pricing.InputCostPerTextToken, pricing.InputCostPerAudioToken)
		cost += adjustment(value(inputDetails.ImageTokens), pricing.InputCostPerTextToken, pricing.InputCostPerImageToken)
	}
	if outputDetails != nil {
		cost += adjustment(value(outputDetails.AudioTokens), pricing.OutputCostPerTextToken, pricing.OutputCostPerAudioToken)
		cost += adjustment(value(outputDetails.ImageTokens), pricing.OutputCostPerTextToken, pricing.OutputCostPerImageToken)
	}

	if inputDetails != nil {
		var cacheBaseText, cacheBaseAudio, cacheBaseImage *float64
		if !options.InputCacheTokensAreAdditional {
			cacheBaseText = pricing.InputCostPerTextToken
			cacheBaseAudio = pricing.InputCostPerAudioToken
			cacheBaseImage = pricing.InputCostPerImageToken
		}
		hasCachedModalities := inputDetails.CachedTextTokens != nil || inputDetails.CachedAudioTokens != nil || inputDetails.CachedImageTokens != nil
		hasCachedModalityPricing := pricing.InputCostPerCachedTextToken != nil || pricing.InputCostPerCachedAudioToken != nil || pricing.InputCostPerCachedImageToken != nil
		if hasCachedModalities && hasCachedModalityPricing {
			cost += adjustment(value(inputDetails.CachedTextTokens), cacheBaseText, pricing.InputCostPerCachedTextToken)
			cost += adjustment(value(inputDetails.CachedAudioTokens), cacheBaseAudio, pricing.InputCostPerCachedAudioToken)
			cost += adjustment(value(inputDetails.CachedImageTokens), cacheBaseImage, pricing.InputCostPerCachedImageToken)
		} else {
			cost += adjustment(value(inputDetails.CachedTokens), cacheBaseText, pricing.InputCostPerCachedToken)
		}
		cost += adjustment(value(inputDetails.CacheWriteTokens), cacheBaseText, pricing.InputCostPerCacheWriteToken)
	}
	if options.OutputReasoningTokensAreAdditional && outputDetails != nil {
		cost += float64(value(outputDetails.ReasoningTokens)) * price(pricing.OutputCostPerTextToken)
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
