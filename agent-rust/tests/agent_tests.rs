use futures::{future::BoxFuture, StreamExt, TryStreamExt};
use llm_agent::AgentResponseStatus;
use llm_agent::RunOptions;
use llm_agent::{
    Agent, AgentError, AgentFunctionTool, AgentItem, AgentParams, AgentRequest, AgentResponse,
    AgentStreamEvent, AgentStreamItemEvent, AgentTool, AgentToolResult, BoxedError,
    InstructionParam, RunState, Toolkit, ToolkitSession,
};
use llm_sdk::{
    llm_sdk_test::{MockGenerateResult, MockLanguageModel, MockStreamResult},
    AudioFormat, AudioOptions, ContentDelta, FunctionTool, JSONSchema, LanguageModelError, Message,
    Modality, ModelResponse, Part, PartDelta, PartialModelResponse, ReasoningOptions,
    ResponseFormatJson, ResponseFormatOption, TextPartDelta, Tool, WebSearchTool,
};
use serde_json::{json, Value};
use std::sync::{
    atomic::{AtomicUsize, Ordering},
    Arc,
};

struct LookupTool;

impl AgentFunctionTool<TestContext> for LookupTool {
    fn name(&self) -> String {
        "lookup".to_string()
    }

    fn description(&self) -> String {
        "Look up a record".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        json!({
            "type": "object",
            "properties": {"query": {"type": "string"}},
            "required": ["query"],
            "additionalProperties": false,
        })
    }

    fn execute<'a>(
        &'a self,
        _args: Value,
        _context: &'a TestContext,
        _state: &'a RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, BoxedError>> {
        Box::pin(async {
            Ok(AgentToolResult {
                content: vec![],
                is_error: false,
            })
        })
    }
}

#[derive(Clone)]
struct TestContext {
    tenant: String,
}

struct CloseTrackingToolkit {
    close_calls: Arc<AtomicUsize>,
}

impl Toolkit<()> for CloseTrackingToolkit {
    fn create_session<'a>(
        &'a self,
        _context: &'a (),
    ) -> BoxFuture<'a, Result<Box<dyn ToolkitSession<()> + Send + Sync>, BoxedError>> {
        let close_calls = self.close_calls.clone();
        Box::pin(async move {
            Ok(Box::new(CloseTrackingToolkitSession { close_calls })
                as Box<dyn ToolkitSession<()> + Send + Sync>)
        })
    }
}

struct CloseTrackingToolkitSession {
    close_calls: Arc<AtomicUsize>,
}

struct FailingCloseToolkit {
    close_calls: Arc<AtomicUsize>,
}

impl Toolkit<()> for FailingCloseToolkit {
    fn create_session<'a>(
        &'a self,
        _context: &'a (),
    ) -> BoxFuture<'a, Result<Box<dyn ToolkitSession<()> + Send + Sync>, BoxedError>> {
        let close_calls = self.close_calls.clone();
        Box::pin(async move {
            Ok(Box::new(FailingCloseToolkitSession { close_calls })
                as Box<dyn ToolkitSession<()> + Send + Sync>)
        })
    }
}

struct FailingCloseToolkitSession {
    close_calls: Arc<AtomicUsize>,
}

impl ToolkitSession<()> for FailingCloseToolkitSession {
    fn system_prompt(&self) -> Option<String> {
        None
    }

    fn tools(&self) -> Vec<AgentTool<()>> {
        Vec::new()
    }

    fn close(self: Box<Self>) -> BoxFuture<'static, Result<(), BoxedError>> {
        Box::pin(async move {
            self.close_calls.fetch_add(1, Ordering::SeqCst);
            Err(std::io::Error::other("cleanup failed").into())
        })
    }
}

impl ToolkitSession<()> for CloseTrackingToolkitSession {
    fn system_prompt(&self) -> Option<String> {
        None
    }

    fn tools(&self) -> Vec<AgentTool<()>> {
        Vec::new()
    }

    fn close(self: Box<Self>) -> BoxFuture<'static, Result<(), BoxedError>> {
        Box::pin(async move {
            self.close_calls.fetch_add(1, Ordering::SeqCst);
            Ok(())
        })
    }
}

