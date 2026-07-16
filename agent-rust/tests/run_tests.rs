use llm_agent::AgentResponseStatus;
use llm_agent::RunOptions;
use llm_sdk::ToolResultStatus;
use std::sync::{
    atomic::{AtomicUsize, Ordering},
    Arc, Mutex,
};
use tokio::sync::Notify;
use tokio_util::sync::CancellationToken;

type DynError = Box<dyn std::error::Error + Send + Sync>;
use futures::{future::BoxFuture, StreamExt, TryStreamExt};
use llm_agent::{
    AgentError, AgentFunctionTool, AgentItem, AgentItemTool, AgentParams, AgentResponse,
    AgentStreamEvent, AgentStreamItemEvent, AgentTool, AgentToolResult, InstructionParam,
    RunSession, RunSessionRequest, RunState, Toolkit, ToolkitSession,
};
use llm_sdk::{
    llm_sdk_test::{MockGenerateResult, MockLanguageModel, MockStreamResult},
    ContentDelta, JSONSchema, LanguageModelError, Message, ModelResponse, ModelUsage, Part,
    PartDelta, PartialModelResponse, ReasoningPartDelta, TextPartDelta, Tool, ToolCallPartDelta,
    WebSearchTool,
};
use serde_json::{json, Value};

type ExecuteFn = dyn for<'a> Fn(Value, &'a RunState) -> Result<AgentToolResult, DynError>
    + Send
    + Sync
    + 'static;

fn mixed_snapshot_partials() -> Vec<PartialModelResponse> {
    vec![
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("partial text")),
            }),
            ..Default::default()
        },
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 1,
                part: PartDelta::ToolCall(
                    ToolCallPartDelta::default()
                        .with_tool_call_id("call_1")
                        .with_tool_name("weather")
                        .with_args(r#"{"city":"Paris"}"#),
                ),
            }),
            ..Default::default()
        },
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 2,
                part: PartDelta::ToolCall(ToolCallPartDelta::default().with_args("{incomplete")),
            }),
            ..Default::default()
        },
    ]
}

fn mixed_snapshot_model_response() -> ModelResponse {
    ModelResponse {
        content: vec![
            Part::text("partial text"),
            Part::tool_call("call_1", "weather", json!({"city": "Paris"})),
        ],
        ..Default::default()
    }
}

#[derive(Clone)]
struct MockTool {
    name: String,
    execute: Arc<ExecuteFn>,
    all_calls: Arc<std::sync::Mutex<Vec<Value>>>,
}

impl MockTool {
    fn new(name: &str, result: AgentToolResult) -> Self {
        let execute: Arc<ExecuteFn> = Arc::new({
            let result = Arc::new(result);
            move |_, _| Ok((*result).clone())
        });

        Self {
            name: name.to_string(),
            execute,
            all_calls: Arc::new(std::sync::Mutex::new(Vec::new())),
        }
    }

    fn with_execute_fn<F>(name: &str, execute: F) -> Self
    where
        F: for<'a> Fn(Value, &'a RunState) -> Result<AgentToolResult, DynError>
            + Send
            + Sync
            + 'static,
    {
        Self {
            name: name.to_string(),
            execute: Arc::new(execute),
            all_calls: Arc::new(std::sync::Mutex::new(Vec::new())),
        }
    }

    fn recorded_calls(&self) -> Vec<Value> {
        self.all_calls.lock().unwrap().clone()
    }
}

struct MockToolkitSessionState<TCtx> {
    system_prompt: Option<String>,
    tools: Vec<AgentTool<TCtx>>,
    system_prompt_calls: std::sync::Mutex<usize>,
    tools_calls: std::sync::Mutex<usize>,
    close_calls: std::sync::Mutex<usize>,
}

impl<TCtx> MockToolkitSessionState<TCtx> {
    fn new(system_prompt: Option<String>, tools: Vec<AgentTool<TCtx>>) -> Arc<Self> {
        Arc::new(Self {
            system_prompt,
            tools,
            system_prompt_calls: std::sync::Mutex::new(0),
            tools_calls: std::sync::Mutex::new(0),
            close_calls: std::sync::Mutex::new(0),
        })
    }

    fn close_calls(&self) -> usize {
        *self.close_calls.lock().unwrap()
    }
}

struct MockToolkitSession<TCtx> {
    state: Arc<MockToolkitSessionState<TCtx>>,
}

impl<TCtx> ToolkitSession<TCtx> for MockToolkitSession<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    fn system_prompt(&self) -> Option<String> {
        let mut calls = self.state.system_prompt_calls.lock().unwrap();
        *calls += 1;
        self.state.system_prompt.clone()
    }

    fn tools(&self) -> Vec<AgentTool<TCtx>> {
        let mut calls = self.state.tools_calls.lock().unwrap();
        *calls += 1;
        self.state.tools.clone()
    }

    fn close(self: Box<Self>) -> BoxFuture<'static, Result<(), DynError>> {
        Box::pin(async move {
            let mut calls = self.state.close_calls.lock().unwrap();
            *calls += 1;
            Ok(())
        })
    }
}

struct MockToolkit<TCtx> {
    state: Arc<MockToolkitSessionState<TCtx>>,
    created_contexts: Arc<std::sync::Mutex<Vec<TCtx>>>,
}

impl<TCtx> MockToolkit<TCtx>
where
    TCtx: Clone,
{
    fn new(state: Arc<MockToolkitSessionState<TCtx>>) -> Self {
        Self {
            state,
            created_contexts: Arc::new(std::sync::Mutex::new(Vec::new())),
        }
    }
}

impl<TCtx> Toolkit<TCtx> for MockToolkit<TCtx>
where
    TCtx: Send + Sync + Clone + 'static,
{
    fn create_session<'a>(
        &'a self,
        context: &'a TCtx,
    ) -> BoxFuture<'a, Result<Box<dyn ToolkitSession<TCtx> + Send + Sync>, DynError>> {
        Box::pin(async move {
            self.created_contexts.lock().unwrap().push(context.clone());
            let boxed: Box<dyn ToolkitSession<TCtx> + Send + Sync> = Box::new(MockToolkitSession {
                state: self.state.clone(),
            });
            Ok(boxed)
        })
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
struct CustomerContext {
    customer: String,
}

#[derive(Clone, Debug, PartialEq)]
struct OrderExecution {
    context: CustomerContext,
    args: Value,
    turn: usize,
}

#[derive(Clone, Default)]
struct LookupOrderTool {
    executions: Arc<std::sync::Mutex<Vec<OrderExecution>>>,
}

impl LookupOrderTool {
    fn new() -> Self {
        Self::default()
    }

    fn executions(&self) -> Vec<OrderExecution> {
        self.executions.lock().unwrap().clone()
    }
}

impl AgentFunctionTool<CustomerContext> for LookupOrderTool {
    fn name(&self) -> String {
        "lookup-order".to_string()
    }

    fn description(&self) -> String {
        "Lookup an order by ID".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        json!({
            "type": "object",
            "properties": {
                "orderId": { "type": "string" }
            },
            "required": ["orderId"],
            "additionalProperties": false
        })
    }

    fn execute<'a>(
        &'a self,
        args: Value,
        context: &'a CustomerContext,
        state: &'a RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, DynError>> {
        Box::pin(async move {
            let order_id = args
                .get("orderId")
                .and_then(Value::as_str)
                .ok_or_else(|| "missing orderId".to_string())?
                .to_string();

            let turn = *state.current_turn.lock().await;
            self.executions.lock().unwrap().push(OrderExecution {
                context: context.clone(),
                args: args.clone(),
                turn,
            });

            let text = format!("Order {order_id} ready for {}", context.customer);
            Ok(AgentToolResult {
                content: vec![Part::text(text)],
                is_error: false,
            })
        })
    }
}

impl AgentFunctionTool<()> for MockTool {
    fn name(&self) -> String {
        self.name.clone()
    }

