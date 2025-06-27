use std::sync::Arc;

type DynError = Box<dyn std::error::Error + Send + Sync>;
use futures::{future::BoxFuture, StreamExt, TryStreamExt};
use llm_agent::{
    AgentError, AgentItem, AgentItemTool, AgentParams, AgentResponse, AgentStreamEvent,
    AgentStreamItemEvent, AgentTool, AgentToolResult, InstructionParam, RunSession,
    RunSessionRequest, RunState, Toolkit, ToolkitSession,
};
use llm_sdk::{
    llm_sdk_test::{MockGenerateResult, MockLanguageModel, MockStreamResult},
    AssistantMessage, ContentDelta, JSONSchema, LanguageModelError, Message, ModelResponse,
    ModelUsage, Part, PartDelta, PartialModelResponse, TextPartDelta, ToolCallPartDelta,
    ToolMessage, UserMessage,
};
use serde_json::{json, Value};

type ExecuteFn = dyn for<'a> Fn(Value, &'a RunState) -> Result<AgentToolResult, DynError>
    + Send
    + Sync
    + 'static;

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

    fn with_execute_fn<F>(name: &str, result: AgentToolResult, execute: F) -> Self
    where
        F: for<'a> Fn(Value, &'a RunState) -> Result<AgentToolResult, DynError>
            + Send
            + Sync
            + 'static,
    {
        let _ = result;

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
    tools: Vec<Arc<dyn AgentTool<TCtx>>>,
    system_prompt_calls: std::sync::Mutex<usize>,
    tools_calls: std::sync::Mutex<usize>,
    close_calls: std::sync::Mutex<usize>,
}

impl<TCtx> MockToolkitSessionState<TCtx> {
    fn new(system_prompt: Option<String>, tools: Vec<Arc<dyn AgentTool<TCtx>>>) -> Arc<Self> {
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

    fn tools(&self) -> Vec<Arc<dyn AgentTool<TCtx>>> {
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

impl AgentTool<CustomerContext> for LookupOrderTool {
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

impl AgentTool<()> for MockTool {
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
        Err(_) => panic!("session should not be shared at close"),
    }
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
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Hello!")],
            }))],
        })
        .await
        .expect("run succeeds");

    let expected = AgentResponse {
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
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Use the tool")],
            }))],
        })
        .await
        .expect("run succeeds");

    let expected = AgentResponse {
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
                is_error: false,
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
async fn run_executes_multiple_tool_calls_in_parallel() {
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
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Use both tools")],
            }))],
        })
        .await
        .expect("run succeeds");

    let expected = AgentResponse {
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
                is_error: false,
            }),
            AgentItem::Tool(AgentItemTool {
                tool_call_id: "call_2".to_string(),
                tool_name: "tool_2".to_string(),
                input: json!({"param": "value2"}),
                output: vec![Part::text("Tool 2 result")],
                is_error: false,
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
        .run(RunSessionRequest {
            input: vec![
                AgentItem::Message(Message::User(UserMessage {
                    content: vec![Part::text("What did I say?")],
                })),
                AgentItem::Message(Message::Assistant(AssistantMessage {
                    content: vec![Part::text("Cached answer")],
                })),
            ],
        })
        .await
        .expect("run succeeds");

    assert_eq!(
        response,
        AgentResponse {
            content: vec![Part::text("Cached answer")],
            output: vec![],
        }
    );
    assert!(model.tracked_generate_inputs().is_empty());

    close_run_session(session).await;
}

#[tokio::test]
async fn run_resumes_tool_processing_from_tool_message_with_partial_results() {
    let resume_tool = MockTool::with_execute_fn(
        "resume_tool",
        AgentToolResult {
            content: vec![],
            is_error: false,
        },
        |_args, _state| {
            Ok(AgentToolResult {
                content: vec![Part::text("call_2 result")],
                is_error: false,
            })
        },
    );

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
        .run(RunSessionRequest {
            input: vec![
                AgentItem::Message(Message::User(UserMessage {
                    content: vec![Part::text("Continue")],
                })),
                AgentItem::Model(ModelResponse {
                    content: vec![
                        Part::tool_call("call_1", "resume_tool", json!({"step": 1})),
                        Part::tool_call("call_2", "resume_tool", json!({"step": 2})),
                    ],
                    ..Default::default()
                }),
                AgentItem::Message(Message::Tool(ToolMessage {
                    content: vec![Part::tool_result(
                        "call_1",
                        "resume_tool",
                        vec![Part::text("already done")],
                    )],
                })),
            ],
        })
        .await
        .expect("run succeeds");

    assert_eq!(resume_tool.recorded_calls(), vec![json!({"step": 2})]);

    assert_eq!(
        response,
        AgentResponse {
            content: vec![Part::text("Final reply")],
            output: vec![
                AgentItem::Tool(AgentItemTool {
                    tool_call_id: "call_2".to_string(),
                    tool_name: "resume_tool".to_string(),
                    input: json!({"step": 2}),
                    output: vec![Part::text("call_2 result")],
                    is_error: false,
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
    let resume_tool = MockTool::with_execute_fn(
        "resume_tool",
        AgentToolResult {
            content: vec![],
            is_error: false,
        },
        |_args, _state| {
            Ok(AgentToolResult {
                content: vec![Part::text("call_2 via item")],
                is_error: false,
            })
        },
    );

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
        .run(RunSessionRequest {
            input: vec![
                AgentItem::Message(Message::User(UserMessage {
                    content: vec![Part::text("Continue")],
                })),
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
                    is_error: false,
                }),
            ],
        })
        .await
        .expect("run succeeds");

    assert_eq!(resume_tool.recorded_calls(), vec![json!({"stage": 2})]);

    assert_eq!(
        response,
        AgentResponse {
            content: vec![Part::text("Final reply from items")],
            output: vec![
                AgentItem::Tool(AgentItemTool {
                    tool_call_id: "call_2".to_string(),
                    tool_name: "resume_tool".to_string(),
                    input: json!({"stage": 2}),
                    output: vec![Part::text("call_2 via item")],
                    is_error: false,
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
        .run(RunSessionRequest {
            input: vec![
                AgentItem::Message(Message::User(UserMessage {
                    content: vec![Part::text("Resume")],
                })),
                AgentItem::Message(Message::Tool(ToolMessage {
                    content: vec![Part::tool_result(
                        "call_1",
                        "resume_tool",
                        vec![Part::text("orphan")],
                    )],
                })),
            ],
        })
        .await
        .expect_err("run fails");

    match err {
        AgentError::Invariant(msg) => {
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
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Calculate some numbers")],
            }))],
        })
        .await
        .expect("run succeeds");

    let expected = AgentResponse {
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
                is_error: false,
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
                is_error: false,
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
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Keep using tools")],
            }))],
        })
        .await;

    match result {
        Err(AgentError::MaxTurnsExceeded(turns)) => assert_eq!(turns, 2),
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
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Use a tool")],
            }))],
        })
        .await;

    match result {
        Err(AgentError::Invariant(message)) => {
            assert!(message.contains("Tool non_existent_tool not found"));
        }
        other => panic!("expected invariant error, got {other:?}"),
    }
}

