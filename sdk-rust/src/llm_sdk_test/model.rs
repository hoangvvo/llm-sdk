use std::{collections::VecDeque, sync::Mutex};

use futures::stream;

use crate::{
    boxed_stream::BoxedStream,
    errors::{LanguageModelError, LanguageModelResult},
    language_model::{LanguageModel, LanguageModelMetadata, LanguageModelStream},
    LanguageModelInput, ModelResponse, PartialModelResponse,
};

/// Result for a mocked `generate` call.
/// It can either be a full response or an error to return.
pub enum MockGenerateResult {
    Response(ModelResponse),
    Error(LanguageModelError),
}

impl MockGenerateResult {
    /// Construct a result that yields the provided response.
    pub fn response(response: ModelResponse) -> Self {
        Self::Response(response)
    }

    /// Construct a result that yields the provided error.
    pub fn error(error: LanguageModelError) -> Self {
        Self::Error(error)
    }
}

impl From<ModelResponse> for MockGenerateResult {
    fn from(response: ModelResponse) -> Self {
        Self::response(response)
    }
}

impl From<LanguageModelResult<ModelResponse>> for MockGenerateResult {
    fn from(result: LanguageModelResult<ModelResponse>) -> Self {
        match result {
            Ok(response) => Self::Response(response),
            Err(error) => Self::Error(error),
        }
    }
}

/// Result for a mocked `stream` call.
/// It can either be a set of partial responses or an error to return.
pub enum MockStreamResult {
    Partials(Vec<PartialModelResponse>),
    Error(LanguageModelError),
}

impl MockStreamResult {
    /// Construct a result that yields the provided partial responses.
    pub fn partials(partials: Vec<PartialModelResponse>) -> Self {
        Self::Partials(partials)
    }

    /// Construct a result that yields the provided error.
    pub fn error(error: LanguageModelError) -> Self {
        Self::Error(error)
    }
}

impl From<Vec<PartialModelResponse>> for MockStreamResult {
    fn from(partials: Vec<PartialModelResponse>) -> Self {
        Self::partials(partials)
    }
}

impl From<PartialModelResponse> for MockStreamResult {
    fn from(partial: PartialModelResponse) -> Self {
        Self::partials(vec![partial])
    }
}

impl From<LanguageModelResult<Vec<PartialModelResponse>>> for MockStreamResult {
    fn from(result: LanguageModelResult<Vec<PartialModelResponse>>) -> Self {
        match result {
            Ok(partials) => Self::Partials(partials),
            Err(error) => Self::Error(error),
        }
    }
}

#[derive(Default)]
struct MockLanguageModelState {
    mocked_generate_results: VecDeque<MockGenerateResult>,
    mocked_stream_results: VecDeque<MockStreamResult>,
    tracked_generate_inputs: Vec<LanguageModelInput>,
    tracked_stream_inputs: Vec<LanguageModelInput>,
}

impl MockLanguageModelState {
    fn enqueue_generate_result(&mut self, result: MockGenerateResult) {
        self.mocked_generate_results.push_back(result);
    }

    fn enqueue_stream_result(&mut self, result: MockStreamResult) {
        self.mocked_stream_results.push_back(result);
    }

    fn reset(&mut self) {
        self.tracked_generate_inputs.clear();
        self.tracked_stream_inputs.clear();
    }

    fn restore(&mut self) {
        self.mocked_generate_results.clear();
        self.mocked_stream_results.clear();
        self.reset();
    }
}

/// A mock language model for testing that tracks inputs and yields predefined outputs.
pub struct MockLanguageModel {
    provider: &'static str,
    model_id: String,
    metadata: Option<LanguageModelMetadata>,
    state: Mutex<MockLanguageModelState>,
}

impl Default for MockLanguageModel {
    fn default() -> Self {
        Self {
            provider: "mock",
            model_id: "mock-model".to_string(),
            metadata: None,
            state: Mutex::new(MockLanguageModelState::default()),
        }
    }
}

impl MockLanguageModel {
    /// Construct a new mock language model instance.
    pub fn new() -> Self {
        Self::default()
    }

    /// Override the provider identifier returned by the mock.
    pub fn set_provider(&mut self, provider: &'static str) {
        self.provider = provider;
    }