    fn description(&self) -> String {
        format!("Mock tool {}", self.name)
    }

    fn parameters(&self) -> JSONSchema {
        json!({"type": "object", "properties": {}})
    }

    fn execute<'a>(
        &'a self,
        args: Value,
        _context: &(),
        state: &'a RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>>> {
        Box::pin(async move {
            self.all_calls.lock().unwrap().push(args.clone());
            (self.execute)(args, state)
        })
    }
}

struct WaitingTool {
    started: Arc<Notify>,
}

impl AgentFunctionTool<()> for WaitingTool {
    fn name(&self) -> String {
        "wait".to_string()
    }

    fn description(&self) -> String {
        "wait".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        json!({"type": "object", "properties": {}})
    }

    fn execute<'a>(
        &'a self,
        _args: Value,
        _context: &'a (),
        state: &'a RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, DynError>> {
        Box::pin(async move {
            self.started.notify_one();
            state.cancellation_token().cancelled().await;
            Err(std::io::Error::from(std::io::ErrorKind::Interrupted).into())
        })
    }
}

struct NonCooperativeTool {
    started: Arc<Notify>,
    release: Arc<Notify>,
}

impl AgentFunctionTool<()> for NonCooperativeTool {
    fn name(&self) -> String {
        "first".to_string()
    }

    fn description(&self) -> String {
        "first".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        json!({"type": "object", "properties": {}})
    }

    fn execute<'a>(
        &'a self,
        _args: Value,
        _context: &'a (),
        _state: &'a RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, DynError>> {
        Box::pin(async move {
            self.started.notify_one();
            self.release.notified().await;
            Ok(AgentToolResult {
                content: vec![Part::text("first finished")],
                is_error: false,
            })
        })
    }
}

#[derive(Clone, Debug, PartialEq)]
struct RunStateObservation {
    turn: usize,
    item_types: Vec<&'static str>,
}

struct StateInspectingTool {
    observation: Arc<Mutex<Option<RunStateObservation>>>,
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
    ) -> BoxFuture<'a, Result<AgentToolResult, DynError>> {
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
                Some(RunStateObservation { turn, item_types });
            Ok(AgentToolResult {
                content: vec![Part::text("inspected")],
                is_error: false,
            })
        })
    }
}

struct ClosingToolkit {
    create_error: Option<&'static str>,
    close_error: Option<&'static str>,
    close_calls: Arc<AtomicUsize>,
}

impl Toolkit<()> for ClosingToolkit {
    fn create_session<'a>(
        &'a self,
        _context: &'a (),
    ) -> BoxFuture<'a, Result<Box<dyn ToolkitSession<()> + Send + Sync>, DynError>> {
        Box::pin(async move {
            if let Some(message) = self.create_error {
                return Err(std::io::Error::other(message).into());
            }

            Ok(Box::new(ClosingToolkitSession {
                close_error: self.close_error,
                close_calls: self.close_calls.clone(),
            }) as Box<dyn ToolkitSession<()> + Send + Sync>)
        })
    }
}

struct ClosingToolkitSession {
    close_error: Option<&'static str>,
    close_calls: Arc<AtomicUsize>,
}

impl ToolkitSession<()> for ClosingToolkitSession {
    fn system_prompt(&self) -> Option<String> {
        None
    }

    fn tools(&self) -> Vec<AgentTool<()>> {
        Vec::new()
    }

    fn close(self: Box<Self>) -> BoxFuture<'static, Result<(), DynError>> {
        Box::pin(async move {
            self.close_calls.fetch_add(1, Ordering::SeqCst);
            if let Some(message) = self.close_error {
                return Err(std::io::Error::other(message).into());
            }
            Ok(())
        })
    }
}

fn closing_toolkit(
    create_error: Option<&'static str>,
    close_error: Option<&'static str>,
    close_calls: &Arc<AtomicUsize>,
) -> ClosingToolkit {
    ClosingToolkit {
        create_error,
        close_error,
        close_calls: close_calls.clone(),
    }
}

async fn new_run_session<TCtx>(
    params: Arc<AgentParams<TCtx>>,
    context: TCtx,
) -> Arc<RunSession<TCtx>>
where
    TCtx: Send + Sync + 'static,
{
    Arc::new(
        RunSession::new(params, context)
            .await
            .expect("failed to create run session"),
    )
}

async fn close_run_session<TCtx>(session: Arc<RunSession<TCtx>>)
where
    TCtx: Send + Sync + 'static,
{
    match Arc::try_unwrap(session) {
        Ok(run_session) => {
            run_session.close().await.expect("close session succeeds");
        }
        Err(session) => panic!(
            "session should not be shared at close (strong count: {})",
            Arc::strong_count(&session)
        ),
    }
}

#[tokio::test]
async fn run_rejects_empty_input_without_calling_model() {
    let model = Arc::new(MockLanguageModel::new());
    let session = RunSession::new(Arc::new(AgentParams::new("test_agent", model.clone())), ())
        .await
        .expect("session should initialize");

    let result = session
        .run(RunSessionRequest { input: vec![] }, RunOptions::default())
        .await;

    assert!(matches!(result, Err(AgentError::Invariant { .. })));
    assert!(model.tracked_generate_inputs().is_empty());
}

#[tokio::test]
async fn run_returns_cancelled_without_calling_model_when_token_already_cancelled() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("ignored")],
        ..Default::default()
    });
    let session = RunSession::new(Arc::new(AgentParams::new("test_agent", model.clone())), ())
        .await
        .expect("session should initialize");
    let cancellation_token = CancellationToken::new();
    cancellation_token.cancel();

    let response = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default().with_cancellation_token(cancellation_token),
        )
        .await
        .expect("cancellation should return a response");

    assert_eq!(response.status, AgentResponseStatus::Cancelled);
    assert!(response.content.is_empty());
    assert!(response.output.is_empty());
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
    let first = MockTool::new(
        "first",
        AgentToolResult {
            content: Vec::new(),
            is_error: false,
        },
    );
    let second = MockTool::new(
        "second",
        AgentToolResult {
            content: Vec::new(),
            is_error: false,
        },
    );
    let session = RunSession::new(
        Arc::new(
            AgentParams::new("test_agent", model)
                .add_tool(first.clone())
                .add_tool(second.clone()),
        ),
        (),
    )
    .await
    .expect("session should initialize");

    let result = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Use tools",
                )]))],
            },
            RunOptions::default(),
        )
        .await;

    match result {
        Err(AgentError::Invariant { message, .. }) => {
            assert!(message.contains("Duplicate tool call ID: duplicate"));
        }
        other => panic!("expected duplicate-ID invariant, got {other:?}"),
    }
    assert!(first.recorded_calls().is_empty());
    assert!(second.recorded_calls().is_empty());
}

