use crate::{
    run::RunSession, tool::AgentTool, types::AgentStream, AgentError, AgentRequest, AgentResponse,
    InstructionParam,
};
use futures::stream::StreamExt;
use llm_sdk::{LanguageModel, ResponseFormatOption};
use std::sync::Arc;

pub struct Agent<TCtx> {
    /// A unique name for the agent.
    /// The name can only contain letters and underscores.
    pub name: String,
    model: Arc<dyn LanguageModel + Send + Sync>,
    instructions: Arc<Vec<InstructionParam<TCtx>>>,
    tools: Arc<Vec<Box<dyn AgentTool<TCtx>>>>,
    response_format: ResponseFormatOption,
    max_turns: usize,
    temperature: Option<f64>,
    top_p: Option<f64>,
    top_k: Option<f64>,
    presence_penalty: Option<f64>,
    frequency_penalty: Option<f64>,
}

impl<TCtx> Agent<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    #[must_use]
    pub fn new(params: AgentParams<TCtx>) -> Self {
        Self {
            name: params.name,
            model: params.model,
            instructions: Arc::new(params.instructions),
            tools: Arc::new(params.tools),
            response_format: params.response_format,
            max_turns: params.max_turns,
            temperature: params.temperature,
            top_p: params.top_p,
            top_k: params.top_k,
            presence_penalty: params.presence_penalty,
            frequency_penalty: params.frequency_penalty,
        }
    }
    /// Create a one-time run of the agent and generate a response.
    /// A session is created for the run and cleaned up afterwards.
    pub async fn run(&self, request: AgentRequest<TCtx>) -> Result<AgentResponse, AgentError> {
        let run_session = self.create_session().await;
        let res = run_session.run(request).await?;
        run_session.finish();
        Ok(res)
    }

    /// Create a one-time streaming run of the agent and generate a response.
    /// A session is created for the run and cleaned up afterwards.
    pub async fn run_stream(&self, request: AgentRequest<TCtx>) -> Result<AgentStream, AgentError> {
        let run_session = Arc::new(self.create_session().await);
        let mut stream = run_session.clone().run_stream(request)?;

        let wrapped_stream = async_stream::stream! {
            let run_session = run_session;
            while let Some(item) = stream.next().await {
                yield item;
            }

            if let Ok(session) = Arc::try_unwrap(run_session) {
                session.finish();
            }
        };

        Ok(AgentStream::from_stream(wrapped_stream))
    }

    /// Create a session for stateful multiple runs of the agent
    pub async fn create_session(&self) -> RunSession<TCtx> {
        RunSession::new(
            self.model.clone(),
            self.instructions.clone(),
            self.tools.clone(),
            self.response_format.clone(),
            self.max_turns,
            self.temperature,
            self.top_p,
            self.top_k,
            self.presence_penalty,
            self.frequency_penalty,
        )
        .await
    }

    pub fn builder(name: &str, model: Arc<dyn LanguageModel + Send + Sync>) -> AgentParams<TCtx> {
        AgentParams::new(name, model)
    }
}

/// Parameters required to create a new agent.
/// # Default Values
/// - `instructions`: `vec![]`
/// - `tools`: `vec![]`
/// - `response_format`: `ResponseFormatOption::Text`
/// - `max_turns`: 10
/// - `temperature`: `None`
/// - `top_p`: `None`
/// - `top_k`: `None`
/// - `presence_penalty`: `None`
/// - `frequency_penalty`: `None`
pub struct AgentParams<TCtx> {
    pub name: String,
    /// The default language model to use for the agent.
    pub model: Arc<dyn LanguageModel + Send + Sync>,
    /// Instructions to be added to system messages when executing the agent.
    /// This can include formatting instructions or other guidance for the
    /// agent.
    pub instructions: Vec<InstructionParam<TCtx>>,
    /// The tools that the agent can use to perform tasks.
    pub tools: Vec<Box<dyn AgentTool<TCtx>>>,
    /// The expected format of the response. Either text or structured output.
    pub response_format: ResponseFormatOption,
    /// Max number of turns for agent to run to protect against infinite loops.
    pub max_turns: usize,
    /// Amount of randomness injected into the response. Ranges from 0.0 to 1.0
    pub temperature: Option<f64>,
    /// An alternative to sampling with temperature, called nucleus sampling,
    /// where the model considers the results of the tokens with `top_p`
    /// probability mass. Ranges from 0.0 to 1.0
    pub top_p: Option<f64>,
    /// Only sample from the top K options for each subsequent token.
    /// Used to remove 'long tail' low probability responses.
    pub top_k: Option<f64>,
    /// Positive values penalize new tokens based on whether they appear in the
    /// text so far, increasing the model's likelihood to talk about new
    /// topics.
    pub presence_penalty: Option<f64>,
    /// Positive values penalize new tokens based on their existing frequency in
    /// the text so far, decreasing the model's likelihood to repeat the
    /// same line verbatim.
    pub frequency_penalty: Option<f64>,
}

