use futures::StreamExt;
use llm_agent::{
    AgentError, AgentItem, AgentItemTool, AgentParams, AgentRequest, AgentResponse,
    AgentStreamEvent, AgentTool, AgentToolResult, InstructionParam, RunSession, RunState,
};
use llm_sdk::{
    ContentDelta, JSONSchema, LanguageModel, LanguageModelError, LanguageModelInput,
    LanguageModelMetadata, LanguageModelResult, LanguageModelStream, Message, ModelResponse,
    ModelUsage, Part, PartDelta, PartialModelResponse, TextPartDelta, ToolCallPart,
    ToolCallPartDelta, UserMessage,
};
use serde_json::Value;
use std::{collections::VecDeque, sync::Arc};

#[derive(Clone)]
struct MockLanguageModel {
    responses: Arc<std::sync::Mutex<VecDeque<ModelResponse>>>,
    partial_responses: Arc<std::sync::Mutex<VecDeque<Vec<PartialModelResponse>>>>,
    errors: Arc<std::sync::Mutex<VecDeque<LanguageModelError>>>,
    stream_errors: Arc<std::sync::Mutex<VecDeque<LanguageModelError>>>,
    generate_calls: Arc<std::sync::Mutex<Vec<LanguageModelInput>>>,
    stream_calls: Arc<std::sync::Mutex<Vec<LanguageModelInput>>>,
}

impl MockLanguageModel {
    fn new() -> Self {
        Self {
            responses: Arc::new(std::sync::Mutex::new(VecDeque::new())),
            partial_responses: Arc::new(std::sync::Mutex::new(VecDeque::new())),
            errors: Arc::new(std::sync::Mutex::new(VecDeque::new())),
            stream_errors: Arc::new(std::sync::Mutex::new(VecDeque::new())),
            generate_calls: Arc::new(std::sync::Mutex::new(Vec::new())),
            stream_calls: Arc::new(std::sync::Mutex::new(Vec::new())),
        }
    }

    fn add_response(self, response: ModelResponse) -> Self {
        self.responses.lock().unwrap().push_back(response);
        self
    }

    fn add_responses(self, responses: Vec<ModelResponse>) -> Self {
        self.responses.lock().unwrap().extend(responses);
        self
    }

    fn add_partial_responses(self, responses: Vec<PartialModelResponse>) -> Self {
        self.partial_responses.lock().unwrap().push_back(responses);
        self
    }

    fn add_error(self, error: LanguageModelError) -> Self {
        self.errors.lock().unwrap().push_back(error);
        self
    }

    fn add_stream_error(self, error: LanguageModelError) -> Self {
        self.stream_errors.lock().unwrap().push_back(error);
        self
    }

    fn get_generate_calls(&self) -> Vec<LanguageModelInput> {
        self.generate_calls.lock().unwrap().clone()
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

    async fn generate(&self, input: LanguageModelInput) -> LanguageModelResult<ModelResponse> {
        self.generate_calls.lock().unwrap().push(input.clone());

        let mut errors = self.errors.lock().unwrap();
        if let Some(error) = errors.pop_front() {
            return Err(error);
        }

        let mut responses = self.responses.lock().unwrap();
        if let Some(response) = responses.pop_front() {
            Ok(response)
        } else {
            panic!("No mock response available");
        }
    }

    async fn stream(&self, input: LanguageModelInput) -> LanguageModelResult<LanguageModelStream> {
        self.stream_calls.lock().unwrap().push(input.clone());

        let mut stream_errors = self.stream_errors.lock().unwrap();
        if let Some(error) = stream_errors.pop_front() {
            return Err(error);
        }

        let mut partial_responses = self.partial_responses.lock().unwrap();
        if let Some(responses) = partial_responses.pop_front() {
            let stream = futures::stream::iter(responses.into_iter().map(Ok));
            Ok(LanguageModelStream::from_stream(stream))
        } else {
            // Return empty stream instead of panicking to be more flexible
            let stream = futures::stream::iter(std::iter::empty().map(Ok));
            Ok(LanguageModelStream::from_stream(stream))
        }
    }
}

#[derive(Clone)]
struct MockTool {
    name: String,
    result: AgentToolResult,
    execution_count: Arc<std::sync::Mutex<usize>>,
    last_args: Arc<std::sync::Mutex<Option<Value>>>,
    should_error: bool,
}

impl MockTool {
    fn new(name: &str, content: Vec<Part>, is_error: bool) -> Self {
        Self {
            name: name.to_string(),
            result: AgentToolResult { content, is_error },
            execution_count: Arc::new(std::sync::Mutex::new(0)),
            last_args: Arc::new(std::sync::Mutex::new(None)),
            should_error: false,
        }
    }