#[tokio::test]
async fn run_records_cancelled_tool_results_for_the_next_run() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![
            Part::tool_call("call_1", "wait", json!({})),
            Part::tool_call("call_2", "wait", json!({})),
        ],
        ..Default::default()
    });
    let started = Arc::new(Notify::new());
    let session = RunSession::new(
        Arc::new(
            AgentParams::new("test_agent", model.clone()).add_tool(WaitingTool {
                started: started.clone(),
            }),
        ),
        (),
    )
    .await
    .expect("session should initialize");
    let cancellation_token = CancellationToken::new();
    let initial = AgentItem::Message(Message::user(vec![Part::text("Wait")]));
    let run = session.run(
        RunSessionRequest {
            input: vec![initial.clone()],
        },
        RunOptions::default().with_cancellation_token(cancellation_token.clone()),
    );
    tokio::pin!(run);

    tokio::select! {
        () = started.notified() => {}
        result = &mut run => panic!("run ended before cancellation: {result:?}"),
    }
    cancellation_token.cancel();
    let response = run.await.expect("cancellation should return a response");

    assert_eq!(response.status, AgentResponseStatus::Cancelled);
    assert_eq!(response.output.len(), 3);
    for item in &response.output[1..] {
        let AgentItem::Tool(tool) = item else {
            panic!("expected cancelled tool item");
        };
        assert_eq!(tool.status, ToolResultStatus::Cancelled);
        assert!(tool.output.is_empty());
    }

    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("continued")],
        ..Default::default()
    });
    let mut next_input = vec![initial];
    next_input.extend(response.output);
    next_input.push(AgentItem::Message(Message::user(vec![Part::text(
        "Continue",
    )])));
    let next_session = RunSession::new(Arc::new(AgentParams::new("test_agent", model.clone())), ())
        .await
        .expect("next session should initialize");
    next_session
        .run(
            RunSessionRequest { input: next_input },
            RunOptions::default(),
        )
        .await
        .expect("next run should succeed");

    let inputs = model.tracked_generate_inputs();
    let Message::Tool(tool_message) = &inputs[1].messages[2] else {
        panic!("expected tool results before the next user message");
    };
    assert_eq!(tool_message.content.len(), 2);
    for part in &tool_message.content {
        let Part::ToolResult(result) = part else {
            panic!("expected tool result");
        };
        assert_eq!(result.status, ToolResultStatus::Cancelled);
        assert!(result.content.is_empty());
    }
}

#[tokio::test]
async fn run_does_not_start_later_tools_after_a_non_cooperative_tool_finishes() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![
            Part::tool_call("call_1", "first", json!({})),
            Part::tool_call("call_2", "second", json!({})),
        ],
        ..Default::default()
    });
    let started = Arc::new(Notify::new());
    let release = Arc::new(Notify::new());
    let second = MockTool::new(
        "second",
        AgentToolResult {
            content: vec![Part::text("second finished")],
            is_error: false,
        },
    );
    let session = RunSession::new(
        Arc::new(
            AgentParams::new("test_agent", model)
                .add_tool(NonCooperativeTool {
                    started: started.clone(),
                    release: release.clone(),
                })
                .add_tool(second.clone()),
        ),
        (),
    )
    .await
    .expect("session should initialize");
    let cancellation_token = CancellationToken::new();
    let run = session.run(
        RunSessionRequest {
            input: vec![AgentItem::Message(Message::user(vec![Part::text(
                "Run both tools",
            )]))],
        },
        RunOptions::default().with_cancellation_token(cancellation_token.clone()),
    );
    tokio::pin!(run);

    tokio::select! {
        () = started.notified() => {}
        result = &mut run => panic!("run ended before cancellation: {result:?}"),
    }
    cancellation_token.cancel();
    release.notify_one();
    let response = run.await.expect("cancellation should return a response");

    assert_eq!(response.status, AgentResponseStatus::Cancelled);
    assert_eq!(response.output.len(), 3);
    let AgentItem::Tool(first_item) = &response.output[1] else {
        panic!("expected completed first tool item");
    };
    assert_eq!(first_item.status, ToolResultStatus::Completed);
    let AgentItem::Tool(second_item) = &response.output[2] else {
        panic!("expected cancelled second tool item");
    };
    assert_eq!(second_item.status, ToolResultStatus::Cancelled);
    assert!(second.recorded_calls().is_empty());
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
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Inspect",
                )]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("run should succeed");

    assert_eq!(
        *observation.lock().expect("observation lock"),
        Some(RunStateObservation {
            turn: 1,
            item_types: vec!["message", "model"],
        })
    );
}

#[tokio::test]
async fn run_stream_returns_invariant_error_for_invalid_delta_sequence() {
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
        .run_stream(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Stream",
                )]))],
            },
            RunOptions::default(),
        )
        .expect("stream should initialize");

    let mut error = None;
    while let Some(event) = stream.next().await {
        if let Err(stream_error) = event {
            error = Some(stream_error);
            break;
        }
    }
    assert!(matches!(error, Some(AgentError::Invariant { .. })));
}

#[tokio::test]
async fn run_session_closes_initialized_toolkits_when_creation_fails() {
    let initialized_close_calls = Arc::new(AtomicUsize::new(0));
    let failed_close_calls = Arc::new(AtomicUsize::new(0));
    let model = Arc::new(MockLanguageModel::new());
    let params = AgentParams::new("test-agent", model)
        .add_toolkit(closing_toolkit(None, None, &initialized_close_calls))
        .add_toolkit(closing_toolkit(
            Some("second toolkit failed"),
            None,
            &failed_close_calls,
        ));

    let result = RunSession::new(Arc::new(params), ()).await;

    assert!(matches!(result, Err(AgentError::Init { .. })));
    assert_eq!(initialized_close_calls.load(Ordering::SeqCst), 1);
    assert_eq!(failed_close_calls.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn run_session_close_attempts_every_toolkit_and_reports_failure() {
    let failing_close_calls = Arc::new(AtomicUsize::new(0));
    let successful_close_calls = Arc::new(AtomicUsize::new(0));
    let model = Arc::new(MockLanguageModel::new());
    let params = AgentParams::new("test-agent", model)
        .add_toolkit(closing_toolkit(
            None,
            Some("cleanup failed"),
            &failing_close_calls,
        ))
        .add_toolkit(closing_toolkit(None, None, &successful_close_calls));
    let session = RunSession::new(Arc::new(params), ())
        .await
        .expect("session initializes");

    let result = session.close().await;

    assert!(matches!(result, Err(AgentError::Cleanup { .. })));
    assert_eq!(failing_close_calls.load(Ordering::SeqCst), 1);
    assert_eq!(successful_close_calls.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn run_returns_response_when_no_tool_call() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Hi!")],
        ..Default::default()
    });

    let session = new_run_session(Arc::new(AgentParams::new("test_agent", model)), ()).await;

    let response = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Hello!",
                )]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("run succeeds");

    let expected = AgentResponse {
        status: AgentResponseStatus::Completed,
        content: vec![Part::text("Hi!")],
        output: vec![AgentItem::Model(ModelResponse {
            content: vec![Part::text("Hi!")],
            ..Default::default()
        })],
    };

    assert_eq!(response, expected);

    close_run_session(session).await;
}

#[tokio::test]
async fn run_executes_single_tool_call_and_returns_response() {
    let tool = MockTool::new(
        "test_tool",
        AgentToolResult {
            content: vec![Part::text("Tool result")],
            is_error: false,
        },
    );

    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call(
            "call_1",
            "test_tool",
            json!({"param": "value"}),
        )],
        usage: Some(ModelUsage::default()),
        cost: Some(0.0015),
    });
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Final response")],
        ..Default::default()
    });

    let session = new_run_session(
        Arc::new(AgentParams::new("test_agent", model).add_tool(tool.clone())),
        (),
    )
    .await;

    let response = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Use the tool",
                )]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("run succeeds");

    let expected = AgentResponse {
        status: AgentResponseStatus::Completed,
        content: vec![Part::text("Final response")],
        output: vec![
            AgentItem::Model(ModelResponse {
                content: vec![Part::tool_call(
                    "call_1",
                    "test_tool",
                    json!({"param": "value"}),
                )],
                usage: Some(ModelUsage::default()),
                cost: Some(0.0015),
            }),
            AgentItem::Tool(AgentItemTool {
                tool_call_id: "call_1".to_string(),
                tool_name: "test_tool".to_string(),
                input: json!({"param": "value"}),
                output: vec![Part::text("Tool result")],
                status: ToolResultStatus::Completed,
            }),
            AgentItem::Model(ModelResponse {
                content: vec![Part::text("Final response")],
                ..Default::default()
            }),
        ],
    };

    assert_eq!(response, expected);

    let calls = tool.recorded_calls();
    assert_eq!(calls, vec![json!({"param": "value"})]);
}

