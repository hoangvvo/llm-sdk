use std::{
    pin::Pin,
    task::{Context, Poll},
};

use crate::{
    LanguageModelCapability, LanguageModelInput, LanguageModelPricing, LanguageModelResult,
    ModelResponse, PartialModelResponse,
};
use futures_core::Stream;

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
    async fn stream(&self, input: LanguageModelInput) -> LanguageModelResult<LanguageModelStream>;
}

pub struct LanguageModelStream(
    Pin<Box<dyn Stream<Item = LanguageModelResult<PartialModelResponse>> + Send>>,
);

impl LanguageModelStream {
    pub fn from_stream<S>(stream: S) -> Self
    where
        S: Stream<Item = LanguageModelResult<PartialModelResponse>> + Send + 'static,
    {
        Self(Box::pin(stream))
    }
}

impl Stream for LanguageModelStream {
    type Item = LanguageModelResult<PartialModelResponse>;

    fn poll_next(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<Option<Self::Item>> {
        self.0.as_mut().poll_next(cx)
    }
}
