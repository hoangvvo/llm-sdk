use crate::{
    boxed_stream::BoxedStream, LanguageModelCapability, LanguageModelInput, LanguageModelPricing,
    LanguageModelResult, ModelResponse, PartialModelResponse,
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
    fn metadata(&self) -> Option<&LanguageModelMetadata>;
    async fn generate(&self, input: LanguageModelInput) -> LanguageModelResult<ModelResponse>;
    async fn stream(&self, input: LanguageModelInput) -> LanguageModelResult<LanguageModelStream>;
}

pub type LanguageModelStream = BoxedStream<'static, LanguageModelResult<PartialModelResponse>>;