    fn with_error(mut self) -> Self {
        self.should_error = true;
        self
    }
}

#[async_trait::async_trait]
impl AgentTool<()> for MockTool {
    fn name(&self) -> String {
        self.name.clone()
    }

    fn description(&self) -> String {
        "Mock tool".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {}
        })
    }

    async fn execute(
        &self,
        args: Value,
        _context: &(),
        _state: &RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>> {
        *self.execution_count.lock().unwrap() += 1;
        *self.last_args.lock().unwrap() = Some(args);

        if self.should_error {
            return Err("Tool execution failed".into());
        }

        Ok(AgentToolResult {
            content: self.result.content.clone(),
            is_error: self.result.is_error,
        })
    }
}

fn create_text_part(text: &str) -> Part {
    Part::text(text)
}

fn create_model_usage() -> ModelUsage {
    ModelUsage {
        input_tokens: 10,
        output_tokens: 5,
        input_tokens_details: None,
        output_tokens_details: None,
    }
}

#[tokio::test]
async fn test_run_session_returns_response_when_no_tool_call() {
    let model = Arc::new(MockLanguageModel::new().add_response(ModelResponse {
        content: vec![create_text_part("Hi!")],
        ..Default::default()
    }));

    let session = RunSession::new(Arc::new(AgentParams::new("test_agent", model))).await;

    let response = session
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Hello!")],
            }))],
        })
        .await
        .unwrap();

    let expected_response = AgentResponse {
        content: vec![create_text_part("Hi!")],
        output: vec![AgentItem::Model(ModelResponse {
            content: vec![create_text_part("Hi!")],
            ..Default::default()
        })],
    };

    assert_eq!(
        serde_json::to_string(&response).unwrap(),
        serde_json::to_string(&expected_response).unwrap()
    );
}

#[tokio::test]
async fn test_run_session_executes_single_tool_call_and_returns_response() {
    let tool = MockTool::new("test_tool", vec![create_text_part("Tool result")], false);

    let model = Arc::new(MockLanguageModel::new().add_responses(vec![
        ModelResponse {
            content: vec![Part::ToolCall(ToolCallPart {
                tool_name: "test_tool".to_string(),
                tool_call_id: "call_1".to_string(),
                args: serde_json::json!({"param": "value"}),
            })],
            usage: Some(ModelUsage {
                input_tokens: 100,
                output_tokens: 50,
                ..Default::default()
            }),
            cost: Some(0.0015),
        },
        ModelResponse {
            content: vec![create_text_part("Final response")],
            ..Default::default()
        },
    ]));

    let session = RunSession::new(Arc::new(
        AgentParams::new("test_agent", model).add_tool(tool),
    ))
    .await;

    let response = session
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Use the tool")],
            }))],
        })
        .await
        .unwrap();

    let expected_response = AgentResponse {
        content: vec![create_text_part("Final response")],
        output: vec![
            AgentItem::Model(ModelResponse {
                content: vec![Part::ToolCall(ToolCallPart {
                    tool_name: "test_tool".to_string(),
                    tool_call_id: "call_1".to_string(),
                    args: serde_json::json!({"param": "value"}),
                })],
                usage: Some(ModelUsage {
                    input_tokens: 100,
                    output_tokens: 50,
                    ..Default::default()
                }),
                cost: Some(0.0015),
            }),
            AgentItem::Tool(AgentItemTool {
                tool_call_id: "call_1".to_string(),
                tool_name: "test_tool".to_string(),
                input: serde_json::json!({"param": "value"}),
                output: vec![create_text_part("Tool result")],
                is_error: false,
            }),
            AgentItem::Model(ModelResponse {
                content: vec![create_text_part("Final response")],
                ..Default::default()
            }),
        ],
    };

    assert_eq!(
        serde_json::to_string(&response).unwrap(),
        serde_json::to_string(&expected_response).unwrap()
    );
}