#[tokio::test]
async fn run_executes_multiple_tool_calls_from_one_model_response() {
    let tool1 = MockTool::new(
        "tool_1",
        AgentToolResult {
            content: vec![Part::text("Tool 1 result")],
            is_error: false,
        },
    );
    let tool2 = MockTool::new(
        "tool_2",
        AgentToolResult {
            content: vec![Part::text("Tool 2 result")],
            is_error: false,
        },
    );

    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![
            Part::tool_call("call_1", "tool_1", json!({"param": "value1"})),
            Part::tool_call("call_2", "tool_2", json!({"param": "value2"})),
        ],
        usage: Some(ModelUsage::default()),
        cost: None,
    });
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Processed both tools")],
        usage: Some(ModelUsage::default()),
        cost: Some(0.0003),
    });

    let session = new_run_session(
        Arc::new(
            AgentParams::new("test_agent", model)
                .add_tool(tool1.clone())
                .add_tool(tool2.clone()),
        ),
        (),
    )
    .await;

    let response = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Use both tools",
                )]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("run succeeds");

    let expected = AgentResponse {
        status: AgentResponseStatus::Completed,
        content: vec![Part::text("Processed both tools")],
        output: vec![
            AgentItem::Model(ModelResponse {
                content: vec![
                    Part::tool_call("call_1", "tool_1", json!({"param": "value1"})),
                    Part::tool_call("call_2", "tool_2", json!({"param": "value2"})),
                ],
                usage: Some(ModelUsage::default()),
                cost: None,
            }),
            AgentItem::Tool(AgentItemTool {
                tool_call_id: "call_1".to_string(),
                tool_name: "tool_1".to_string(),
                input: json!({"param": "value1"}),
                output: vec![Part::text("Tool 1 result")],
                status: ToolResultStatus::Completed,
            }),
            AgentItem::Tool(AgentItemTool {
                tool_call_id: "call_2".to_string(),
                tool_name: "tool_2".to_string(),
                input: json!({"param": "value2"}),
                output: vec![Part::text("Tool 2 result")],
                status: ToolResultStatus::Completed,
            }),
            AgentItem::Model(ModelResponse {
                content: vec![Part::text("Processed both tools")],
                usage: Some(ModelUsage::default()),
                cost: Some(0.0003),
            }),
        ],
    };

    assert_eq!(response, expected);
    assert_eq!(tool1.recorded_calls(), vec![json!({"param": "value1"})]);
    assert_eq!(tool2.recorded_calls(), vec![json!({"param": "value2"})]);
}

#[tokio::test]
async fn run_returns_existing_assistant_response_without_new_model_output() {
    let model = Arc::new(MockLanguageModel::new());

    let session = new_run_session(Arc::new(AgentParams::new("cached", model.clone())), ()).await;

    let response = session
        .run(
            RunSessionRequest {
                input: vec![
                    AgentItem::Message(Message::user(vec![Part::text("What did I say?")])),
                    AgentItem::Message(Message::assistant(vec![Part::text("Cached answer")])),
                ],
            },
            RunOptions::default(),
        )
        .await
        .expect("run succeeds");

    assert_eq!(
        response,
        AgentResponse {
            status: AgentResponseStatus::Completed,
            content: vec![Part::text("Cached answer")],
            output: vec![],
        }
    );
    assert!(model.tracked_generate_inputs().is_empty());

    close_run_session(session).await;
}

#[tokio::test]
async fn run_resumes_tool_processing_from_tool_message_with_partial_results() {
    let resume_tool = MockTool::with_execute_fn("resume_tool", |_args, _state| {
        Ok(AgentToolResult {
            content: vec![Part::text("call_2 result")],
            is_error: false,
        })
    });

    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Final reply")],
        ..Default::default()
    });

    let session = new_run_session(
        Arc::new(AgentParams::new("resumable", model.clone()).add_tool(resume_tool.clone())),
        (),
    )
    .await;

    let response = session
        .run(
            RunSessionRequest {
                input: vec![
                    AgentItem::Message(Message::user(vec![Part::text("Continue")])),
                    AgentItem::Model(ModelResponse {
                        content: vec![
                            Part::tool_call("call_1", "resume_tool", json!({"step": 1})),
                            Part::tool_call("call_2", "resume_tool", json!({"step": 2})),
                        ],
                        ..Default::default()
                    }),
                    AgentItem::Message(Message::tool(vec![Part::tool_result(
                        "call_1",
                        "resume_tool",
                        vec![Part::text("already done")],
                    )])),
                ],
            },
            RunOptions::default(),
        )
        .await
        .expect("run succeeds");

    assert_eq!(resume_tool.recorded_calls(), vec![json!({"step": 2})]);

    assert_eq!(
        response,
        AgentResponse {
            status: AgentResponseStatus::Completed,
            content: vec![Part::text("Final reply")],
            output: vec![
                AgentItem::Tool(AgentItemTool {
                    tool_call_id: "call_2".to_string(),
                    tool_name: "resume_tool".to_string(),
                    input: json!({"step": 2}),
                    output: vec![Part::text("call_2 result")],
                    status: ToolResultStatus::Completed,
                }),
                AgentItem::Model(ModelResponse {
                    content: vec![Part::text("Final reply")],
                    ..Default::default()
                }),
            ],
        }
    );

    close_run_session(session).await;
}

#[tokio::test]
async fn run_resumes_tool_processing_when_trailing_items_are_tool_entries() {
    let resume_tool = MockTool::with_execute_fn("resume_tool", |_args, _state| {
        Ok(AgentToolResult {
            content: vec![Part::text("call_2 via item")],
            is_error: false,
        })
    });

    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Final reply from items")],
        ..Default::default()
    });

    let session = new_run_session(
        Arc::new(
            AgentParams::new("resumable_tool_items", model.clone()).add_tool(resume_tool.clone()),
        ),
        (),
    )
    .await;

    let response = session
        .run(
            RunSessionRequest {
                input: vec![
                    AgentItem::Message(Message::user(vec![Part::text("Continue")])),
                    AgentItem::Model(ModelResponse {
                        content: vec![
                            Part::tool_call("call_1", "resume_tool", json!({"stage": 1})),
                            Part::tool_call("call_2", "resume_tool", json!({"stage": 2})),
                        ],
                        ..Default::default()
                    }),
                    AgentItem::Tool(AgentItemTool {
                        tool_call_id: "call_1".to_string(),
                        tool_name: "resume_tool".to_string(),
                        input: json!({"stage": 1}),
                        output: vec![Part::text("already done")],
                        status: ToolResultStatus::Completed,
                    }),
                ],
            },
            RunOptions::default(),
        )
        .await
        .expect("run succeeds");

    assert_eq!(resume_tool.recorded_calls(), vec![json!({"stage": 2})]);

    assert_eq!(
        response,
        AgentResponse {
            status: AgentResponseStatus::Completed,
            content: vec![Part::text("Final reply from items")],
            output: vec![
                AgentItem::Tool(AgentItemTool {
                    tool_call_id: "call_2".to_string(),
                    tool_name: "resume_tool".to_string(),
                    input: json!({"stage": 2}),
                    output: vec![Part::text("call_2 via item")],
                    status: ToolResultStatus::Completed,
                }),
                AgentItem::Model(ModelResponse {
                    content: vec![Part::text("Final reply from items")],
                    ..Default::default()
                }),
            ],
        }
    );

    close_run_session(session).await;
}

#[tokio::test]
async fn run_errors_when_tool_results_lack_preceding_assistant_content() {
    let resume_tool = MockTool::new(
        "resume_tool",
        AgentToolResult {
            content: vec![Part::text("unused")],
            is_error: false,
        },
    );

    let model = Arc::new(MockLanguageModel::new());

    let session = new_run_session(
        Arc::new(AgentParams::new("resumable_error", model).add_tool(resume_tool.clone())),
        (),
    )
    .await;

    let err = session
        .run(
            RunSessionRequest {
                input: vec![
                    AgentItem::Message(Message::user(vec![Part::text("Resume")])),
                    AgentItem::Message(Message::tool(vec![Part::tool_result(
                        "call_1",
                        "resume_tool",
                        vec![Part::text("orphan")],
                    )])),
                ],
            },
            RunOptions::default(),
        )
        .await
        .expect_err("run fails");

    match err {
        AgentError::Invariant { message: msg, .. } => {
            assert!(msg.contains("Expected a model item or assistant message"));
        }
        other => panic!("unexpected error: {other:?}"),
    }

    close_run_session(session).await;
}

