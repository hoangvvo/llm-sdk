use crate::{LanguageModelPricing, ModelUsage};

impl ModelUsage {
    #[must_use]
    pub fn calculate_cost(&self, pricing: &LanguageModelPricing) -> f64 {
        let input_text_tokens = self
            .input_tokens_details
            .as_ref()
            .and_then(|details| details.text_tokens)
            .unwrap_or(self.input_tokens);
        let input_audio_tokens = self
            .input_tokens_details
            .as_ref()
            .and_then(|details| details.audio_tokens)
            .unwrap_or(0);
        let input_image_tokens = self
            .input_tokens_details
            .as_ref()
            .and_then(|details| details.image_tokens)
            .unwrap_or(0);
        let input_cached_text_tokens = self
            .input_tokens_details
            .as_ref()
            .and_then(|details| details.cached_text_tokens)
            .unwrap_or(0);
        let input_cached_audio_tokens = self
            .input_tokens_details
            .as_ref()
            .and_then(|details| details.cached_audio_tokens)
            .unwrap_or(0);
        let input_cached_image_tokens = self
            .input_tokens_details
            .as_ref()
            .and_then(|details| details.cached_image_tokens)
            .unwrap_or(0);

        let output_text_tokens = self
            .output_tokens_details
            .as_ref()
            .and_then(|details| details.text_tokens)
            .unwrap_or(self.output_tokens);
        let output_audio_tokens = self
            .output_tokens_details
            .as_ref()
            .and_then(|details| details.audio_tokens)
            .unwrap_or(0);
        let output_image_tokens = self
            .output_tokens_details
            .as_ref()
            .and_then(|details| details.image_tokens)
            .unwrap_or(0);

        f64::from(input_text_tokens) * pricing.input_cost_per_text_token.unwrap_or(0.0)
            + f64::from(input_audio_tokens) * pricing.input_cost_per_audio_token.unwrap_or(0.0)
            + f64::from(input_image_tokens) * pricing.input_cost_per_image_token.unwrap_or(0.0)
            + f64::from(input_cached_text_tokens) * pricing.input_cost_per_text_token.unwrap_or(0.0)
            + f64::from(input_cached_audio_tokens)
                * pricing.input_cost_per_audio_token.unwrap_or(0.0)
            + f64::from(input_cached_image_tokens)
                * pricing.input_cost_per_image_token.unwrap_or(0.0)
            + f64::from(output_text_tokens) * pricing.output_cost_per_text_token.unwrap_or(0.0)
            + f64::from(output_audio_tokens) * pricing.output_cost_per_audio_token.unwrap_or(0.0)
            + f64::from(output_image_tokens) * pricing.output_cost_per_image_token.unwrap_or(0.0)
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
        }
    }
}