#[tokio::test]
async fn agent_run_creates_session_runs_and_finishes() {
    let close_calls = Arc::new(AtomicUsize::new(0));
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Mock response")],
        ..Default::default()
    });

    let agent = Agent::new(AgentParams::new("test-agent", model.clone()).add_toolkit(
        CloseTrackingToolkit {
            close_calls: close_calls.clone(),
        },
    ));

    let response = agent
        .run(
            AgentRequest {
                context: (),
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("agent run succeeds");

    let expected = AgentResponse {
        status: AgentResponseStatus::Completed,
        content: vec![Part::text("Mock response")],
        output: vec![AgentItem::Model(ModelResponse {
            content: vec![Part::text("Mock response")],
            ..Default::default()
        })],
    };

    assert_eq!(response, expected);
    assert_eq!(close_calls.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn agent_run_closes_session_when_generation_fails() {
    let close_calls = Arc::new(AtomicUsize::new(0));
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(MockGenerateResult::error(LanguageModelError::InvalidInput(
        "generation failed".to_string(),
    )));
    let agent = Agent::new(AgentParams::new("test-agent", model).add_toolkit(
        CloseTrackingToolkit {
            close_calls: close_calls.clone(),
        },
    ));

    let result = agent
        .run(
            AgentRequest {
                context: (),
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default(),
        )
        .await;

    assert!(matches!(result, Err(AgentError::LanguageModel { .. })));
    assert_eq!(close_calls.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn agent_run_preserves_model_error_when_cleanup_fails() {
    let close_calls = Arc::new(AtomicUsize::new(0));
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(MockGenerateResult::error(LanguageModelError::InvalidInput(
        "generation failed".to_string(),
    )));
    let agent = Agent::new(AgentParams::new("test-agent", model).add_toolkit(
        FailingCloseToolkit {
            close_calls: close_calls.clone(),
        },
    ));

    let result = agent
        .run(
            AgentRequest {
                context: (),
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default(),
        )
        .await;

    assert!(matches!(result, Err(AgentError::LanguageModel { .. })));
    assert_eq!(close_calls.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn agent_run_returns_cleanup_error_when_session_cleanup_fails() {
    let close_calls = Arc::new(AtomicUsize::new(0));
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("done")],
        ..Default::default()
    });
    let agent = Agent::new(AgentParams::new("test-agent", model).add_toolkit(
        FailingCloseToolkit {
            close_calls: close_calls.clone(),
        },
    ));

    let error = agent
        .run(
            AgentRequest {
                context: (),
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default(),
        )
        .await
        .expect_err("cleanup failure should fail the run");

    assert!(matches!(&error, AgentError::Cleanup { .. }));
    assert!(error.snapshot().is_none());
    assert_eq!(close_calls.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn agent_run_stream_creates_session_streams_and_finishes() {
    let close_calls = Arc::new(AtomicUsize::new(0));
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(MockStreamResult::partials(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::Text(TextPartDelta::new("Mock")),
        }),
        ..Default::default()
    }]));

    let agent = Agent::new(AgentParams::new("test-agent", model.clone()).add_toolkit(
        CloseTrackingToolkit {
            close_calls: close_calls.clone(),
        },
    ));

    let stream = agent
        .run_stream(
            AgentRequest {
                context: (),
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("agent run_stream succeeds");

    let events = stream
        .map_err(|err| err.to_string())
        .try_collect::<Vec<_>>()
        .await
        .expect("collect stream");

    let expected = vec![
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("Mock")),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Item(AgentStreamItemEvent {
            index: 0,
            item: AgentItem::Model(ModelResponse {
                content: vec![Part::text("Mock")],
                ..Default::default()
            }),
        }),
        AgentStreamEvent::Response(AgentResponse {
            status: AgentResponseStatus::Completed,
            content: vec![Part::text("Mock")],
            output: vec![AgentItem::Model(ModelResponse {
                content: vec![Part::text("Mock")],
                ..Default::default()
            })],
        }),
    ];

    assert_eq!(events, expected);
    assert_eq!(close_calls.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn agent_run_stream_closes_session_when_streaming_fails() {
    let close_calls = Arc::new(AtomicUsize::new(0));
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(MockStreamResult::error(LanguageModelError::InvalidInput(
        "stream failed".to_string(),
    )));
    let agent = Agent::new(AgentParams::new("test-agent", model).add_toolkit(
        CloseTrackingToolkit {
            close_calls: close_calls.clone(),
        },
    ));

    let stream = agent
        .run_stream(
            AgentRequest {
                context: (),
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("create agent stream");
    let result = stream.try_collect::<Vec<_>>().await;

    assert!(matches!(result, Err(AgentError::LanguageModel { .. })));
    assert_eq!(close_calls.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn agent_run_stream_emits_cleanup_error_instead_of_response() {
    let close_calls = Arc::new(AtomicUsize::new(0));
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(MockStreamResult::partials(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::Text(TextPartDelta::new("done")),
        }),
        ..Default::default()
    }]));
    let agent = Agent::new(AgentParams::new("test-agent", model).add_toolkit(
        FailingCloseToolkit {
            close_calls: close_calls.clone(),
        },
    ));
    let mut stream = agent
        .run_stream(
            AgentRequest {
                context: (),
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("create agent stream");

    let mut events = Vec::new();
    let error = loop {
        match stream.next().await {
            Some(Ok(event)) => events.push(event),
            Some(Err(error)) => break error,
            None => panic!("stream ended without cleanup error"),
        }
    };
    assert!(matches!(&error, AgentError::Cleanup { .. }));
    assert!(error.snapshot().is_none());
    assert!(matches!(
        events.as_slice(),
        [AgentStreamEvent::Partial(_), AgentStreamEvent::Item(_)]
    ));
    assert_eq!(close_calls.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn agent_builder_forwards_complete_public_configuration() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("configured")],
        ..Default::default()
    });
    let response_format = ResponseFormatOption::Json(ResponseFormatJson {
        name: "answer".to_string(),
        description: Some("A configured answer".to_string()),
        schema: Some(json!({
            "type": "object",
            "properties": {"answer": {"type": "string"}},
            "required": ["answer"],
            "additionalProperties": false,
        })),
    });
    let audio = AudioOptions {
        format: Some(AudioFormat::Mp3),
        voice: Some("alloy".to_string()),
        language: Some("en".to_string()),
    };
    let reasoning = ReasoningOptions {
        enabled: true,
        budget_tokens: Some(256),
    };
    let web_search = WebSearchTool {
        allowed_domains: Some(vec!["example.com".to_string()]),
        ..Default::default()
    };
    let agent = Agent::builder("configured-agent", model.clone())
        .instructions(vec![
            InstructionParam::String("Static".to_string()),
            InstructionParam::Func(Box::new(|context: &TestContext| {
                Ok(format!("Tenant: {}", context.tenant))
            })),
        ])
        .add_tool(LookupTool)
        .add_tool(web_search.clone())
        .response_format(response_format.clone())
        .max_turns(3)
        .temperature(0.2)
        .top_p(0.8)
        .top_k(12)
        .presence_penalty(0.1)
        .frequency_penalty(0.3)
        .modalities(vec![Modality::Text, Modality::Audio])
        .audio(audio.clone())
        .reasoning(reasoning.clone())
        .build();

    agent
        .run(
            AgentRequest {
                context: TestContext {
                    tenant: "acme".to_string(),
                },
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Configure this",
                )]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("agent run should succeed");

    let inputs = model.tracked_generate_inputs();
    assert_eq!(inputs.len(), 1);
    let input = &inputs[0];
    assert_eq!(input.system_prompt.as_deref(), Some("Static\nTenant: acme"));
    assert_eq!(
        input.messages,
        vec![Message::user(vec![Part::text("Configure this")])]
    );
    assert_eq!(
        input.tools,
        Some(vec![
            Tool::Function(FunctionTool::new(
                "lookup",
                "Look up a record",
                LookupTool.parameters(),
            )),
            Tool::WebSearch(web_search),
        ])
    );
    assert_eq!(input.response_format, Some(response_format));
    assert_eq!(input.temperature, Some(0.2));
    assert_eq!(input.top_p, Some(0.8));
    assert_eq!(input.top_k, Some(12));
    assert_eq!(input.presence_penalty, Some(0.1));
    assert_eq!(input.frequency_penalty, Some(0.3));
    assert_eq!(
        input.modalities,
        Some(vec![Modality::Text, Modality::Audio])
    );
    assert_eq!(input.audio, Some(audio));
    assert_eq!(input.reasoning, Some(reasoning));
}