#[tokio::test]
async fn run_handles_multiple_turns_with_tool_calls() {
    let tool = MockTool::new(
        "calculator",
        AgentToolResult {
            content: vec![Part::text("Calculation result")],
            is_error: false,
        },
    );

    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call(
            "call_1",
            "calculator",
            json!({"operation": "add", "a": 1, "b": 2}),
        )],
        ..Default::default()
    });
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call(
            "call_2",
            "calculator",
            json!({"operation": "multiply", "a": 3, "b": 4}),
        )],
        ..Default::default()
    });
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("All calculations done")],
        ..Default::default()
    });

    let session = new_run_session(
        Arc::new(AgentParams::new("test_agent", model).add_tool(tool.clone())),
        (),
    )
    .await;

    let response = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Calculate some numbers",
                )]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("run succeeds");

    let expected = AgentResponse {
        status: AgentResponseStatus::Completed,
        content: vec![Part::text("All calculations done")],
        output: vec![
            AgentItem::Model(ModelResponse {
                content: vec![Part::tool_call(
                    "call_1",
                    "calculator",
                    json!({"operation": "add", "a": 1, "b": 2}),
                )],
                ..Default::default()
            }),
            AgentItem::Tool(AgentItemTool {
                tool_call_id: "call_1".to_string(),
                tool_name: "calculator".to_string(),
                input: json!({"operation": "add", "a": 1, "b": 2}),
                output: vec![Part::text("Calculation result")],
                status: ToolResultStatus::Completed,
            }),
            AgentItem::Model(ModelResponse {
                content: vec![Part::tool_call(
                    "call_2",
                    "calculator",
                    json!({"operation": "multiply", "a": 3, "b": 4}),
                )],
                ..Default::default()
            }),
            AgentItem::Tool(AgentItemTool {
                tool_call_id: "call_2".to_string(),
                tool_name: "calculator".to_string(),
                input: json!({"operation": "multiply", "a": 3, "b": 4}),
                output: vec![Part::text("Calculation result")],
                status: ToolResultStatus::Completed,
            }),
            AgentItem::Model(ModelResponse {
                content: vec![Part::text("All calculations done")],
                ..Default::default()
            }),
        ],
    };

    assert_eq!(response, expected);
    assert_eq!(
        tool.recorded_calls(),
        vec![
            json!({"operation": "add", "a": 1, "b": 2}),
            json!({"operation": "multiply", "a": 3, "b": 4}),
        ]
    );
}

#[tokio::test]
async fn run_throws_max_turns_exceeded_error() {
    let tool = MockTool::new(
        "test_tool",
        AgentToolResult {
            content: vec![Part::text("Tool result")],
            is_error: false,
        },
    );

    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call("call_1", "test_tool", json!({}))],
        ..Default::default()
    });
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call("call_2", "test_tool", json!({}))],
        ..Default::default()
    });
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call("call_3", "test_tool", json!({}))],
        ..Default::default()
    });

    let session = new_run_session(
        Arc::new(
            AgentParams::new("test_agent", model)
                .add_tool(tool)
                .max_turns(2),
        ),
        (),
    )
    .await;

    let result = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Keep using tools",
                )]))],
            },
            RunOptions::default(),
        )
        .await;

    match result {
        Err(AgentError::MaxTurnsExceeded { max_turns, .. }) => assert_eq!(max_turns, 2),
        other => panic!("expected max turns exceeded error, got {other:?}"),
    }
}

#[tokio::test]
async fn run_throws_invariant_error_when_tool_not_found() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call("call_1", "non_existent_tool", json!({}))],
        ..Default::default()
    });

    let session = new_run_session(Arc::new(AgentParams::new("test_agent", model)), ()).await;

    let result = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Use a tool",
                )]))],
            },
            RunOptions::default(),
        )
        .await;

    match result {
        Err(AgentError::Invariant { message, .. }) => {
            assert!(message.contains("Tool non_existent_tool not found"));
        }
        other => panic!("expected invariant error, got {other:?}"),
    }
}

#[tokio::test]
async fn run_throws_tool_execution_error() {
    let failing_tool = MockTool::with_execute_fn("failing_tool", |_args, _state| {
        Err("Tool execution failed".into())
    });

    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call("call_1", "failing_tool", json!({}))],
        ..Default::default()
    });

    let session = new_run_session(
        Arc::new(AgentParams::new("test_agent", model).add_tool(failing_tool)),
        (),
    )
    .await;

    let result = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Use the tool",
                )]))],
            },
            RunOptions::default(),
        )
        .await;

    assert!(matches!(result, Err(AgentError::ToolExecution { .. })));
}

#[tokio::test]
async fn run_handles_tool_returning_error_result() {
    let tool = MockTool::new(
        "test_tool",
        AgentToolResult {
            content: vec![Part::text("Error: Invalid parameters")],
            is_error: true,
        },
    );

    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call(
            "call_1",
            "test_tool",
            json!({"invalid": true}),
        )],
        ..Default::default()
    });
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Handled the error")],
        ..Default::default()
    });

    let session = new_run_session(
        Arc::new(AgentParams::new("test_agent", model).add_tool(tool.clone())),
        (),
    )
    .await;

    let response = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Use the tool",
                )]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("run succeeds");

    let expected = AgentResponse {
        status: AgentResponseStatus::Completed,
        content: vec![Part::text("Handled the error")],
        output: vec![
            AgentItem::Model(ModelResponse {
                content: vec![Part::tool_call(
                    "call_1",
                    "test_tool",
                    json!({"invalid": true}),
                )],
                ..Default::default()
            }),
            AgentItem::Tool(AgentItemTool {
                tool_call_id: "call_1".to_string(),
                tool_name: "test_tool".to_string(),
                input: json!({"invalid": true}),
                output: vec![Part::text("Error: Invalid parameters")],
                status: ToolResultStatus::Failed,
            }),
            AgentItem::Model(ModelResponse {
                content: vec![Part::text("Handled the error")],
                ..Default::default()
            }),
        ],
    };

    assert_eq!(response, expected);
    assert_eq!(tool.recorded_calls(), vec![json!({"invalid": true})]);
}

#[tokio::test]
async fn run_passes_sampling_parameters_to_model() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Response")],
        ..Default::default()
    });

    let session = new_run_session(
        Arc::new(
            AgentParams::new("test_agent", model.clone())
                .temperature(0.7)
                .top_p(0.9)
                .top_k(40)
                .presence_penalty(0.1)
                .frequency_penalty(0.2),
        ),
        (),
    )
    .await;

    session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("run succeeds");

    let inputs = model.tracked_generate_inputs();
    assert_eq!(inputs.len(), 1);
    let input = &inputs[0];
    assert_eq!(input.temperature, Some(0.7));
    assert_eq!(input.top_p, Some(0.9));
    assert_eq!(input.top_k, Some(40));
    assert_eq!(input.presence_penalty, Some(0.1));
    assert_eq!(input.frequency_penalty, Some(0.2));
}

#[tokio::test]
async fn run_passes_provider_hosted_tools_to_model() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Search complete")],
        ..Default::default()
    });
    let web_search = WebSearchTool {
        allowed_domains: Some(vec!["example.com".to_string()]),
        ..Default::default()
    };
    let session = new_run_session(
        Arc::new(AgentParams::new("test_agent", model.clone()).add_tool(web_search.clone())),
        (),
    )
    .await;

    session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Find an example",
                )]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("run succeeds");

    let inputs = model.tracked_generate_inputs();
    assert_eq!(inputs.len(), 1);
    assert_eq!(inputs[0].tools, Some(vec![Tool::WebSearch(web_search)]));
}