impl<TCtx> AgentParams<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    pub fn new(name: &str, model: Arc<dyn LanguageModel + Send + Sync>) -> Self {
        Self {
            name: name.to_string(),
            model,
            instructions: Vec::new(),
            tools: Vec::new(),
            response_format: ResponseFormatOption::Text,
            max_turns: 10,
            temperature: None,
            top_p: None,
            top_k: None,
            presence_penalty: None,
            frequency_penalty: None,
        }
    }

    /// Add an instruction
    #[must_use]
    pub fn add_instruction(mut self, instruction: impl Into<InstructionParam<TCtx>>) -> Self {
        self.instructions.push(instruction.into());
        self
    }

    /// Add a tool
    #[must_use]
    pub fn add_tool(mut self, tool: impl AgentTool<TCtx> + 'static) -> Self {
        self.tools.push(Box::new(tool));
        self
    }

    /// Set the response format
    #[must_use]
    pub fn response_format(mut self, response_format: ResponseFormatOption) -> Self {
        self.response_format = response_format;
        self
    }

    /// Set the max turns
    #[must_use]
    pub fn max_turns(mut self, max_turns: usize) -> Self {
        self.max_turns = max_turns;
        self
    }

    /// Set the temperature for sampling
    /// Amount of randomness injected into the response. Ranges from 0.0 to 1.0
    #[must_use]
    pub fn temperature(mut self, temperature: f64) -> Self {
        self.temperature = Some(temperature);
        self
    }

    /// Set the `top_p` for nucleus sampling
    /// An alternative to sampling with temperature, called nucleus sampling,
    /// where the model considers the results of the tokens with `top_p`
    /// probability mass. Ranges from 0.0 to 1.0
    #[must_use]
    pub fn top_p(mut self, top_p: f64) -> Self {
        self.top_p = Some(top_p);
        self
    }

    /// Set the `top_k` for sampling
    /// Only sample from the top K options for each subsequent token.
    /// Used to remove 'long tail' low probability responses.
    #[must_use]
    pub fn top_k(mut self, top_k: f64) -> Self {
        self.top_k = Some(top_k);
        self
    }

    /// Set the presence penalty
    /// Positive values penalize new tokens based on whether they appear in the
    /// text so far, increasing the model's likelihood to talk about new
    /// topics.
    #[must_use]
    pub fn presence_penalty(mut self, presence_penalty: f64) -> Self {
        self.presence_penalty = Some(presence_penalty);
        self
    }

    /// Set the frequency penalty
    /// Positive values penalize new tokens based on their existing frequency in
    /// the text so far, decreasing the model's likelihood to repeat the
    /// same line verbatim.
    #[must_use]
    pub fn frequency_penalty(mut self, frequency_penalty: f64) -> Self {
        self.frequency_penalty = Some(frequency_penalty);
        self
    }

    #[must_use]
    pub fn build(self) -> Agent<TCtx> {
        Agent::new(self)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{AgentItem, AgentRequest};
    use llm_sdk::{
        ContentDelta, LanguageModel, LanguageModelError, LanguageModelInput, LanguageModelMetadata,
        LanguageModelStream, Message, ModelResponse, ModelUsage, Part, PartDelta,
        PartialModelResponse, TextPart, TextPartDelta, UserMessage,
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
            content: vec![Part::Text(TextPart {
                text: "Mock response".to_string(),
                id: None,
            })],
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
                input: vec![AgentItem::Message(Message::User(UserMessage {
                    content: vec![Part::Text(TextPart {
                        text: "Hello".to_string(),
                        id: None,
                    })],
                }))],
            })
            .await
            .unwrap();

        // Check the content contains our expected text
        assert_eq!(response.content.len(), 1);
        if let Part::Text(text_part) = &response.content[0] {
            assert_eq!(text_part.text, "Mock response");
        }
        assert_eq!(response.output.len(), 1);

        if let AgentItem::Message(Message::Assistant(assistant_msg)) = &response.output[0] {
            assert_eq!(assistant_msg.content.len(), 1);
            if let Part::Text(text_part) = &assistant_msg.content[0] {
                assert_eq!(text_part.text, "Mock response");
            }
        } else {
            panic!("Expected assistant message");
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
                            id: None,
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
                input: vec![AgentItem::Message(Message::User(UserMessage {
                    content: vec![Part::Text(TextPart {
                        text: "Hello".to_string(),
                        id: None,
                    })],
                }))],
            })
            .await
            .unwrap();

        let events: Vec<_> = stream.collect().await;

        // Should have partial events, message event, and response event
        assert!(!events.is_empty());

        // Check that we get at least one partial event
        let has_partial = events.iter().any(|event| {
            if let Ok(crate::types::AgentStreamEvent::Partial(_)) = event {
                true
            } else {
                false
            }
        });
        assert!(has_partial);
    }
}