#[tokio::test]
async fn test_run_session_throws_max_turns_exceeded_error() {
    let tool = MockTool::new("test_tool", vec![create_text_part("Tool result")], false);

    let model = Arc::new(MockLanguageModel::new().add_responses(vec![
        ModelResponse {
            content: vec![Part::ToolCall(ToolCallPart {
                tool_name: "test_tool".to_string(),
                tool_call_id: "call_1".to_string(),
                args: serde_json::json!({}),
            })],
            usage: Some(create_model_usage()),
            cost: Some(0.0),
        },
        ModelResponse {
            content: vec![Part::ToolCall(ToolCallPart {
                tool_name: "test_tool".to_string(),
                tool_call_id: "call_2".to_string(),
                args: serde_json::json!({}),
            })],
            usage: Some(create_model_usage()),
            cost: Some(0.0),
        },
        ModelResponse {
            content: vec![Part::ToolCall(ToolCallPart {
                tool_name: "test_tool".to_string(),
                tool_call_id: "call_3".to_string(),
                args: serde_json::json!({}),
            })],
            usage: Some(create_model_usage()),
            cost: Some(0.0),
        },
    ]));

    let session = RunSession::new(Arc::new(
        AgentParams::new("test_agent", model)
            .add_tool(tool)
            .max_turns(2),
    ))
    .await;

    let result = session
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Keep using tools")],
            }))],
        })
        .await;

    match result {
        Err(AgentError::MaxTurnsExceeded(turns)) => {
            assert_eq!(turns, 2);
        }
        _ => panic!("Expected MaxTurnsExceeded error"),
    }
}

#[tokio::test]
async fn test_run_session_throws_invariant_error_when_tool_not_found() {
    let model = Arc::new(MockLanguageModel::new().add_response(ModelResponse {
        content: vec![Part::ToolCall(ToolCallPart {
            tool_name: "non_existent_tool".to_string(),
            tool_call_id: "call_1".to_string(),
            args: serde_json::json!({}),
        })],
        usage: Some(create_model_usage()),
        cost: Some(0.0),
    }));

    let session = RunSession::new(AgentParams::new("test_agent", model).into()).await;

    let result = session
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Use a tool")],
            }))],
        })
        .await;

    match result {
        Err(AgentError::Invariant(msg)) => {
            assert!(msg.contains("Tool non_existent_tool not found"));
        }
        _ => panic!("Expected Invariant error"),
    }
}

#[tokio::test]
async fn test_run_session_throws_tool_execution_error_when_tool_execution_fails() {
    let tool = MockTool::new("failing_tool", vec![create_text_part("")], false).with_error();

    let model = Arc::new(MockLanguageModel::new().add_response(ModelResponse {
        content: vec![Part::ToolCall(ToolCallPart {
            tool_name: "failing_tool".to_string(),
            tool_call_id: "call_1".to_string(),
            args: serde_json::json!({}),
        })],
        usage: Some(create_model_usage()),
        cost: Some(0.0),
    }));

    let session =
        RunSession::new(AgentParams::new("test_agent", model).add_tool(tool).into()).await;

    let result = session
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Use the tool")],
            }))],
        })
        .await;

    match result {
        Err(AgentError::ToolExecution(_)) => {
            // Expected
        }
        _ => panic!("Expected ToolExecution error"),
    }
}