#[tokio::test]
async fn run_throws_language_model_error_when_generation_fails() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(MockGenerateResult::error(LanguageModelError::InvalidInput(
        "API quota exceeded".to_string(),
    )));

    let session = new_run_session(Arc::new(AgentParams::new("test_agent", model)), ()).await;

    let result = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default(),
        )
        .await;

    match result {
        Err(AgentError::LanguageModel { source: err, .. }) => {
            assert!(err.to_string().contains("API quota exceeded"));
        }
        other => panic!("expected language model error, got {other:?}"),
    }
}

#[tokio::test]
async fn run_includes_string_and_dynamic_function_instructions() {
    #[derive(Clone)]
    struct TestContext {
        user_role: String,
    }

    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Response")],
        ..Default::default()
    });

    let params = Arc::new(
        AgentParams::new("test_agent", model.clone()).instructions(vec![
            InstructionParam::String("You are a helpful assistant.".to_string()),
            InstructionParam::Func(Box::new(|ctx: &TestContext| {
                Ok(format!("The user is a {}.", ctx.user_role))
            })),
            InstructionParam::String("Always be polite.".to_string()),
        ]),
    );

    let session = new_run_session(
        params,
        TestContext {
            user_role: "developer".to_string(),
        },
    )
    .await;

    session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("run succeeds");

    let inputs = model.tracked_generate_inputs();
    assert_eq!(inputs.len(), 1);
    assert_eq!(
        inputs[0].system_prompt.as_deref(),
        Some("You are a helpful assistant.\nThe user is a developer.\nAlways be polite."),
    );

    close_run_session(session).await;
}

#[tokio::test]
async fn run_merges_toolkit_prompts_and_tools() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call(
            "call-1",
            "lookup-order",
            json!({"orderId": "123"}),
        )],
        ..Default::default()
    });
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Order ready")],
        ..Default::default()
    });

    let lookup_tool = LookupOrderTool::new();
    let toolkit_session_state = MockToolkitSessionState::new(
        Some("Toolkit prompt".to_string()),
        vec![AgentTool::function(lookup_tool.clone())],
    );
    let toolkit = MockToolkit::new(toolkit_session_state.clone());
    let contexts_handle = toolkit.created_contexts.clone();

    let context = CustomerContext {
        customer: "Ada".to_string(),
    };

    let session = new_run_session(
        Arc::new(AgentParams::new("toolkit-agent", model.clone()).add_toolkit(toolkit)),
        context.clone(),
    )
    .await;

    let response = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Status?",
                )]))],
            },
            RunOptions::default(),
        )
        .await
        .expect("run succeeds");

    assert_eq!(
        contexts_handle.lock().unwrap().clone(),
        vec![context.clone()]
    );

    let executions = lookup_tool.executions();
    assert_eq!(executions.len(), 1);
    assert_eq!(executions[0].context, context);
    assert_eq!(executions[0].args, json!({"orderId": "123"}));
    assert_eq!(executions[0].turn, 1);

    let inputs = model.tracked_generate_inputs();
    assert_eq!(inputs.len(), 2);
    for input in inputs {
        assert_eq!(input.system_prompt, Some("Toolkit prompt".to_string()));
        let tools = input.tools.expect("tools present");
        assert_eq!(tools.len(), 1);
        assert!(matches!(
            &tools[0],
            Tool::Function(tool) if tool.name == "lookup-order"
        ));
    }

    let expected = AgentResponse {
        status: AgentResponseStatus::Completed,
        content: vec![Part::text("Order ready")],
        output: vec![
            AgentItem::Model(ModelResponse {
                content: vec![Part::tool_call(
                    "call-1",
                    "lookup-order",
                    json!({"orderId": "123"}),
                )],
                ..Default::default()
            }),
            AgentItem::Tool(AgentItemTool {
                tool_call_id: "call-1".to_string(),
                tool_name: "lookup-order".to_string(),
                input: json!({"orderId": "123"}),
                output: vec![Part::text("Order 123 ready for Ada")],
                status: ToolResultStatus::Completed,
            }),
            AgentItem::Model(ModelResponse {
                content: vec![Part::text("Order ready")],
                ..Default::default()
            }),
        ],
    };

    assert_eq!(response, expected);

    close_run_session(session).await;

    assert_eq!(toolkit_session_state.close_calls(), 1);
}

#[tokio::test]
async fn run_stream_returns_cancelled_without_calling_model_when_token_already_cancelled() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::Text(TextPartDelta::new("ignored".to_string())),
        }),
        ..Default::default()
    }]);
    let session = RunSession::new(Arc::new(AgentParams::new("test_agent", model.clone())), ())
        .await
        .expect("session should initialize");
    let cancellation_token = CancellationToken::new();
    cancellation_token.cancel();

    let events = session
        .run_stream(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default().with_cancellation_token(cancellation_token),
        )
        .expect("run_stream succeeds")
        .map_err(|err| err.to_string())
        .try_collect::<Vec<_>>()
        .await
        .expect("collect stream");

    assert_eq!(
        events,
        vec![AgentStreamEvent::Response(AgentResponse {
            content: Vec::new(),
            output: Vec::new(),
            status: AgentResponseStatus::Cancelled,
        })]
    );
    assert!(model.tracked_stream_inputs().is_empty());
}

#[tokio::test]
async fn run_stream_streams_response_when_no_tool_call() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(vec![
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("Hel".to_string())),
            }),
            ..Default::default()
        },
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("lo".to_string())),
            }),
            ..Default::default()
        },
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("!".to_string())),
            }),
            ..Default::default()
        },
    ]);

    let session =
        new_run_session(Arc::new(AgentParams::new("test_agent", model.clone())), ()).await;

    let stream = session
        .clone()
        .run_stream(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hi")]))],
            },
            RunOptions::default(),
        )
        .expect("run_stream succeeds");

    let events = stream
        .map_err(|err| err.to_string())
        .try_collect::<Vec<_>>()
        .await
        .expect("collect stream");

    let expected = vec![
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("Hel".to_string())),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("lo".to_string())),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("!".to_string())),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Item(AgentStreamItemEvent {
            index: 0,
            item: AgentItem::Model(ModelResponse {
                content: vec![Part::text("Hello!")],
                ..Default::default()
            }),
        }),
        AgentStreamEvent::Response(AgentResponse {
            status: AgentResponseStatus::Completed,
            content: vec![Part::text("Hello!")],
            output: vec![AgentItem::Model(ModelResponse {
                content: vec![Part::text("Hello!")],
                ..Default::default()
            })],
        }),
    ];

    assert_eq!(events, expected);

    close_run_session(session).await;
}

#[tokio::test]
async fn run_stream_merges_toolkit_prompts_and_tools() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::Text(TextPartDelta::new("Done".to_string())),
        }),
        ..Default::default()
    }]);

    let lookup_tool = LookupOrderTool::new();
    let toolkit_session_state = MockToolkitSessionState::new(
        Some("Streaming toolkit prompt".to_string()),
        vec![AgentTool::function(lookup_tool.clone())],
    );
    let toolkit = MockToolkit::new(toolkit_session_state.clone());
    let contexts_handle = toolkit.created_contexts.clone();

    let context = CustomerContext {
        customer: "Ben".to_string(),
    };

    let session = new_run_session(
        Arc::new(AgentParams::new("toolkit-stream-agent", model.clone()).add_toolkit(toolkit)),
        context.clone(),
    )
    .await;

    let stream = session
        .clone()
        .run_stream(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default(),
        )
        .expect("run_stream succeeds");

    let events = stream
        .map_err(|err| err.to_string())
        .try_collect::<Vec<_>>()
        .await
        .expect("collect stream");

    let expected = vec![
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("Done".to_string())),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Item(AgentStreamItemEvent {
            index: 0,
            item: AgentItem::Model(ModelResponse {
                content: vec![Part::text("Done")],
                ..Default::default()
            }),
        }),
        AgentStreamEvent::Response(AgentResponse {
            status: AgentResponseStatus::Completed,
            content: vec![Part::text("Done")],
            output: vec![AgentItem::Model(ModelResponse {
                content: vec![Part::text("Done")],
                ..Default::default()
            })],
        }),
    ];

    assert_eq!(events, expected);
    assert_eq!(contexts_handle.lock().unwrap().clone(), vec![context]);
    assert!(lookup_tool.executions().is_empty());

    let inputs = model.tracked_stream_inputs();
    assert_eq!(inputs.len(), 1);
    let input = &inputs[0];
    assert_eq!(
        input.system_prompt,
        Some("Streaming toolkit prompt".to_string())
    );
    let tools = input.tools.clone().expect("tools present");
    assert_eq!(tools.len(), 1);
    assert!(matches!(
        &tools[0],
        Tool::Function(tool) if tool.name == "lookup-order"
    ));

    close_run_session(session).await;

    assert_eq!(toolkit_session_state.close_calls(), 1);
}

