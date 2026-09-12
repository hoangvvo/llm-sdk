use crate::{LanguageModelPricing, ModelServerToolUsage, ModelTokensDetails, ModelUsage};

/// Counting conventions needed to price unmodified provider usage.
pub struct ModelUsageCostOptions {
    /// True when `input_tokens` excludes cache reads and writes.
    pub input_cache_tokens_are_additional: bool,
    /// True when `output_tokens` excludes reasoning tokens.
    pub output_reasoning_tokens_are_additional: bool,
}

impl ModelUsage {
    /// Estimates USD charges using the provider's counting conventions.
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
        // Unattributed tokens are assumed to be text. The highest configured
        // rate is only a fallback for models, such as TTS, that do not
        // define a text rate.
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

        let mut input_cost = f64::from(input_tokens) * input_base_price;
        let mut output_cost = f64::from(output_tokens) * output_base_price;
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
            input_cost += adjustment(
                details.audio_tokens.unwrap_or(0),
                input_base_price,
                input_audio_price,
            );
            input_cost += adjustment(
                details.image_tokens.unwrap_or(0),
                input_base_price,
                input_image_price,
            );
        }
        if let Some(details) = output_details {
            output_cost += adjustment(
                details.audio_tokens.unwrap_or(0),
                output_base_price,
                output_audio_price,
            );
            output_cost += adjustment(
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
                input_cost += adjustment(
                    details.cached_text_tokens.unwrap_or(0),
                    cache_base_text,
                    cached_text_price,
                );
                input_cost += adjustment(
                    details.cached_audio_tokens.unwrap_or(0),
                    cache_base_audio,
                    cached_audio_price,
                );
                input_cost += adjustment(
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
                input_cost += adjustment(
                    details.cached_tokens.unwrap_or(0),
                    cache_base_text,
                    cached_price,
                );
            }
            let cache_write_price = pricing
                .input_cost_per_cache_write_token
                .unwrap_or(input_text_price);
            input_cost += adjustment(
                details.cache_write_tokens.unwrap_or(0),
                cache_base_text,
                cache_write_price,
            );
            // One-hour cache writes are a subset of the cache writes charged
            // above.
            input_cost += adjustment(
                details.extended_cache_write_tokens.unwrap_or(0),
                cache_write_price,
                pricing
                    .input_cost_per_extended_cache_write_token
                    .unwrap_or(cache_write_price),
            );
        }

        if let Some(details) = output_details {
            if options.output_reasoning_tokens_are_additional {
                output_cost += f64::from(details.reasoning_tokens.unwrap_or(0)) * output_text_price;
            } else {
                output_cost += adjustment(
                    details.reasoning_tokens.unwrap_or(0),
                    output_base_price,
                    output_text_price,
                );
            }
        }

        if let Some(long_context) = &pricing.long_context {
            if input_tokens > long_context.threshold_tokens {
                input_cost *= long_context.input_cost_multiplier.unwrap_or(1.0);
                output_cost *= long_context.output_cost_multiplier.unwrap_or(1.0);
            }
        }

        let web_search_requests = self
            .server_tool_use
            .as_ref()
            .and_then(|usage| usage.web_search_requests)
            .unwrap_or(0);
        input_cost
            + output_cost
            + f64::from(web_search_requests)
                * pricing.cost_per_web_search_request.unwrap_or(0.0).max(0.0)
    }

    /// Sums the usage of a separate request into this usage.
    pub fn add(&mut self, other: &Self) {
        self.input_tokens = self.input_tokens.saturating_add(other.input_tokens);
        self.output_tokens = self.output_tokens.saturating_add(other.output_tokens);

        if let Some(other_details) = &other.input_tokens_details {
            self.input_tokens_details
                .get_or_insert_default()
                .add(other_details);
        }
        if let Some(other_details) = &other.output_tokens_details {
            self.output_tokens_details
                .get_or_insert_default()
                .add(other_details);
        }
        if let Some(other_usage) = &other.server_tool_use {
            self.server_tool_use
                .get_or_insert_default()
                .add(other_usage);
        }
    }

    /// Merges a cumulative usage snapshot of the same request, such as the
    /// usage reported by a later stream chunk, keeping the highest known
    /// counts.
    pub fn merge_max(&mut self, other: &Self) {
        self.input_tokens = self.input_tokens.max(other.input_tokens);
        self.output_tokens = self.output_tokens.max(other.output_tokens);

        if let Some(other_details) = &other.input_tokens_details {
            self.input_tokens_details
                .get_or_insert_default()
                .merge_max(other_details);
        }
        if let Some(other_details) = &other.output_tokens_details {
            self.output_tokens_details
                .get_or_insert_default()
                .merge_max(other_details);
        }
        if let Some(other_usage) = &other.server_tool_use {
            self.server_tool_use
                .get_or_insert_default()
                .merge_max(other_usage);
        }
    }
}

impl ModelTokensDetails {
    /// Sums the counts of another breakdown into this one.
    pub fn add(&mut self, other: &Self) {
        sum_count(&mut self.text_tokens, other.text_tokens);
        sum_count(&mut self.audio_tokens, other.audio_tokens);
        sum_count(&mut self.image_tokens, other.image_tokens);
        sum_count(&mut self.cached_text_tokens, other.cached_text_tokens);
        sum_count(&mut self.cached_audio_tokens, other.cached_audio_tokens);
        sum_count(&mut self.cached_image_tokens, other.cached_image_tokens);
        sum_count(&mut self.cached_tokens, other.cached_tokens);
        sum_count(&mut self.cache_write_tokens, other.cache_write_tokens);
        sum_count(
            &mut self.extended_cache_write_tokens,
            other.extended_cache_write_tokens,
        );
        sum_count(&mut self.reasoning_tokens, other.reasoning_tokens);
    }

    /// Keeps the highest known count of every field.
    pub fn merge_max(&mut self, other: &Self) {
        max_count(&mut self.text_tokens, other.text_tokens);
        max_count(&mut self.audio_tokens, other.audio_tokens);
        max_count(&mut self.image_tokens, other.image_tokens);
        max_count(&mut self.cached_text_tokens, other.cached_text_tokens);
        max_count(&mut self.cached_audio_tokens, other.cached_audio_tokens);
        max_count(&mut self.cached_image_tokens, other.cached_image_tokens);
        max_count(&mut self.cached_tokens, other.cached_tokens);
        max_count(&mut self.cache_write_tokens, other.cache_write_tokens);
        max_count(
            &mut self.extended_cache_write_tokens,
            other.extended_cache_write_tokens,
        );
        max_count(&mut self.reasoning_tokens, other.reasoning_tokens);
    }
}

impl ModelServerToolUsage {
    /// Sums the counts of another server tool usage into this one.
    pub fn add(&mut self, other: &Self) {
        sum_count(&mut self.web_search_requests, other.web_search_requests);
    }

    /// Keeps the highest known count of every field.
    pub fn merge_max(&mut self, other: &Self) {
        max_count(&mut self.web_search_requests, other.web_search_requests);
    }
}

fn sum_count(current: &mut Option<u32>, incoming: Option<u32>) {
    if let Some(incoming) = incoming {
        *current = Some(current.unwrap_or(0).saturating_add(incoming));
    }
}

fn max_count(current: &mut Option<u32>, incoming: Option<u32>) {
    if let Some(incoming) = incoming {
        *current = Some(current.unwrap_or(0).max(incoming));
    }
}