#[tokio::test]
async fn run_throws_tool_execution_error() {
    let failing_tool = MockTool::with_execute_fn(
        "failing_tool",
        AgentToolResult {
            content: vec![],
            is_error: false,
        },
        |_args, _state| Err("Tool execution failed".into()),
    );

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
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Use the tool")],
            }))],
        })
        .await;

    assert!(matches!(result, Err(AgentError::ToolExecution(_))));
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
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Use the tool")],
            }))],
        })
        .await
        .expect("run succeeds");

    let expected = AgentResponse {
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
                is_error: true,
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
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Hello")],
            }))],
        })
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
async fn run_throws_language_model_error_when_generation_fails() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(MockGenerateResult::error(LanguageModelError::InvalidInput(
        "API quota exceeded".to_string(),
    )));

    let session = new_run_session(Arc::new(AgentParams::new("test_agent", model)), ()).await;

    let result = session
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Hello")],
            }))],
        })
        .await;

    match result {
        Err(AgentError::LanguageModel(err)) => {
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
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Hello")],
            }))],
        })
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

    let lookup_tool = Arc::new(LookupOrderTool::new());
    let toolkit_session_state = MockToolkitSessionState::new(
        Some("Toolkit prompt".to_string()),
        vec![lookup_tool.clone()],
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
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Status?")],
            }))],
        })
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
        assert_eq!(tools[0].name, "lookup-order");
    }

    let expected = AgentResponse {
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
                is_error: false,
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
async fn run_stream_streams_response_when_no_tool_call() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(vec![
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "Hel".to_string(),
                    citation: None,
                }),
            }),
            ..Default::default()
        },
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "lo".to_string(),
                    citation: None,
                }),
            }),
            ..Default::default()
        },
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "!".to_string(),
                    citation: None,
                }),
            }),
            ..Default::default()
        },
    ]);

    let session =
        new_run_session(Arc::new(AgentParams::new("test_agent", model.clone())), ()).await;

    let stream = session
        .clone()
        .run_stream(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Hi")],
            }))],
        })
        .await
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
                part: PartDelta::Text(TextPartDelta {
                    text: "Hel".to_string(),
                    citation: None,
                }),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "lo".to_string(),
                    citation: None,
                }),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "!".to_string(),
                    citation: None,
                }),
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
            part: PartDelta::Text(TextPartDelta {
                text: "Done".to_string(),
                citation: None,
            }),
        }),
        ..Default::default()
    }]);

    let lookup_tool = Arc::new(LookupOrderTool::new());
    let toolkit_session_state = MockToolkitSessionState::new(
        Some("Streaming toolkit prompt".to_string()),
        vec![lookup_tool.clone()],
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
        .run_stream(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Hello")],
            }))],
        })
        .await
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
                part: PartDelta::Text(TextPartDelta {
                    text: "Done".to_string(),
                    citation: None,
                }),
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
    assert_eq!(tools[0].name, "lookup-order");

    close_run_session(session).await;

    assert_eq!(toolkit_session_state.close_calls(), 1);
}

