use crate::{LanguageModelPricing, ModelUsage};

pub struct ModelUsageCostOptions {
    pub input_cache_tokens_are_additional: bool,
    pub output_reasoning_tokens_are_additional: bool,
}

impl ModelUsage {
    #[must_use]
    pub fn calculate_cost(
        &self,
        pricing: &LanguageModelPricing,
        options: &ModelUsageCostOptions,
    ) -> f64 {
        let mut cost = f64::from(self.input_tokens)
            * pricing.input_cost_per_text_token.unwrap_or(0.0)
            + f64::from(self.output_tokens) * pricing.output_cost_per_text_token.unwrap_or(0.0);

        let adjustment = |tokens: u32, regular_price: Option<f64>, category_price: Option<f64>| {
            category_price.map_or(0.0, |category_price| {
                f64::from(tokens) * (category_price - regular_price.unwrap_or(0.0))
            })
        };

        if let Some(details) = &self.input_tokens_details {
            cost += adjustment(
                details.audio_tokens.unwrap_or(0),
                pricing.input_cost_per_text_token,
                pricing.input_cost_per_audio_token,
            );
            cost += adjustment(
                details.image_tokens.unwrap_or(0),
                pricing.input_cost_per_text_token,
                pricing.input_cost_per_image_token,
            );
        }
        if let Some(details) = &self.output_tokens_details {
            cost += adjustment(
                details.audio_tokens.unwrap_or(0),
                pricing.output_cost_per_text_token,
                pricing.output_cost_per_audio_token,
            );
            cost += adjustment(
                details.image_tokens.unwrap_or(0),
                pricing.output_cost_per_text_token,
                pricing.output_cost_per_image_token,
            );
        }

        if let Some(details) = &self.input_tokens_details {
            let has_cached_modalities = details.cached_text_tokens.is_some()
                || details.cached_audio_tokens.is_some()
                || details.cached_image_tokens.is_some();
            let has_cached_modality_pricing = pricing.input_cost_per_cached_text_token.is_some()
                || pricing.input_cost_per_cached_audio_token.is_some()
                || pricing.input_cost_per_cached_image_token.is_some();
            let (cache_base_text, cache_base_audio, cache_base_image) =
                if options.input_cache_tokens_are_additional {
                    (None, None, None)
                } else {
                    (
                        pricing.input_cost_per_text_token,
                        pricing.input_cost_per_audio_token,
                        pricing.input_cost_per_image_token,
                    )
                };
            if has_cached_modalities && has_cached_modality_pricing {
                cost += adjustment(
                    details.cached_text_tokens.unwrap_or(0),
                    cache_base_text,
                    pricing.input_cost_per_cached_text_token,
                );
                cost += adjustment(
                    details.cached_audio_tokens.unwrap_or(0),
                    cache_base_audio,
                    pricing.input_cost_per_cached_audio_token,
                );
                cost += adjustment(
                    details.cached_image_tokens.unwrap_or(0),
                    cache_base_image,
                    pricing.input_cost_per_cached_image_token,
                );
            } else {
                cost += adjustment(
                    details.cached_tokens.unwrap_or(0),
                    cache_base_text,
                    pricing.input_cost_per_cached_token,
                );
            }
            cost += adjustment(
                details.cache_write_tokens.unwrap_or(0),
                cache_base_text,
                pricing.input_cost_per_cache_write_token,
            );
        }

        if options.output_reasoning_tokens_are_additional {
            cost += f64::from(
                self.output_tokens_details
                    .as_ref()
                    .and_then(|details| details.reasoning_tokens)
                    .unwrap_or(0),
            ) * pricing.output_cost_per_text_token.unwrap_or(0.0);
        }

        cost
    }

