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

    f64::from(input_text_tokens) * pricing.input_cost_per_text_token.unwrap_or(0.0)
        + f64::from(input_audio_tokens) * pricing.input_cost_per_audio_token.unwrap_or(0.0)
        + f64::from(input_image_tokens) * pricing.input_cost_per_image_token.unwrap_or(0.0)
        + f64::from(input_cached_text_tokens) * pricing.input_cost_per_text_token.unwrap_or(0.0)
        + f64::from(input_cached_audio_tokens) * pricing.input_cost_per_audio_token.unwrap_or(0.0)
        + f64::from(input_cached_image_tokens) * pricing.input_cost_per_image_token.unwrap_or(0.0)
        + f64::from(output_text_tokens) * pricing.output_cost_per_text_token.unwrap_or(0.0)
        + f64::from(output_audio_tokens) * pricing.output_cost_per_audio_token.unwrap_or(0.0)
        + f64::from(output_image_tokens) * pricing.output_cost_per_image_token.unwrap_or(0.0)
}
