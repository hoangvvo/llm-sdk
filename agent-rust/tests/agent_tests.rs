use futures::StreamExt;
use llm_agent::*;
use llm_sdk::{
    ContentDelta, LanguageModel, LanguageModelError, LanguageModelInput, LanguageModelMetadata,
    LanguageModelStream, Message, ModelResponse, ModelUsage, Part, PartDelta, PartialModelResponse,
    TextPartDelta,
};
use std::sync::{Arc, Mutex};

#[derive(Clone)]
struct MockLanguageModel {
    responses: Arc<Mutex<Vec<ModelResponse>>>,
    partial_responses: Arc<Mutex<Vec<Vec<PartialModelResponse>>>>,
    generate_calls: Arc<Mutex<Vec<LanguageModelInput>>>,
}

impl MockLanguageModel {
    fn new() -> Self {
        Self {
            responses: Arc::new(Mutex::new(Vec::new())),
            partial_responses: Arc::new(Mutex::new(Vec::new())),
            generate_calls: Arc::new(Mutex::new(Vec::new())),
        }
    }

    fn add_response(self, response: ModelResponse) -> Self {
        self.responses.lock().unwrap().push(response);
        self
    }

    fn add_partial_responses(self, responses: Vec<PartialModelResponse>) -> Self {
        self.partial_responses.lock().unwrap().push(responses);
        self
    }
}

#[async_trait::async_trait]
impl LanguageModel for MockLanguageModel {
    fn model_id(&self) -> String {
        "mock-model".to_string()
    }

    fn provider(&self) -> &'static str {
        "mock"
    }

    fn metadata(&self) -> Option<&LanguageModelMetadata> {
        None
    }

    async fn generate(
        &self,
        input: LanguageModelInput,
    ) -> Result<ModelResponse, LanguageModelError> {
        self.generate_calls.lock().unwrap().push(input.clone());

        let mut responses = self.responses.lock().unwrap();
        if let Some(response) = responses.pop() {
            Ok(response)
        } else {
            Err(LanguageModelError::InvalidInput(
                "No mock response available".to_string(),
            ))
        }
    }

    async fn stream(
        &self,
        _input: LanguageModelInput,
    ) -> Result<LanguageModelStream, LanguageModelError> {
        let mut partial_responses = self.partial_responses.lock().unwrap();
        if let Some(responses) = partial_responses.pop() {
            let stream = futures::stream::iter(responses.into_iter().map(Ok));
            Ok(LanguageModelStream::from_stream(stream))
        } else {
            Err(LanguageModelError::InvalidInput(
                "No mock partial responses available".to_string(),
            ))
        }
    }
}

#[tokio::test]
async fn test_agent_run_creates_session_runs_and_finishes() {
    let model = Arc::new(MockLanguageModel::new().add_response(ModelResponse {
        content: vec![Part::text("Mock response")],
        usage: Some(ModelUsage {
            input_tokens: 10,
            output_tokens: 5,
            input_tokens_details: None,
            output_tokens_details: None,
        }),
        cost: Some(0.0),
    }));

    let agent = Agent::new(AgentParams::new("test-agent", model.clone()));

    let response = agent
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
        })
        .await
        .unwrap();

    // Check the content contains our expected text
    assert_eq!(response.content.len(), 1);
    if let Part::Text(text_part) = &response.content[0] {
        assert_eq!(text_part.text, "Mock response");
    }
    assert_eq!(response.output.len(), 1);

    if let AgentItem::Model(model_response) = &response.output[0] {
        assert_eq!(model_response.content.len(), 1);
        if let Part::Text(text_part) = &model_response.content[0] {
            assert_eq!(text_part.text, "Mock response");
        }
    } else {
        panic!("Expected model response item");
    }
}

#[tokio::test]
async fn test_agent_run_stream_creates_session_streams_and_finishes() {
    let model =
        Arc::new(
            MockLanguageModel::new().add_partial_responses(vec![PartialModelResponse {
                delta: Some(ContentDelta {
                    index: 0,
                    part: PartDelta::Text(TextPartDelta {
                        text: "Mock".to_string(),
                    }),
                }),
                usage: Some(ModelUsage {
                    input_tokens: 5,
                    output_tokens: 2,
                    input_tokens_details: None,
                    output_tokens_details: None,
                }),
            }]),
        );

    let agent = Agent::new(AgentParams::new("test-agent", model.clone()));

    let stream = agent
        .run_stream(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
        })
        .await
        .unwrap();

    let events: Vec<_> = stream.collect().await;

    // Should have partial events, message event, and response event
    assert!(!events.is_empty());

    // Check that we get at least one partial event
    let has_partial = events.iter().any(|event| {
        if let Ok(AgentStreamEvent::Partial(_)) = event {
            true
        } else {
            false
        }
    });
    assert!(has_partial);
}