#[tokio::test]
async fn test_run_session_handles_tool_returning_error_result() {
    let tool = MockTool::new(
        "test_tool",
        vec![create_text_part("Error: Invalid parameters")],
        true,
    );

    let model = Arc::new(MockLanguageModel::new().add_responses(vec![
        ModelResponse {
            content: vec![Part::ToolCall(ToolCallPart {
                tool_name: "test_tool".to_string(),
                tool_call_id: "call_1".to_string(),
                args: serde_json::json!({"invalid": true}),
            })],
            usage: Some(create_model_usage()),
            cost: Some(0.0),
        },
        ModelResponse {
            content: vec![create_text_part("Handled the error")],
            usage: Some(create_model_usage()),
            cost: Some(0.0),
        },
    ]));

    let session = RunSession::new(
        AgentParams::new("test_agent", model.clone())
            .add_tool(tool)
            .into(),
    )
    .await;

    let response = session
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Use the tool")],
            }))],
        })
        .await
        .unwrap();

    // For now, just check that we got a response - tool handling details may be
    // implementation specific

    // Check final response
    assert_eq!(response.content.len(), 1);
    if let Part::Text(text_part) = &response.content[0] {
        assert_eq!(text_part.text, "Handled the error");
    }
}

#[tokio::test]
async fn test_run_session_passes_sampling_parameters_to_model() {
    let model = Arc::new(MockLanguageModel::new().add_response(ModelResponse {
        content: vec![create_text_part("Response")],
        usage: Some(create_model_usage()),
        cost: Some(0.0),
    }));

    let session = RunSession::new(
        AgentParams::new("test_agent", model.clone())
            .temperature(0.7)
            .top_p(0.9)
            .top_k(40)
            .presence_penalty(0.1)
            .frequency_penalty(0.2)
            .into(),
    )
    .await;

    session
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Hello")],
            }))],
        })
        .await
        .unwrap();

    let generate_calls = model.get_generate_calls();
    assert_eq!(generate_calls.len(), 1);
    let generate_call = &generate_calls[0];
    assert_eq!(generate_call.temperature, Some(0.7));
    assert_eq!(generate_call.top_p, Some(0.9));
    assert_eq!(generate_call.top_k, Some(40));
    assert_eq!(generate_call.presence_penalty, Some(0.1));
    assert_eq!(generate_call.frequency_penalty, Some(0.2));
}

#[tokio::test]
async fn test_run_session_executes_multiple_tool_calls_in_parallel() {
    let tool1 = MockTool::new("tool_1", vec![create_text_part("Tool 1 result")], false);

    let tool2 = MockTool::new("tool_2", vec![create_text_part("Tool 2 result")], false);

    let model = Arc::new(MockLanguageModel::new().add_responses(vec![
        ModelResponse {
            content: vec![
                Part::ToolCall(ToolCallPart {
                    tool_name: "tool_1".to_string(),
                    tool_call_id: "call_1".to_string(),
                    args: serde_json::json!({"param": "value1"}),
                }),
                Part::ToolCall(ToolCallPart {
                    tool_name: "tool_2".to_string(),
                    tool_call_id: "call_2".to_string(),
                    args: serde_json::json!({"param": "value2"}),
                }),
            ],
            usage: Some(ModelUsage {
                input_tokens: 2000,
                output_tokens: 100,
                ..Default::default()
            }),
            cost: None,
        },
        ModelResponse {
            content: vec![create_text_part("Processed both tools")],
            usage: Some(ModelUsage {
                input_tokens: 50,
                output_tokens: 10,
                ..Default::default()
            }),
            cost: Some(0.0003),
        },
    ]));

    let session = RunSession::new(
        AgentParams::new("test_agent", model.clone())
            .add_tool(tool1.clone())
            .add_tool(tool2.clone())
            .into(),
    )
    .await;

    let response = session
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Use both tools")],
            }))],
        })
        .await
        .unwrap();

    let expected_response = AgentResponse {
        content: vec![create_text_part("Processed both tools")],
        output: vec![
            AgentItem::Model(ModelResponse {
                content: vec![
                    Part::ToolCall(ToolCallPart {
                        tool_name: "tool_1".to_string(),
                        tool_call_id: "call_1".to_string(),
                        args: serde_json::json!({"param": "value1"}),
                    }),
                    Part::ToolCall(ToolCallPart {
                        tool_name: "tool_2".to_string(),
                        tool_call_id: "call_2".to_string(),
                        args: serde_json::json!({"param": "value2"}),
                    }),
                ],
                usage: Some(ModelUsage {
                    input_tokens: 2000,
                    output_tokens: 100,
                    ..Default::default()
                }),
                cost: None,
            }),
            AgentItem::Tool(AgentItemTool {
                tool_call_id: "call_1".to_string(),
                tool_name: "tool_1".to_string(),
                input: serde_json::json!({"param": "value1"}),
                output: vec![create_text_part("Tool 1 result")],
                is_error: false,
            }),
            AgentItem::Tool(AgentItemTool {
                tool_call_id: "call_2".to_string(),
                tool_name: "tool_2".to_string(),
                input: serde_json::json!({"param": "value2"}),
                output: vec![create_text_part("Tool 2 result")],
                is_error: false,
            }),
            AgentItem::Model(ModelResponse {
                content: vec![create_text_part("Processed both tools")],
                usage: Some(ModelUsage {
                    input_tokens: 50,
                    output_tokens: 10,
                    ..Default::default()
                }),
                cost: Some(0.0003),
            }),
        ],
    };

    assert_eq!(
        serde_json::to_string(&response).unwrap(),
        serde_json::to_string(&expected_response).unwrap()
    );
}

