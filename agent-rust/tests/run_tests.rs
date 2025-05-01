use std::sync::Arc;

use futures::{StreamExt, TryStreamExt};
use llm_agent::{
    AgentError, AgentItem, AgentItemTool, AgentParams, AgentRequest, AgentResponse,
    AgentStreamEvent, AgentTool, AgentToolResult, InstructionParam, RunSession, RunState,
};
use llm_sdk::{
    llm_sdk_test::{MockGenerateResult, MockLanguageModel, MockStreamResult},
    ContentDelta, JSONSchema, LanguageModelError, Message, ModelResponse, ModelUsage, Part,
    PartDelta, PartialModelResponse, TextPartDelta, ToolCallPartDelta, UserMessage,
};
use serde_json::{json, Value};

type ExecuteFn = dyn for<'a> Fn(
        Value,
        &'a RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>>
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
        F: for<'a> Fn(
                Value,
                &'a RunState,
            )
                -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>>
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

#[async_trait::async_trait]
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

    async fn execute(
        &self,
        args: Value,
        _context: &(),
        state: &RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>> {
        self.all_calls.lock().unwrap().push(args.clone());
        (self.execute)(args, state)
    }
}

#[tokio::test]
async fn run_returns_response_when_no_tool_call() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Hi!")],
        ..Default::default()
    });

    let session = RunSession::new(Arc::new(AgentParams::new("test_agent", model))).await;

    let response = session
        .run(AgentRequest {
            context: (),
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

    let session = RunSession::new(Arc::new(
        AgentParams::new("test_agent", model).add_tool(tool.clone()),
    ))
    .await;

    let response = session
        .run(AgentRequest {
            context: (),
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

    let session = RunSession::new(Arc::new(
        AgentParams::new("test_agent", model)
            .add_tool(tool1.clone())
            .add_tool(tool2.clone()),
    ))
    .await;

    let response = session
        .run(AgentRequest {
            context: (),
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

    let session = RunSession::new(Arc::new(
        AgentParams::new("test_agent", model).add_tool(tool.clone()),
    ))
    .await;

    let response = session
        .run(AgentRequest {
            context: (),
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

    let session = RunSession::new(Arc::new(AgentParams::new("test_agent", model))).await;

    let result = session
        .run(AgentRequest {
            context: (),
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

    let session = RunSession::new(Arc::new(
        AgentParams::new("test_agent", model).add_tool(failing_tool),
    ))
    .await;

    let result = session
        .run(AgentRequest {
            context: (),
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

    let session = RunSession::new(Arc::new(
        AgentParams::new("test_agent", model).add_tool(tool.clone()),
    ))
    .await;

    let response = session
        .run(AgentRequest {
            context: (),
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

    let session = RunSession::new(Arc::new(
        AgentParams::new("test_agent", model.clone())
            .temperature(0.7)
            .top_p(0.9)
            .top_k(40)
            .presence_penalty(0.1)
            .frequency_penalty(0.2),
    ))
    .await;

    session
        .run(AgentRequest {
            context: (),
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

    let session = RunSession::new(Arc::new(AgentParams::new("test_agent", model))).await;

    let result = session
        .run(AgentRequest {
            context: (),
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

    let session: RunSession<TestContext> = RunSession::new(Arc::new(
        AgentParams::new("test_agent", model.clone()).instructions(vec![
            InstructionParam::String("You are a helpful assistant.".to_string()),
            InstructionParam::Func(Box::new(|ctx: &TestContext| {
                Ok(format!("The user is a {}.", ctx.user_role))
            })),
            InstructionParam::String("Always be polite.".to_string()),
        ]),
    ))
    .await;

    session
        .run(AgentRequest {
            context: TestContext {
                user_role: "developer".to_string(),
            },
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
                }),
            }),
            ..Default::default()
        },
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "lo".to_string(),
                }),
            }),
            ..Default::default()
        },
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "!".to_string(),
                }),
            }),
            ..Default::default()
        },
    ]);

    let session = RunSession::new(Arc::new(AgentParams::new("test_agent", model.clone()))).await;

    let events = session
        .run_stream(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Hi")],
            }))],
        })
        .await
        .expect("run_stream succeeds")
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
                }),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "lo".to_string(),
                }),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "!".to_string(),
                }),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Item(AgentItem::Model(ModelResponse {
            content: vec![Part::text("Hello!")],
            ..Default::default()
        })),
        AgentStreamEvent::Response(AgentResponse {
            content: vec![Part::text("Hello!")],
            output: vec![AgentItem::Model(ModelResponse {
                content: vec![Part::text("Hello!")],
                ..Default::default()
            })],
        }),
    ];

    assert_eq!(events, expected);
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
                }),
            }),
            ..Default::default()
        },
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: " response".to_string(),
                }),
            }),
            ..Default::default()
        },
    ]);

    let events = RunSession::new(Arc::new(
        AgentParams::new("test_agent", model).add_tool(tool.clone()),
    ))
    .await
    .run_stream(AgentRequest {
        context: (),
        input: vec![AgentItem::Message(Message::User(UserMessage {
            content: vec![Part::text("Use tool")],
        }))],
    })
    .await
    .expect("run_stream succeeds")
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
        AgentStreamEvent::Item(AgentItem::Model(ModelResponse {
            content: vec![Part::tool_call("call_1", "test_tool", tool_args.clone())],
            ..Default::default()
        })),
        AgentStreamEvent::Item(AgentItem::Tool(AgentItemTool {
            tool_call_id: "call_1".to_string(),
            tool_name: "test_tool".to_string(),
            input: tool_args.clone(),
            output: vec![Part::text("Tool result")],
            is_error: false,
        })),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "Final".to_string(),
                }),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: " response".to_string(),
                }),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Item(AgentItem::Model(ModelResponse {
            content: vec![Part::text("Final response")],
            ..Default::default()
        })),
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
            }),
        }),
        ..Default::default()
    }]);

    let events = RunSession::new(Arc::new(
        AgentParams::new("test_agent", model).add_tool(tool.clone()),
    ))
    .await
    .run_stream(AgentRequest {
        context: (),
        input: vec![AgentItem::Message(Message::User(UserMessage {
            content: vec![Part::text("Calculate")],
        }))],
    })
    .await
    .expect("run_stream succeeds")
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
        AgentStreamEvent::Item(AgentItem::Model(ModelResponse {
            content: vec![Part::tool_call("call_1", "calculator", first_args.clone())],
            ..Default::default()
        })),
        AgentStreamEvent::Item(AgentItem::Tool(AgentItemTool {
            tool_call_id: "call_1".to_string(),
            tool_name: "calculator".to_string(),
            input: first_args.clone(),
            output: vec![Part::text("Calculation done")],
            is_error: false,
        })),
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
        AgentStreamEvent::Item(AgentItem::Model(ModelResponse {
            content: vec![Part::tool_call("call_2", "calculator", second_args.clone())],
            ..Default::default()
        })),
        AgentStreamEvent::Item(AgentItem::Tool(AgentItemTool {
            tool_call_id: "call_2".to_string(),
            tool_name: "calculator".to_string(),
            input: second_args.clone(),
            output: vec![Part::text("Calculation done")],
            is_error: false,
        })),
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "All done".to_string(),
                }),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Item(AgentItem::Model(ModelResponse {
            content: vec![Part::text("All done")],
            ..Default::default()
        })),
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

    let result = RunSession::new(Arc::new(
        AgentParams::new("test_agent", model)
            .add_tool(tool)
            .max_turns(2),
    ))
    .await
    .run_stream(AgentRequest {
        context: (),
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
}

#[tokio::test]
async fn run_stream_throws_language_model_error() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(MockStreamResult::error(LanguageModelError::InvalidInput(
        "Rate limit exceeded".to_string(),
    )));

    let result = RunSession::new(Arc::new(AgentParams::new("test_agent", model)))
        .await
        .run_stream(AgentRequest {
            context: (),
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
}
