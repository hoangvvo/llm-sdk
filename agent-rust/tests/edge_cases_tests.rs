use futures::{future::BoxFuture, StreamExt};
use llm_agent::{
    AgentError, AgentFunctionTool, AgentItem, AgentParams, AgentToolResult, BoxedError, RunSession,
    RunSessionRequest, RunState,
};
use llm_sdk::{
    llm_sdk_test::{MockLanguageModel, MockStreamResult},
    ContentDelta, JSONSchema, Message, ModelResponse, Part, PartDelta, PartialModelResponse,
    ReasoningPartDelta, TextPartDelta,
};
use serde_json::{json, Value};
use std::sync::{
    atomic::{AtomicUsize, Ordering},
    Arc, Mutex,
};

struct CountingTool {
    name: &'static str,
    executions: Arc<AtomicUsize>,
}

impl AgentFunctionTool<()> for CountingTool {
    fn name(&self) -> String {
        self.name.to_string()
    }

    fn description(&self) -> String {
        self.name.to_string()
    }

    fn parameters(&self) -> JSONSchema {
        json!({"type": "object", "properties": {}})
    }

    fn execute<'a>(
        &'a self,
        _args: Value,
        _context: &'a (),
        _state: &'a RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, BoxedError>> {
        self.executions.fetch_add(1, Ordering::SeqCst);
        Box::pin(async {
            Ok(AgentToolResult {
                content: vec![],
                is_error: false,
            })
        })
    }
}

#[tokio::test]
async fn run_rejects_empty_input_without_calling_model() {
    let model = Arc::new(MockLanguageModel::new());
    let session = RunSession::new(Arc::new(AgentParams::new("test_agent", model.clone())), ())
        .await
        .expect("session should initialize");

    let result = session.run(RunSessionRequest { input: vec![] }).await;

    assert!(matches!(result, Err(AgentError::Invariant(_))));
    assert!(model.tracked_generate_inputs().is_empty());
}

#[tokio::test]
async fn run_rejects_duplicate_tool_call_ids_before_execution() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![
            Part::tool_call("duplicate", "first", json!({})),
            Part::tool_call("duplicate", "second", json!({})),
        ],
        ..Default::default()
    });
    let executions = Arc::new(AtomicUsize::new(0));
    let session = RunSession::new(
        Arc::new(
            AgentParams::new("test_agent", model)
                .add_tool(CountingTool {
                    name: "first",
                    executions: executions.clone(),
                })
                .add_tool(CountingTool {
                    name: "second",
                    executions: executions.clone(),
                }),
        ),
        (),
    )
    .await
    .expect("session should initialize");

    let result = session
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::user(vec![Part::text(
                "Use tools",
            )]))],
        })
        .await;

    match result {
        Err(AgentError::Invariant(message)) => {
            assert!(message.contains("Duplicate tool call ID: duplicate"));
        }
        other => panic!("expected duplicate-ID invariant, got {other:?}"),
    }
    assert_eq!(executions.load(Ordering::SeqCst), 0);
}

#[derive(Clone, Debug, PartialEq)]
struct Observation {
    turn: usize,
    item_types: Vec<&'static str>,
}

struct StateInspectingTool {
    observation: Arc<Mutex<Option<Observation>>>,
}

impl AgentFunctionTool<()> for StateInspectingTool {
    fn name(&self) -> String {
        "inspect_state".to_string()
    }

    fn description(&self) -> String {
        "Inspect run state".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        json!({"type": "object", "properties": {}})
    }

    fn execute<'a>(
        &'a self,
        _args: Value,
        _context: &'a (),
        state: &'a RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, BoxedError>> {
        Box::pin(async move {
            let item_types = state
                .items()
                .await
                .iter()
                .map(|item| match item {
                    AgentItem::Message(_) => "message",
                    AgentItem::Model(_) => "model",
                    AgentItem::Tool(_) => "tool",
                })
                .collect();
            let turn = *state.current_turn.lock().await;
            *self.observation.lock().expect("observation lock") =
                Some(Observation { turn, item_types });
            Ok(AgentToolResult {
                content: vec![Part::text("inspected")],
                is_error: false,
            })
        })
    }
}

#[tokio::test]
async fn run_passes_current_turn_and_accumulated_items_to_tool() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call("call_1", "inspect_state", json!({}))],
        ..Default::default()
    });
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("done")],
        ..Default::default()
    });
    let observation = Arc::new(Mutex::new(None));
    let session = RunSession::new(
        Arc::new(
            AgentParams::new("test_agent", model).add_tool(StateInspectingTool {
                observation: observation.clone(),
            }),
        ),
        (),
    )
    .await
    .expect("session should initialize");

    session
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::user(vec![Part::text(
                "Inspect",
            )]))],
        })
        .await
        .expect("run should succeed");

    assert_eq!(
        *observation.lock().expect("observation lock"),
        Some(Observation {
            turn: 1,
            item_types: vec!["message", "model"],
        })
    );
}

#[tokio::test]
async fn run_stream_invalid_delta_sequence_returns_invariant_error() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(MockStreamResult::partials(vec![
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("hello")),
            }),
            ..Default::default()
        },
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Reasoning(ReasoningPartDelta::default().with_text("wrong type")),
            }),
            ..Default::default()
        },
    ]));
    let session = Arc::new(
        RunSession::new(Arc::new(AgentParams::new("test_agent", model)), ())
            .await
            .expect("session should initialize"),
    );
    let mut stream = session
        .run_stream(RunSessionRequest {
            input: vec![AgentItem::Message(Message::user(vec![Part::text(
                "Stream",
            )]))],
        })
        .expect("stream should initialize");

    let mut error = None;
    while let Some(event) = stream.next().await {
        if let Err(stream_error) = event {
            error = Some(stream_error);
            break;
        }
    }
    assert!(matches!(error, Some(AgentError::Invariant(_))));
}
