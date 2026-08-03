use crate::{LanguageModelPricing, ModelTokensDetails, ModelUsage};

pub struct ModelUsageCostOptions {
    pub input_cache_tokens_are_additional: bool,
    pub output_reasoning_tokens_are_additional: bool,
}

impl ModelUsage {
    #[allow(clippy::too_many_lines)]
    #[must_use]
    pub fn calculate_cost(
        &self,
        pricing: &LanguageModelPricing,
        options: &ModelUsageCostOptions,
    ) -> f64 {
        let max_price =
            |prices: &[Option<f64>]| prices.iter().flatten().copied().fold(0.0_f64, f64::max);
        let has_modality_breakdown = |details: Option<&ModelTokensDetails>| {
            details.is_some_and(|details| {
                details.text_tokens.is_some()
                    || details.audio_tokens.is_some()
                    || details.image_tokens.is_some()
            })
        };
        let modality_total = |details: Option<&ModelTokensDetails>| {
            details.map_or(0, |details| {
                details
                    .text_tokens
                    .unwrap_or(0)
                    .saturating_add(details.audio_tokens.unwrap_or(0))
                    .saturating_add(details.image_tokens.unwrap_or(0))
            })
        };

        let input_details = self.input_tokens_details.as_ref();
        let output_details = self.output_tokens_details.as_ref();
        let input_has_modality_breakdown = has_modality_breakdown(input_details);
        let input_max_price = max_price(&[
            pricing.input_cost_per_text_token,
            pricing.input_cost_per_audio_token,
            pricing.input_cost_per_image_token,
        ]);
        let output_max_price = max_price(&[
            pricing.output_cost_per_text_token,
            pricing.output_cost_per_audio_token,
            pricing.output_cost_per_image_token,
        ]);
        // Unattributed tokens are assumed to be text. The highest configured rate is
        // only a fallback for models, such as TTS, that do not define a text rate.
        let input_base_price = pricing.input_cost_per_text_token.unwrap_or(input_max_price);
        let output_base_price = pricing
            .output_cost_per_text_token
            .unwrap_or(output_max_price);

        let has_cached_modalities = input_details.is_some_and(|details| {
            details.cached_text_tokens.is_some()
                || details.cached_audio_tokens.is_some()
                || details.cached_image_tokens.is_some()
        });
        let cached_read_tokens = input_details.map_or(0, |details| {
            if has_cached_modalities {
                details
                    .cached_text_tokens
                    .unwrap_or(0)
                    .saturating_add(details.cached_audio_tokens.unwrap_or(0))
                    .saturating_add(details.cached_image_tokens.unwrap_or(0))
            } else {
                details.cached_tokens.unwrap_or(0)
            }
        });
        let included_cache_tokens = if options.input_cache_tokens_are_additional {
            0
        } else {
            cached_read_tokens.saturating_add(
                input_details
                    .and_then(|details| details.cache_write_tokens)
                    .unwrap_or(0),
            )
        };
        let input_tokens = self
            .input_tokens
            .max(modality_total(input_details))
            .max(included_cache_tokens);
        let output_detail_tokens = modality_total(output_details).saturating_add(
            if options.output_reasoning_tokens_are_additional {
                0
            } else {
                output_details
                    .and_then(|details| details.reasoning_tokens)
                    .unwrap_or(0)
            },
        );
        let output_tokens = self.output_tokens.max(output_detail_tokens);

        let mut cost = f64::from(input_tokens) * input_base_price
            + f64::from(output_tokens) * output_base_price;
        let adjustment = |tokens: u32, regular_price: f64, category_price: f64| {
            f64::from(tokens) * (category_price - regular_price)
        };

        let input_text_price = pricing
            .input_cost_per_text_token
            .unwrap_or(input_base_price);
        let input_audio_price = pricing
            .input_cost_per_audio_token
            .unwrap_or(input_text_price);
        let input_image_price = pricing
            .input_cost_per_image_token
            .unwrap_or(input_text_price);
        let output_text_price = pricing
            .output_cost_per_text_token
            .unwrap_or(output_base_price);
        let output_audio_price = pricing
            .output_cost_per_audio_token
            .unwrap_or(output_text_price);
        let output_image_price = pricing
            .output_cost_per_image_token
            .unwrap_or(output_text_price);

        if let Some(details) = input_details {
            cost += adjustment(
                details.audio_tokens.unwrap_or(0),
                input_base_price,
                input_audio_price,
            );
            cost += adjustment(
                details.image_tokens.unwrap_or(0),
                input_base_price,
                input_image_price,
            );
        }
        if let Some(details) = output_details {
            cost += adjustment(
                details.audio_tokens.unwrap_or(0),
                output_base_price,
                output_audio_price,
            );
            cost += adjustment(
                details.image_tokens.unwrap_or(0),
                output_base_price,
                output_image_price,
            );
        }

        let (cache_base_text, cache_base_audio, cache_base_image) =
            if options.input_cache_tokens_are_additional {
                (0.0, 0.0, 0.0)
            } else if input_has_modality_breakdown {
                (input_text_price, input_audio_price, input_image_price)
            } else {
                (input_base_price, input_base_price, input_base_price)
            };
        if let Some(details) = input_details {
            if has_cached_modalities {
                let cached_text_price = pricing
                    .input_cost_per_cached_text_token
                    .or(pricing.input_cost_per_cached_token)
                    .unwrap_or(input_text_price);
                let cached_audio_price = pricing
                    .input_cost_per_cached_audio_token
                    .or(pricing.input_cost_per_cached_token)
                    .unwrap_or(input_audio_price);
                let cached_image_price = pricing
                    .input_cost_per_cached_image_token
                    .or(pricing.input_cost_per_cached_token)
                    .unwrap_or(input_image_price);
                cost += adjustment(
                    details.cached_text_tokens.unwrap_or(0),
                    cache_base_text,
                    cached_text_price,
                );
                cost += adjustment(
                    details.cached_audio_tokens.unwrap_or(0),
                    cache_base_audio,
                    cached_audio_price,
                );
                cost += adjustment(
                    details.cached_image_tokens.unwrap_or(0),
                    cache_base_image,
                    cached_image_price,
                );
            } else {
                let cached_price = pricing.input_cost_per_cached_token.unwrap_or_else(|| {
                    if pricing.input_cost_per_cached_text_token.is_some()
                        || pricing.input_cost_per_cached_audio_token.is_some()
                        || pricing.input_cost_per_cached_image_token.is_some()
                    {
                        max_price(&[
                            pricing.input_cost_per_cached_text_token,
                            pricing.input_cost_per_cached_audio_token,
                            pricing.input_cost_per_cached_image_token,
                        ])
                    } else {
                        input_base_price
                    }
                });
                cost += adjustment(
                    details.cached_tokens.unwrap_or(0),
                    cache_base_text,
                    cached_price,
                );
            }
            cost += adjustment(
                details.cache_write_tokens.unwrap_or(0),
                cache_base_text,
                pricing
                    .input_cost_per_cache_write_token
                    .unwrap_or(input_text_price),
            );
        }

        if let Some(details) = output_details {
            if options.output_reasoning_tokens_are_additional {
                cost += f64::from(details.reasoning_tokens.unwrap_or(0)) * output_text_price;
            } else {
                cost += adjustment(
                    details.reasoning_tokens.unwrap_or(0),
                    output_base_price,
                    output_text_price,
                );
            }
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
