use crate::{
    boxed_stream::BoxedStream, LanguageModelCapabilities, LanguageModelInput, LanguageModelPricing,
    LanguageModelResult, ModelResponse, PartialModelResponse,
};
use futures::future::BoxFuture;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct LanguageModelMetadata {
    pub pricing: Option<LanguageModelPricing>,
    pub capabilities: Option<LanguageModelCapabilities>,
}

pub trait LanguageModel: Send + Sync {
    fn provider(&self) -> &'static str;
    fn model_id(&self) -> String;
    fn metadata(&self) -> Option<&LanguageModelMetadata>;
    fn generate(
        &self,
        input: LanguageModelInput,
    ) -> BoxFuture<'_, LanguageModelResult<ModelResponse>>;
    fn stream(
        &self,
        input: LanguageModelInput,
    ) -> BoxFuture<'_, LanguageModelResult<LanguageModelStream>>;
}

pub type LanguageModelStream = BoxedStream<'static, LanguageModelResult<PartialModelResponse>>;
