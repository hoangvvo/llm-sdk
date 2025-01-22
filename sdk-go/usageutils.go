package llmsdk

// CalculateCost calculates the total cost based on usage statistics and pricing information
func CalculateCost(usage ModelUsage, pricing LanguageModelPricing) float64 {
	// Extract input token counts with fallbacks
	inputTextTokens := usage.InputTokens
	if usage.InputTokensDetails != nil && usage.InputTokensDetails.TextTokens != nil {
		inputTextTokens = *usage.InputTokensDetails.TextTokens
	}

	var inputAudioTokens int
	if usage.InputTokensDetails != nil && usage.InputTokensDetails.AudioTokens != nil {
		inputAudioTokens = *usage.InputTokensDetails.AudioTokens
	}

	var inputImageTokens int
	if usage.InputTokensDetails != nil && usage.InputTokensDetails.ImageTokens != nil {
		inputImageTokens = *usage.InputTokensDetails.ImageTokens
	}

	var inputCachedTextTokens int
	if usage.InputTokensDetails != nil && usage.InputTokensDetails.CachedTextTokens != nil {
		inputCachedTextTokens = *usage.InputTokensDetails.CachedTextTokens
	}

	var inputCachedAudioTokens int
	if usage.InputTokensDetails != nil && usage.InputTokensDetails.CachedAudioTokens != nil {
		inputCachedAudioTokens = *usage.InputTokensDetails.CachedAudioTokens
	}

	var inputCachedImageTokens int
	if usage.InputTokensDetails != nil && usage.InputTokensDetails.CachedImageTokens != nil {
		inputCachedImageTokens = *usage.InputTokensDetails.CachedImageTokens
	}

	// Extract output token counts with fallbacks
	outputTextTokens := usage.OutputTokens
	if usage.OutputTokensDetails != nil && usage.OutputTokensDetails.TextTokens != nil {
		outputTextTokens = *usage.OutputTokensDetails.TextTokens
	}

	var outputAudioTokens int
	if usage.OutputTokensDetails != nil && usage.OutputTokensDetails.AudioTokens != nil {
		outputAudioTokens = *usage.OutputTokensDetails.AudioTokens
	}

	var outputImageTokens int
	if usage.OutputTokensDetails != nil && usage.OutputTokensDetails.ImageTokens != nil {
		outputImageTokens = *usage.OutputTokensDetails.ImageTokens
	}

	// Calculate cost components using pricing with zero fallbacks
	var inputTextCost float64
	if pricing.InputCostPerTextToken != nil {
		inputTextCost = float64(inputTextTokens) * (*pricing.InputCostPerTextToken)
	}

	var inputAudioCost float64
	if pricing.InputCostPerAudioToken != nil {
		inputAudioCost = float64(inputAudioTokens) * (*pricing.InputCostPerAudioToken)
	}

	var inputImageCost float64
	if pricing.InputCostPerImageToken != nil {
		inputImageCost = float64(inputImageTokens) * (*pricing.InputCostPerImageToken)
	}

	var inputCachedTextCost float64
	if pricing.InputCostPerCachedTextToken != nil {
		inputCachedTextCost = float64(inputCachedTextTokens) * (*pricing.InputCostPerCachedTextToken)
	}

	var inputCachedAudioCost float64
	if pricing.InputCostPerCachedAudioToken != nil {
		inputCachedAudioCost = float64(inputCachedAudioTokens) * (*pricing.InputCostPerCachedAudioToken)
	}

	var inputCachedImageCost float64
	if pricing.InputCostPerCachedImageToken != nil {
		inputCachedImageCost = float64(inputCachedImageTokens) * (*pricing.InputCostPerCachedImageToken)
	}

	var outputTextCost float64
	if pricing.OutputCostPerTextToken != nil {
		outputTextCost = float64(outputTextTokens) * (*pricing.OutputCostPerTextToken)
	}

	var outputAudioCost float64
	if pricing.OutputCostPerAudioToken != nil {
		outputAudioCost = float64(outputAudioTokens) * (*pricing.OutputCostPerAudioToken)
	}

	var outputImageCost float64
	if pricing.OutputCostPerImageToken != nil {
		outputImageCost = float64(outputImageTokens) * (*pricing.OutputCostPerImageToken)
	}

	// Sum all costs
	return inputTextCost +
		inputAudioCost +
		inputImageCost +
		inputCachedTextCost +
		inputCachedAudioCost +
		inputCachedImageCost +
		outputTextCost +
		outputAudioCost +
		outputImageCost
}