    pub fn add(&mut self, other: &Self) {
        self.input_tokens += other.input_tokens;
        self.output_tokens += other.output_tokens;

        if let Some(other_input_details) = &other.input_tokens_details {
            let self_input_details = self.input_tokens_details.get_or_insert_default();
            if let Some(text_tokens) = other_input_details.text_tokens {
                self_input_details.text_tokens =
                    Some(self_input_details.text_tokens.unwrap_or(0) + text_tokens);
            }
            if let Some(audio_tokens) = other_input_details.audio_tokens {
                self_input_details.audio_tokens =
                    Some(self_input_details.audio_tokens.unwrap_or(0) + audio_tokens);
            }
            if let Some(image_tokens) = other_input_details.image_tokens {
                self_input_details.image_tokens =
                    Some(self_input_details.image_tokens.unwrap_or(0) + image_tokens);
            }
            if let Some(cached_text_tokens) = other_input_details.cached_text_tokens {
                self_input_details.cached_text_tokens =
                    Some(self_input_details.cached_text_tokens.unwrap_or(0) + cached_text_tokens);
            }
            if let Some(cached_audio_tokens) = other_input_details.cached_audio_tokens {
                self_input_details.cached_audio_tokens =
                    Some(self_input_details.cached_audio_tokens.unwrap_or(0) + cached_audio_tokens);
            }
            if let Some(cached_image_tokens) = other_input_details.cached_image_tokens {
                self_input_details.cached_image_tokens =
                    Some(self_input_details.cached_image_tokens.unwrap_or(0) + cached_image_tokens);
            }
            if let Some(cached_tokens) = other_input_details.cached_tokens {
                self_input_details.cached_tokens =
                    Some(self_input_details.cached_tokens.unwrap_or(0) + cached_tokens);
            }
            if let Some(cache_write_tokens) = other_input_details.cache_write_tokens {
                self_input_details.cache_write_tokens =
                    Some(self_input_details.cache_write_tokens.unwrap_or(0) + cache_write_tokens);
            }
            if let Some(reasoning_tokens) = other_input_details.reasoning_tokens {
                self_input_details.reasoning_tokens =
                    Some(self_input_details.reasoning_tokens.unwrap_or(0) + reasoning_tokens);
            }
        }

        if let Some(other_output_details) = &other.output_tokens_details {
            let self_output_details = self.output_tokens_details.get_or_insert_default();
            if let Some(text_tokens) = other_output_details.text_tokens {
                self_output_details.text_tokens =
                    Some(self_output_details.text_tokens.unwrap_or(0) + text_tokens);
            }
            if let Some(audio_tokens) = other_output_details.audio_tokens {
                self_output_details.audio_tokens =
                    Some(self_output_details.audio_tokens.unwrap_or(0) + audio_tokens);
            }
            if let Some(image_tokens) = other_output_details.image_tokens {
                self_output_details.image_tokens =
                    Some(self_output_details.image_tokens.unwrap_or(0) + image_tokens);
            }
            if let Some(cached_text_tokens) = other_output_details.cached_text_tokens {
                self_output_details.cached_text_tokens =
                    Some(self_output_details.cached_text_tokens.unwrap_or(0) + cached_text_tokens);
            }
            if let Some(cached_audio_tokens) = other_output_details.cached_audio_tokens {
                self_output_details.cached_audio_tokens = Some(
                    self_output_details.cached_audio_tokens.unwrap_or(0) + cached_audio_tokens,
                );
            }
            if let Some(cached_image_tokens) = other_output_details.cached_image_tokens {
                self_output_details.cached_image_tokens = Some(
                    self_output_details.cached_image_tokens.unwrap_or(0) + cached_image_tokens,
                );
            }
            if let Some(cached_tokens) = other_output_details.cached_tokens {
                self_output_details.cached_tokens =
                    Some(self_output_details.cached_tokens.unwrap_or(0) + cached_tokens);
            }
            if let Some(cache_write_tokens) = other_output_details.cache_write_tokens {
                self_output_details.cache_write_tokens =
                    Some(self_output_details.cache_write_tokens.unwrap_or(0) + cache_write_tokens);
            }
            if let Some(reasoning_tokens) = other_output_details.reasoning_tokens {
                self_output_details.reasoning_tokens =
                    Some(self_output_details.reasoning_tokens.unwrap_or(0) + reasoning_tokens);
            }
        }
    }
}