#[tokio::test]
#[allow(clippy::too_many_lines)]
async fn run_stream_streams_tool_call_execution_and_response() {
    let tool = MockTool::new(
        "test_tool",
        AgentToolResult {
            content: vec![Part::text("Tool result")],
            is_error: false,
        },
    );

    let tool_args = json!({"a": 1, "b": 2, "operation": "add"});

    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::ToolCall(
                ToolCallPartDelta::default()
                    .with_tool_call_id("call_1".to_string())
                    .with_tool_name("test_tool".to_string())
                    .with_args(tool_args.to_string()),
            ),
        }),
        ..Default::default()
    }]);
    model.enqueue_stream(vec![
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("Final".to_string())),
            }),
            ..Default::default()
        },
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new(" response".to_string())),
            }),
            ..Default::default()
        },
    ]);

    let session = new_run_session(
        Arc::new(AgentParams::new("test_agent", model).add_tool(tool.clone())),
        (),
    )
    .await;

    let stream = session
        .clone()
        .run_stream(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Use tool",
                )]))],
            },
            RunOptions::default(),
        )
        .expect("run_stream succeeds");

    let events = stream
        .map_err(|err| err.to_string())
        .try_collect::<Vec<_>>()
        .await
        .expect("collect stream");

    let expected = vec![
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::ToolCall(
                    ToolCallPartDelta::default()
                        .with_tool_call_id("call_1".to_string())
                        .with_tool_name("test_tool".to_string())
                        .with_args(tool_args.to_string()),
                ),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Item(AgentStreamItemEvent {
            index: 0,
            item: AgentItem::Model(ModelResponse {
                content: vec![Part::tool_call("call_1", "test_tool", tool_args.clone())],
                ..Default::default()
            }),
        }),
        AgentStreamEvent::Item(AgentStreamItemEvent {
            index: 1,
            item: AgentItem::Tool(AgentItemTool {
                tool_call_id: "call_1".to_string(),
                tool_name: "test_tool".to_string(),
                input: tool_args.clone(),
                output: vec![Part::text("Tool result")],
                status: ToolResultStatus::Completed,
            }),
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("Final".to_string())),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new(" response".to_string())),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Item(AgentStreamItemEvent {
            index: 2,
            item: AgentItem::Model(ModelResponse {
                content: vec![Part::text("Final response")],
                ..Default::default()
            }),
        }),
        AgentStreamEvent::Response(AgentResponse {
            status: AgentResponseStatus::Completed,
            content: vec![Part::text("Final response")],
            output: vec![
                AgentItem::Model(ModelResponse {
                    content: vec![Part::tool_call("call_1", "test_tool", tool_args.clone())],
                    ..Default::default()
                }),
                AgentItem::Tool(AgentItemTool {
                    tool_call_id: "call_1".to_string(),
                    tool_name: "test_tool".to_string(),
                    input: tool_args.clone(),
                    output: vec![Part::text("Tool result")],
                    status: ToolResultStatus::Completed,
                }),
                AgentItem::Model(ModelResponse {
                    content: vec![Part::text("Final response")],
                    ..Default::default()
                }),
            ],
        }),
    ];

    assert_eq!(events, expected);

    close_run_session(session).await;
    assert_eq!(tool.recorded_calls(), vec![tool_args]);
}

#[tokio::test]
#[allow(clippy::too_many_lines)]
async fn run_stream_handles_multiple_turns() {
    let tool = MockTool::new(
        "calculator",
        AgentToolResult {
            content: vec![Part::text("Calculation done")],
            is_error: false,
        },
    );

    let first_args = json!({"a": 1, "b": 2});
    let second_args = json!({"a": 3, "b": 4});

    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::ToolCall(
                ToolCallPartDelta::default()
                    .with_tool_call_id("call_1".to_string())
                    .with_tool_name("calculator".to_string())
                    .with_args(first_args.to_string()),
            ),
        }),
        ..Default::default()
    }]);
    model.enqueue_stream(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::ToolCall(
                ToolCallPartDelta::default()
                    .with_tool_call_id("call_2".to_string())
                    .with_tool_name("calculator".to_string())
                    .with_args(second_args.to_string()),
            ),
        }),
        ..Default::default()
    }]);
    model.enqueue_stream(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::Text(TextPartDelta::new("All done".to_string())),
        }),
        ..Default::default()
    }]);

    let session = new_run_session(
        Arc::new(AgentParams::new("test_agent", model).add_tool(tool.clone())),
        (),
    )
    .await;

    let stream = session
        .clone()
        .run_stream(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Calculate",
                )]))],
            },
            RunOptions::default(),
        )
        .expect("run_stream succeeds");

    let events = stream
        .map_err(|err| err.to_string())
        .try_collect::<Vec<_>>()
        .await
        .expect("collect stream");

    let expected = vec![
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::ToolCall(
                    ToolCallPartDelta::default()
                        .with_tool_call_id("call_1".to_string())
                        .with_tool_name("calculator".to_string())
                        .with_args(first_args.to_string()),
                ),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Item(AgentStreamItemEvent {
            index: 0,
            item: AgentItem::Model(ModelResponse {
                content: vec![Part::tool_call("call_1", "calculator", first_args.clone())],
                ..Default::default()
            }),
        }),
        AgentStreamEvent::Item(AgentStreamItemEvent {
            index: 1,
            item: AgentItem::Tool(AgentItemTool {
                tool_call_id: "call_1".to_string(),
                tool_name: "calculator".to_string(),
                input: first_args.clone(),
                output: vec![Part::text("Calculation done")],
                status: ToolResultStatus::Completed,
            }),
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::ToolCall(
                    ToolCallPartDelta::default()
                        .with_tool_call_id("call_2".to_string())
                        .with_tool_name("calculator".to_string())
                        .with_args(second_args.to_string()),
                ),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Item(AgentStreamItemEvent {
            index: 2,
            item: AgentItem::Model(ModelResponse {
                content: vec![Part::tool_call("call_2", "calculator", second_args.clone())],
                ..Default::default()
            }),
        }),
        AgentStreamEvent::Item(AgentStreamItemEvent {
            index: 3,
            item: AgentItem::Tool(AgentItemTool {
                tool_call_id: "call_2".to_string(),
                tool_name: "calculator".to_string(),
                input: second_args.clone(),
                output: vec![Part::text("Calculation done")],
                status: ToolResultStatus::Completed,
            }),
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("All done".to_string())),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Item(AgentStreamItemEvent {
            index: 4,
            item: AgentItem::Model(ModelResponse {
                content: vec![Part::text("All done")],
                ..Default::default()
            }),
        }),
        AgentStreamEvent::Response(AgentResponse {
            status: AgentResponseStatus::Completed,
            content: vec![Part::text("All done")],
            output: vec![
                AgentItem::Model(ModelResponse {
                    content: vec![Part::tool_call("call_1", "calculator", first_args.clone())],
                    ..Default::default()
                }),
                AgentItem::Tool(AgentItemTool {
                    tool_call_id: "call_1".to_string(),
                    tool_name: "calculator".to_string(),
                    input: first_args.clone(),
                    output: vec![Part::text("Calculation done")],
                    status: ToolResultStatus::Completed,
                }),
                AgentItem::Model(ModelResponse {
                    content: vec![Part::tool_call("call_2", "calculator", second_args.clone())],
                    ..Default::default()
                }),
                AgentItem::Tool(AgentItemTool {
                    tool_call_id: "call_2".to_string(),
                    tool_name: "calculator".to_string(),
                    input: second_args.clone(),
                    output: vec![Part::text("Calculation done")],
                    status: ToolResultStatus::Completed,
                }),
                AgentItem::Model(ModelResponse {
                    content: vec![Part::text("All done")],
                    ..Default::default()
                }),
            ],
        }),
    ];

    assert_eq!(events, expected);

    close_run_session(session).await;
}

#[tokio::test]
async fn run_stream_throws_max_turns_exceeded_error() {
    let tool = MockTool::new(
        "test_tool",
        AgentToolResult {
            content: vec![Part::text("Tool result")],
            is_error: false,
        },
    );

    let args = json!({});

    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::ToolCall(
                ToolCallPartDelta::default()
                    .with_tool_call_id("call_1".to_string())
                    .with_tool_name("test_tool".to_string())
                    .with_args(args.to_string()),
            ),
        }),
        ..Default::default()
    }]);
    model.enqueue_stream(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::ToolCall(
                ToolCallPartDelta::default()
                    .with_tool_call_id("call_2".to_string())
                    .with_tool_name("test_tool".to_string())
                    .with_args(args.to_string()),
            ),
        }),
        ..Default::default()
    }]);
    model.enqueue_stream(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::ToolCall(
                ToolCallPartDelta::default()
                    .with_tool_call_id("call_3".to_string())
                    .with_tool_name("test_tool".to_string())
                    .with_args(args.to_string()),
            ),
        }),
        ..Default::default()
    }]);

    let session = new_run_session(
        Arc::new(
            AgentParams::new("test_agent", model)
                .add_tool(tool)
                .max_turns(2),
        ),
        (),
    )
    .await;

    let result = session.clone().run_stream(
        RunSessionRequest {
            input: vec![AgentItem::Message(Message::user(vec![Part::text(
                "Keep using tools",
            )]))],
        },
        RunOptions::default(),
    );

    match result {
        Err(AgentError::MaxTurnsExceeded { .. }) => {}
        Ok(mut stream) => {
            let mut seen = false;
            while let Some(event) = stream.next().await {
                if matches!(event, Err(AgentError::MaxTurnsExceeded { .. })) {
                    seen = true;
                    break;
                }
            }
            assert!(seen, "expected MaxTurnsExceeded error during streaming");
        }
        other => {
            drop(other);
            panic!("expected MaxTurnsExceeded error")
        }
    }

    close_run_session(session).await;
}