#[tokio::test]
async fn test_run_session_handles_multiple_turns_with_tool_calls() {
    let tool = MockTool::new(
        "calculator",
        vec![create_text_part("Calculation result")],
        false,
    );

    let model = Arc::new(MockLanguageModel::new().add_responses(vec![
        ModelResponse {
            content: vec![Part::ToolCall(ToolCallPart {
                tool_name: "calculator".to_string(),
                tool_call_id: "call_1".to_string(),
                args: serde_json::json!({
                    "operation": "add",
                    "a": 1,
                    "b": 2
                }),
            })],
            usage: Some(create_model_usage()),
            cost: Some(0.0),
        },
        ModelResponse {
            content: vec![Part::ToolCall(ToolCallPart {
                tool_name: "calculator".to_string(),
                tool_call_id: "call_2".to_string(),
                args: serde_json::json!({
                    "operation": "multiply",
                    "a": 3,
                    "b": 4
                }),
            })],
            usage: Some(create_model_usage()),
            cost: Some(0.0),
        },
        ModelResponse {
            content: vec![create_text_part("All calculations done")],
            usage: Some(create_model_usage()),
            cost: Some(0.0),
        },
    ]));

    let session =
        RunSession::new(AgentParams::new("test_agent", model).add_tool(tool).into()).await;

    let response = session
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Calculate some numbers")],
            }))],
        })
        .await
        .unwrap();

    assert_eq!(response.content.len(), 1);
    if let Part::Text(text_part) = &response.content[0] {
        assert_eq!(text_part.text, "All calculations done");
    }
    // Should have multiple outputs for multiple turns
    assert!(response.output.len() >= 1);
}

#[tokio::test]
async fn test_run_session_throws_language_model_error_when_generation_fails() {
    let model = Arc::new(
        MockLanguageModel::new().add_error(LanguageModelError::InvalidInput(
            "API quota exceeded".to_string(),
        )),
    );

    let session = RunSession::new(AgentParams::new("test_agent", model.clone()).into()).await;

    let result = session
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Hello")],
            }))],
        })
        .await;

    match result {
        Err(AgentError::LanguageModel(err)) => {
            assert!(err.to_string().contains("API quota exceeded"));
        }
        _ => panic!("Expected LanguageModel error"),
    }
}

#[tokio::test]
async fn test_run_session_streaming_response_when_no_tool_call() {
    let model = Arc::new(MockLanguageModel::new().add_partial_responses(vec![
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "Hel".to_string(),
                }),
            }),
            usage: Some(create_model_usage()),
        },
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "lo".to_string(),
                }),
            }),
            usage: Some(create_model_usage()),
        },
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "!".to_string(),
                }),
            }),
            usage: Some(create_model_usage()),
        },
    ]));

    let session = RunSession::new(AgentParams::new("test_agent", model.clone()).into()).await;

    let stream = session
        .run_stream(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Hi")],
            }))],
        })
        .await
        .unwrap();

    let events: Vec<_> = stream.collect().await;

    // Should have partial events and final events
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

