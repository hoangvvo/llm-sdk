use futures::{future::BoxFuture, TryStreamExt};
use llm_agent::{
    Agent, AgentError, AgentItem, AgentParams, AgentRequest, AgentResponse, AgentStreamEvent,
    AgentStreamItemEvent, AgentTool, BoxedError, Toolkit, ToolkitSession,
};
use llm_sdk::{
    llm_sdk_test::{MockGenerateResult, MockLanguageModel, MockStreamResult},
    ContentDelta, LanguageModelError, Message, ModelResponse, Part, PartDelta,
    PartialModelResponse, TextPartDelta,
};
use std::sync::{
    atomic::{AtomicUsize, Ordering},
    Arc,
};

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
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
        })
        .await
        .expect("agent run succeeds");

    let expected = AgentResponse {
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
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
        })
        .await;

    assert!(matches!(result, Err(AgentError::LanguageModel(_))));
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
        .run_stream(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
        })
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
        .run_stream(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
        })
        .await
        .expect("create agent stream");
    let result = stream.try_collect::<Vec<_>>().await;

    assert!(matches!(result, Err(AgentError::LanguageModel(_))));
    assert_eq!(close_calls.load(Ordering::SeqCst), 1);
}