#[tokio::test]
async fn run_stream_throws_language_model_error() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(MockStreamResult::error(LanguageModelError::InvalidInput(
        "Rate limit exceeded".to_string(),
    )));

    let session = new_run_session(Arc::new(AgentParams::new("test_agent", model)), ()).await;

    let result = session.clone().run_stream(
        RunSessionRequest {
            input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
        },
        RunOptions::default(),
    );

    match result {
        Err(AgentError::LanguageModel { source: err, .. }) => {
            assert!(err.to_string().contains("Rate limit exceeded"));
        }
        Ok(mut stream) => {
            let mut seen = false;
            while let Some(event) = stream.next().await {
                match event {
                    Err(AgentError::LanguageModel { source: err, .. }) => {
                        assert!(err.to_string().contains("Rate limit exceeded"));
                        seen = true;
                        break;
                    }
                    Err(_) => break,
                    Ok(_) => {}
                }
            }
            assert!(
                seen,
                "expected language model error during stream consumption"
            );
        }
        other => {
            drop(other);
            panic!("expected language model error")
        }
    }

    close_run_session(session).await;
}

#[tokio::test]
async fn run_stream_commits_materializable_partial_content_before_error() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(MockStreamResult::partials_then_error(
        mixed_snapshot_partials(),
        LanguageModelError::InvalidInput("stream failed".to_string()),
    ));
    let session = new_run_session(Arc::new(AgentParams::new("test_agent", model)), ()).await;
    let mut stream = session
        .clone()
        .run_stream(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default(),
        )
        .expect("stream should initialize");

    let mut partial_count = 0;
    let mut committed_item = None;
    let error = loop {
        match stream.next().await {
            Some(Ok(AgentStreamEvent::Partial(_))) => partial_count += 1,
            Some(Ok(AgentStreamEvent::Item(event))) => committed_item = Some(event.item),
            Some(Ok(event)) => panic!("unexpected terminal event: {event:?}"),
            Some(Err(error)) => break error,
            None => panic!("stream ended without an error"),
        }
    };
    assert_eq!(partial_count, 3);
    let expected_item = AgentItem::Model(mixed_snapshot_model_response());
    assert_eq!(committed_item, Some(expected_item.clone()));
    let snapshot = error.snapshot().expect("error should contain a snapshot");
    assert_eq!(snapshot.output, vec![expected_item]);

    close_run_session(session).await;
}

#[tokio::test]
async fn run_stream_records_cancelled_results_for_materialized_tool_calls() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(mixed_snapshot_partials());
    let session = new_run_session(Arc::new(AgentParams::new("test_agent", model)), ()).await;
    let cancellation_token = CancellationToken::new();
    let mut stream = session
        .clone()
        .run_stream(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
            },
            RunOptions::default().with_cancellation_token(cancellation_token.clone()),
        )
        .expect("stream should initialize");

    for _ in 0..3 {
        assert!(matches!(
            stream.next().await,
            Some(Ok(AgentStreamEvent::Partial(_)))
        ));
    }
    cancellation_token.cancel();

    let item = match stream.next().await {
        Some(Ok(AgentStreamEvent::Item(event))) => event.item,
        event => panic!("expected committed model item, got {event:?}"),
    };
    let expected_item = AgentItem::Model(mixed_snapshot_model_response());
    assert_eq!(item, expected_item);

    let cancelled_tool_item = match stream.next().await {
        Some(Ok(AgentStreamEvent::Item(event))) => event.item,
        event => panic!("expected cancelled tool item, got {event:?}"),
    };
    let expected_tool_item = AgentItem::Tool(AgentItemTool {
        tool_call_id: "call_1".to_string(),
        tool_name: "weather".to_string(),
        input: json!({"city": "Paris"}),
        output: Vec::new(),
        status: ToolResultStatus::Cancelled,
    });
    assert_eq!(cancelled_tool_item, expected_tool_item);

    let response = match stream.next().await {
        Some(Ok(AgentStreamEvent::Response(response))) => response,
        event => panic!("expected cancelled response, got {event:?}"),
    };
    assert_eq!(response.status, AgentResponseStatus::Cancelled);
    assert!(response.content.is_empty());
    assert_eq!(response.output, vec![expected_item, expected_tool_item]);
    assert!(stream.next().await.is_none());

    close_run_session(session).await;
}

#[tokio::test]
async fn run_session_reports_instruction_resolution_as_init_error() {
    let model = Arc::new(MockLanguageModel::new());
    let params = Arc::new(AgentParams::new("test_agent", model).add_instruction(
        |(): &()| -> Result<String, DynError> {
            Err(std::io::Error::other("could not load tenant instructions").into())
        },
    ));

    let result = RunSession::new(params, ()).await;

    match result {
        Err(AgentError::Init { source: err, .. }) => {
            assert!(err
                .to_string()
                .contains("could not load tenant instructions"));
        }
        _ => panic!("expected agent initialization error"),
    }
}
