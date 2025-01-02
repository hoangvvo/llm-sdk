use crate::{LanguageModelPricing, ModelUsage};

pub fn calculate_cost(usage: &ModelUsage, pricing: &LanguageModelPricing) -> f64 {
    let input_text_tokens = usage
        .input_tokens_details
        .as_ref()
        .and_then(|details| details.text_tokens)
        .unwrap_or(usage.input_tokens);
    let input_audio_tokens = usage
        .input_tokens_details
        .as_ref()
        .and_then(|details| details.audio_tokens)
        .unwrap_or(0);
    let input_image_tokens = usage
        .input_tokens_details
        .as_ref()
        .and_then(|details| details.image_tokens)
        .unwrap_or(0);
    let input_cached_text_tokens = usage
        .input_tokens_details
        .as_ref()
        .and_then(|details| details.cached_text_tokens)
        .unwrap_or(0);
    let input_cached_audio_tokens = usage
        .input_tokens_details
        .as_ref()
        .and_then(|details| details.cached_audio_tokens)
        .unwrap_or(0);
    let input_cached_image_tokens = usage
        .input_tokens_details
        .as_ref()
        .and_then(|details| details.cached_image_tokens)
        .unwrap_or(0);

    let output_text_tokens = usage
        .output_tokens_details
        .as_ref()
        .and_then(|details| details.text_tokens)
        .unwrap_or(usage.output_tokens);
    let output_audio_tokens = usage
        .output_tokens_details
        .as_ref()
        .and_then(|details| details.audio_tokens)
        .unwrap_or(0);
    let output_image_tokens = usage
        .output_tokens_details
        .as_ref()
        .and_then(|details| details.image_tokens)
        .unwrap_or(0);

    return input_text_tokens as f64 * pricing.input_cost_per_text_token.unwrap_or(0.0)
        + input_audio_tokens as f64 * pricing.input_cost_per_audio_token.unwrap_or(0.0)
        + input_image_tokens as f64 * pricing.input_cost_per_image_token.unwrap_or(0.0)
        + input_cached_text_tokens as f64 * pricing.input_cost_per_text_token.unwrap_or(0.0)
        + input_cached_audio_tokens as f64 * pricing.input_cost_per_audio_token.unwrap_or(0.0)
        + input_cached_image_tokens as f64 * pricing.input_cost_per_image_token.unwrap_or(0.0)
        + output_text_tokens as f64 * pricing.output_cost_per_text_token.unwrap_or(0.0)
        + output_audio_tokens as f64 * pricing.output_cost_per_audio_token.unwrap_or(0.0)
        + output_image_tokens as f64 * pricing.output_cost_per_image_token.unwrap_or(0.0);
}

// import type { LanguageModelPricing, ModelUsage } from "../types.js";

// export function calculateCost(
//   usage: ModelUsage,
//   pricing: LanguageModelPricing,
// ) {
//   let cost = 0;
//   const inputTextTokens =
//     usage.input_tokens_details?.text_tokens ?? usage.input_tokens;
//   const inputAudioTokens = usage.input_tokens_details?.audio_tokens ?? 0;
//   const inputImageTokens = usage.input_tokens_details?.image_tokens ?? 0;
//   const inputCachedTextTokens =
//     usage.input_tokens_details?.cached_text_tokens ?? 0;
//   const inputCachedAudioTokens =
//     usage.input_tokens_details?.cached_audio_tokens ?? 0;
//   const inputCachedImageTokens =
//     usage.input_tokens_details?.cached_image_tokens ?? 0;

//   const outputTextTokens =
//     usage.output_tokens_details?.text_tokens ?? usage.output_tokens;
//   const outputAudioTokens = usage.output_tokens_details?.audio_tokens ?? 0;
//   const outputImageTokens = usage.output_tokens_details?.image_tokens ?? 0;

//   cost +=
//     inputTextTokens * (pricing.input_cost_per_text_token ?? 0) +
//     inputAudioTokens * (pricing.input_cost_per_audio_token ?? 0) +
//     inputImageTokens * (pricing.input_cost_per_image_token ?? 0) +
//     inputCachedTextTokens * (pricing.input_cost_per_text_token ?? 0) +
//     inputCachedAudioTokens * (pricing.input_cost_per_audio_token ?? 0) +
//     inputCachedImageTokens * (pricing.input_cost_per_image_token ?? 0) +
//     outputTextTokens * (pricing.output_cost_per_text_token ?? 0) +
//     outputAudioTokens * (pricing.output_cost_per_audio_token ?? 0) +
//     outputImageTokens * (pricing.output_cost_per_image_token ?? 0);

//   return cost;
// }
