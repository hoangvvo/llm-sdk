use crate::{
    openai::{OpenAIModel, OpenAIModelOptions},
    LanguageModel,
};

/// Store all available Large language models and use a suitable one based on
/// input provider name and model ID
pub struct LanguageModelRegistry {
    language_models: Vec<Box<dyn LanguageModel>>,
}

pub struct LanguageModelRegistryOptions {
    pub openai_api_key: String,
}

impl LanguageModelRegistry {
    pub fn new(options: LanguageModelRegistryOptions) -> Self {
        Self {
            language_models: vec![
                // === OpenAI models ===
                // pricing: https://openai.com/api/pricing/
                // models: https://platform.openai.com/docs/models
                Box::new(OpenAIModel::new(OpenAIModelOptions {
                    api_key: options.openai_api_key.clone(),
                    model_id: "gpt-4o".to_string(),
                    base_url: None,
                    structured_outputs: true,
                })),
            ],
        }
    }

    pub fn get_model(&self, model_id: String) -> Option<&dyn LanguageModel> {
        self.language_models
            .iter()
            .find(|model| model.model_id() == model_id)
            .map(|model| model.as_ref())
    }
}