#[tokio::test]
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
            part: PartDelta::ToolCall(ToolCallPartDelta {
                tool_call_id: Some("call_1".to_string()),
                tool_name: Some("test_tool".to_string()),
                args: Some(tool_args.to_string()),
                id: None,
            }),
        }),
        ..Default::default()
    }]);
    model.enqueue_stream(vec![
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "Final".to_string(),
                    citation: None,
                }),
            }),
            ..Default::default()
        },
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: " response".to_string(),
                    citation: None,
                }),
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
        .run_stream(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Use tool")],
            }))],
        })
        .await
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
                part: PartDelta::ToolCall(ToolCallPartDelta {
                    tool_call_id: Some("call_1".to_string()),
                    tool_name: Some("test_tool".to_string()),
                    args: Some(tool_args.to_string()),
                    id: None,
                }),
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
                is_error: false,
            }),
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "Final".to_string(),
                    citation: None,
                }),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: " response".to_string(),
                    citation: None,
                }),
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
                    is_error: false,
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
            part: PartDelta::ToolCall(ToolCallPartDelta {
                tool_call_id: Some("call_1".to_string()),
                tool_name: Some("calculator".to_string()),
                args: Some(first_args.to_string()),
                id: None,
            }),
        }),
        ..Default::default()
    }]);
    model.enqueue_stream(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::ToolCall(ToolCallPartDelta {
                tool_call_id: Some("call_2".to_string()),
                tool_name: Some("calculator".to_string()),
                args: Some(second_args.to_string()),
                id: None,
            }),
        }),
        ..Default::default()
    }]);
    model.enqueue_stream(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::Text(TextPartDelta {
                text: "All done".to_string(),
                citation: None,
            }),
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
        .run_stream(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Calculate")],
            }))],
        })
        .await
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
                part: PartDelta::ToolCall(ToolCallPartDelta {
                    tool_call_id: Some("call_1".to_string()),
                    tool_name: Some("calculator".to_string()),
                    args: Some(first_args.to_string()),
                    id: None,
                }),
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
                is_error: false,
            }),
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::ToolCall(ToolCallPartDelta {
                    tool_call_id: Some("call_2".to_string()),
                    tool_name: Some("calculator".to_string()),
                    args: Some(second_args.to_string()),
                    id: None,
                }),
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
                is_error: false,
            }),
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "All done".to_string(),
                    citation: None,
                }),
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
                    is_error: false,
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
                    is_error: false,
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
            part: PartDelta::ToolCall(ToolCallPartDelta {
                tool_call_id: Some("call_1".to_string()),
                tool_name: Some("test_tool".to_string()),
                args: Some(args.to_string()),
                id: None,
            }),
        }),
        ..Default::default()
    }]);
    model.enqueue_stream(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::ToolCall(ToolCallPartDelta {
                tool_call_id: Some("call_2".to_string()),
                tool_name: Some("test_tool".to_string()),
                args: Some(args.to_string()),
                id: None,
            }),
        }),
        ..Default::default()
    }]);
    model.enqueue_stream(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::ToolCall(ToolCallPartDelta {
                tool_call_id: Some("call_3".to_string()),
                tool_name: Some("test_tool".to_string()),
                args: Some(args.to_string()),
                id: None,
            }),
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

    let result = session
        .clone()
        .run_stream(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Keep using tools")],
            }))],
        })
        .await;

    match result {
        Err(AgentError::MaxTurnsExceeded(_)) => {}
        Ok(mut stream) => {
            let mut seen = false;
            while let Some(event) = stream.next().await {
                if matches!(event, Err(AgentError::MaxTurnsExceeded(_))) {
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

    let result = session
        .clone()
        .run_stream(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Hello")],
            }))],
        })
        .await;

    match result {
        Err(AgentError::LanguageModel(err)) => {
            assert!(err.to_string().contains("Rate limit exceeded"));
        }
        Ok(mut stream) => {
            let mut seen = false;
            while let Some(event) = stream.next().await {
                match event {
                    Err(AgentError::LanguageModel(err)) => {
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
