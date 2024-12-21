use crate::{LanguageModelInput, LanguageModelResult, ModelResponse};

#[async_trait::async_trait]
pub trait LanguageModel: Send + Sync {
    fn provider(&self) -> &'static str;
    fn model_id(&self) -> String;
    async fn generate(&self, input: LanguageModelInput) -> LanguageModelResult<ModelResponse>;
}