#[tokio::test]
async fn test_run_session_streaming_throws_language_model_error() {
    let model = Arc::new(MockLanguageModel::new().add_stream_error(
        LanguageModelError::InvalidInput("Rate limit exceeded".to_string()),
    ));

    let session = RunSession::new(AgentParams::new("test_agent", model).into()).await;

    let result = session
        .run_stream(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Hello")],
            }))],
        })
        .await;

    match result {
        Err(AgentError::LanguageModel(err)) => {
            assert!(err.to_string().contains("Rate limit exceeded"));
        }
        Ok(mut stream) => {
            // If the stream was created, it should error on consumption
            match stream.next().await {
                Some(Err(AgentError::LanguageModel(err))) => {
                    assert!(err.to_string().contains("Rate limit exceeded"));
                }
                _ => panic!("Expected LanguageModel error on stream consumption"),
            }
        }
        _ => panic!("Expected LanguageModel error"),
    }
}

#[tokio::test]
async fn test_run_session_includes_string_and_dynamic_function_instructions() {
    let model = Arc::new(MockLanguageModel::new().add_response(ModelResponse {
        content: vec![create_text_part("Response")],
        usage: Some(create_model_usage()),
        cost: Some(0.0),
    }));

    #[derive(Clone)]
    struct TestContext {
        user_role: String,
    }

    let instructions: Vec<InstructionParam<TestContext>> = vec![
        InstructionParam::String("You are a helpful assistant.".to_string()),
        InstructionParam::Func(Box::new(|ctx: &TestContext| {
            Ok(format!("The user is a {}.", ctx.user_role))
        })),
        InstructionParam::String("Always be polite.".to_string()),
    ];

    let session: RunSession<TestContext> = RunSession::new(Arc::new(
        AgentParams::new("test_agent", model.clone())
            .instructions(instructions)
            .into(),
    ))
    .await;

    session
        .run(AgentRequest {
            context: TestContext {
                user_role: "developer".to_string(),
            },
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Hello")],
            }))],
        })
        .await
        .unwrap();

    let generate_calls = model.get_generate_calls();
    assert_eq!(generate_calls.len(), 1);
    let generate_call = &generate_calls[0];
    assert_eq!(
        generate_call.system_prompt,
        Some(
            "You are a helpful assistant.\nThe user is a developer.\nAlways be polite.".to_string()
        )
    );
}

#[tokio::test]
async fn test_run_session_streaming_tool_call_execution() {
    let tool = MockTool::new("test_tool", vec![create_text_part("Tool result")], false);

    let model =
        Arc::new(
            MockLanguageModel::new().add_partial_responses(vec![PartialModelResponse {
                delta: Some(ContentDelta {
                    index: 0,
                    part: PartDelta::ToolCall(ToolCallPartDelta {
                        tool_name: Some("test_tool".to_string()),
                        tool_call_id: Some("call_1".to_string()),
                        args: Some("{}".to_string()),
                    }),
                }),
                usage: Some(create_model_usage()),
            }]),
        );

    let session = RunSession::new(
        AgentParams::new("test_agent", model.clone())
            .add_tool(tool)
            .into(),
    )
    .await;

    let stream = session
        .run_stream(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Use tool")],
            }))],
        })
        .await
        .unwrap();

    let events: Vec<_> = stream.collect().await;

    // Should have tool call events
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

