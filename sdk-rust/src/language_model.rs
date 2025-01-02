use futures_core::Stream;

use crate::{
    LanguageModelCapability, LanguageModelInput, LanguageModelPricing, LanguageModelResult,
    ModelResponse, PartialModelResponse,
};

#[derive(Debug, Clone, Default)]
pub struct LanguageModelMetadata {
    pub pricing: Option<LanguageModelPricing>,
    pub capabilities: Option<Vec<LanguageModelCapability>>,
}

#[async_trait::async_trait]
pub trait LanguageModel: Send + Sync {
    fn provider(&self) -> &'static str;
    fn model_id(&self) -> String;
    async fn generate(&self, input: LanguageModelInput) -> LanguageModelResult<ModelResponse>;
    async fn stream(&self, input: LanguageModelInput) -> LanguageModelStreamResult;
}

pub struct LanguageModelStreamResult {}

impl Stream for LanguageModelStreamResult {
    type Item = LanguageModelResult<PartialModelResponse>;

    fn poll_next(
        self: std::pin::Pin<&mut Self>,
        _cx: &mut std::task::Context<'_>,
    ) -> std::task::Poll<Option<Self::Item>> {
        // Implement the logic to poll the next item
        std::task::Poll::Pending
    }
}