    /// Override the model identifier returned by the mock.
    pub fn set_model_id<S: Into<String>>(&mut self, model_id: S) {
        self.model_id = model_id.into();
    }

    /// Override the metadata returned by the mock.
    pub fn set_metadata(&mut self, metadata: Option<LanguageModelMetadata>) {
        self.metadata = metadata;
    }

    /// Enqueue one or more mocked generate results.
    pub fn enqueue_generate_results<I>(&self, results: I) -> &Self
    where
        I: IntoIterator<Item = MockGenerateResult>,
    {
        let mut state = self.state.lock().expect("mock state poisoned");
        for result in results {
            state.enqueue_generate_result(result);
        }
        drop(state);
        self
    }

    /// Convenience to enqueue a single mocked generate result.
    pub fn enqueue_generate<R>(&self, result: R) -> &Self
    where
        R: Into<MockGenerateResult>,
    {
        self.enqueue_generate_results(std::iter::once(result.into()))
    }

    /// Enqueue one or more mocked stream results.
    pub fn enqueue_stream_results<I>(&self, results: I) -> &Self
    where
        I: IntoIterator<Item = MockStreamResult>,
    {
        let mut state = self.state.lock().expect("mock state poisoned");
        for result in results {
            state.enqueue_stream_result(result);
        }
        drop(state);
        self
    }

    /// Convenience to enqueue a single mocked stream result.
    pub fn enqueue_stream<R>(&self, result: R) -> &Self
    where
        R: Into<MockStreamResult>,
    {
        self.enqueue_stream_results(std::iter::once(result.into()))
    }

    /// Retrieve the tracked generate inputs accumulated so far.
    pub fn tracked_generate_inputs(&self) -> Vec<LanguageModelInput> {
        let state = self.state.lock().expect("mock state poisoned");
        state.tracked_generate_inputs.clone()
    }

    /// Retrieve the tracked stream inputs accumulated so far.
    pub fn tracked_stream_inputs(&self) -> Vec<LanguageModelInput> {
        let state = self.state.lock().expect("mock state poisoned");
        state.tracked_stream_inputs.clone()
    }

    /// Reset tracked inputs without touching enqueued results.
    pub fn reset(&self) {
        let mut state = self.state.lock().expect("mock state poisoned");
        state.reset();
    }

    /// Clear both tracked inputs and enqueued results.
    pub fn restore(&self) {
        let mut state = self.state.lock().expect("mock state poisoned");
        state.restore();
    }
}

#[async_trait::async_trait]
impl LanguageModel for MockLanguageModel {
    fn provider(&self) -> &'static str {
        self.provider
    }

    fn model_id(&self) -> String {
        self.model_id.clone()
    }

    fn metadata(&self) -> Option<&LanguageModelMetadata> {
        self.metadata.as_ref()
    }

    async fn generate(&self, input: LanguageModelInput) -> LanguageModelResult<ModelResponse> {
        let mut state = self.state.lock().expect("mock state poisoned");
        state.tracked_generate_inputs.push(input.clone());

        let result = state.mocked_generate_results.pop_front().ok_or_else(|| {
            LanguageModelError::Invariant(
                self.provider,
                "no mocked generate results available".into(),
            )
        })?;

        match result {
            MockGenerateResult::Response(response) => Ok(response),
            MockGenerateResult::Error(error) => Err(error),
        }
    }

    async fn stream(&self, input: LanguageModelInput) -> LanguageModelResult<LanguageModelStream> {
        let mut state = self.state.lock().expect("mock state poisoned");

        let result = state.mocked_stream_results.pop_front().ok_or_else(|| {
            LanguageModelError::Invariant(
                self.provider,
                "no mocked stream results available".into(),
            )
        })?;

        state.tracked_stream_inputs.push(input.clone());

        match result {
            MockStreamResult::Error(error) => Err(error),
            MockStreamResult::Partials(partials) => {
                let stream = stream_from_partials(partials);
                Ok(stream)
            }
        }
    }
}

fn stream_from_partials(partials: Vec<PartialModelResponse>) -> LanguageModelStream {
    let iter = stream::iter(partials.into_iter().map(Ok));
    BoxedStream::from_stream(iter)
}