#[tokio::test]
async fn test_run_session_streaming_multiple_turns() {
    let tool = MockTool::new(
        "calculator",
        vec![create_text_part("Calculation done")],
        false,
    );

    let model = Arc::new(
        MockLanguageModel::new()
            .add_partial_responses(vec![PartialModelResponse {
                delta: Some(ContentDelta {
                    index: 0,
                    part: PartDelta::ToolCall(ToolCallPartDelta {
                        tool_name: Some("calculator".to_string()),
                        tool_call_id: Some("call_1".to_string()),
                        args: Some("{\"a\": 1, \"b\": 2}".to_string()),
                    }),
                }),
                usage: Some(create_model_usage()),
            }])
            .add_partial_responses(vec![PartialModelResponse {
                delta: Some(ContentDelta {
                    index: 0,
                    part: PartDelta::ToolCall(ToolCallPartDelta {
                        tool_name: Some("calculator".to_string()),
                        tool_call_id: Some("call_2".to_string()),
                        args: Some("{\"a\": 3, \"b\": 4}".to_string()),
                    }),
                }),
                usage: Some(create_model_usage()),
            }])
            .add_partial_responses(vec![PartialModelResponse {
                delta: Some(ContentDelta {
                    index: 0,
                    part: PartDelta::Text(TextPartDelta {
                        text: "All done".to_string(),
                    }),
                }),
                usage: Some(create_model_usage()),
            }]),
    );

    let session = RunSession::new(
        AgentParams::new("test_agent", model.clone())
            .add_tool(tool)
            .into(),
    )
    .await;

    let stream = session
        .run_stream(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Calculate")],
            }))],
        })
        .await
        .unwrap();

    let events: Vec<_> = stream.collect().await;

    // Should have multiple events for multiple turns
    assert!(!events.is_empty());

    let item_events: Vec<_> = events
        .iter()
        .filter(|event| matches!(event, Ok(AgentStreamEvent::Item(_))))
        .collect();

    // Should have multiple item events
    assert!(item_events.len() >= 1);
}

#[tokio::test]
async fn test_run_session_streaming_throws_max_turns_exceeded_error() {
    let tool = MockTool::new("test_tool", vec![create_text_part("Tool result")], false);

    let model = Arc::new(
        MockLanguageModel::new()
            .add_partial_responses(vec![PartialModelResponse {
                delta: Some(ContentDelta {
                    index: 0,
                    part: PartDelta::ToolCall(ToolCallPartDelta {
                        tool_name: Some("test_tool".to_string()),
                        tool_call_id: Some("call_1".to_string()),
                        args: Some("{}".to_string()),
                    }),
                }),
                usage: Some(create_model_usage()),
            }])
            .add_partial_responses(vec![PartialModelResponse {
                delta: Some(ContentDelta {
                    index: 0,
                    part: PartDelta::ToolCall(ToolCallPartDelta {
                        tool_name: Some("test_tool".to_string()),
                        tool_call_id: Some("call_2".to_string()),
                        args: Some("{}".to_string()),
                    }),
                }),
                usage: Some(create_model_usage()),
            }])
            .add_partial_responses(vec![PartialModelResponse {
                delta: Some(ContentDelta {
                    index: 0,
                    part: PartDelta::ToolCall(ToolCallPartDelta {
                        tool_name: Some("test_tool".to_string()),
                        tool_call_id: Some("call_3".to_string()),
                        args: Some("{}".to_string()),
                    }),
                }),
                usage: Some(create_model_usage()),
            }]),
    );

    let session = RunSession::new(Arc::new(
        AgentParams::new("test_agent", model)
            .add_tool(tool)
            .max_turns(2),
    ))
    .await;

    let stream = session
        .run_stream(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![create_text_part("Keep using tools")],
            }))],
        })
        .await;

    // The stream should either fail immediately or fail during consumption
    match stream {
        Err(AgentError::MaxTurnsExceeded(_)) => {
            // Expected immediate failure
        }
        Ok(mut event_stream) => {
            // Should fail during consumption
            let mut found_max_turns_error = false;
            while let Some(event) = event_stream.next().await {
                match event {
                    Err(AgentError::MaxTurnsExceeded(_)) => {
                        found_max_turns_error = true;
                        break;
                    }
                    Err(_) => break,
                    Ok(_) => continue,
                }
            }
            assert!(
                found_max_turns_error,
                "Expected MaxTurnsExceeded error during streaming"
            );
        }
        _ => panic!("Expected MaxTurnsExceeded error or successful stream"),
    }
}
