use axum::{
    extract::Request,
    http::{header::AUTHORIZATION, StatusCode},
    middleware::{self, Next},
    response::Response,
    Router,
};
use llm_agent::AgentResponseStatus;
use llm_agent::RunOptions;
use llm_agent::{
    mcp::{MCPParams, MCPStreamableHTTPParams, MCPToolkit},
    Agent, AgentError, AgentItem, AgentParams, AgentResponse, BoxedError, RunSessionRequest,
};
use llm_sdk::ToolResultStatus;
use llm_sdk::{
    llm_sdk_test::MockLanguageModel, AudioFormat, LanguageModel, LanguageModelError,
    LanguageModelInput, LanguageModelMetadata, LanguageModelResult, LanguageModelStream, Message,
    ModelResponse, Part,
};
use rmcp::{
    handler::server::ServerHandler,
    model::{
        CallToolRequestParams, CallToolResult, Implementation, InitializeRequestParams, JsonObject,
        ListToolsResult, PaginatedRequestParams, ServerCapabilities, ServerInfo, Tool,
    },
    transport::streamable_http_server::{
        session::local::LocalSessionManager, tower::StreamableHttpServerConfig,
        StreamableHttpService,
    },
    ErrorData, RoleServer,
};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::{
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex,
    },
    time::Duration,
};
use tokio::{
    net::TcpListener,
    sync::{oneshot, Notify, RwLock},
    time::timeout,
};
use tokio_util::sync::CancellationToken;

const IMAGE_DATA: &str = "AAEC";
const AUDIO_DATA: &str = "AwQ=";
const AUTH_TOKEN: &str = "mcp-test-token";

#[tokio::test]
async fn agent_hydrates_mcp_tools_over_streamable_http() -> Result<(), BoxedError> {
    let stub = start_stub_mcp_server()
        .await
        .map_err(|err| format!("Failed to start stub MCP server: {err}"))?;

    let model = Arc::new(MockLanguageModel::new());
    let tool_args = json!({ "shift": "evening" });
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call(
            "call_1",
            "list_shuttles",
            tool_args.clone(),
        )],
        ..Default::default()
    });
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Ready to roll.")],
        ..Default::default()
    });

    let stub_url = stub.url().to_string();
    let agent = Agent::new(AgentParams::new("mcp-test", model.clone()).add_toolkit(
        MCPToolkit::new({
            let stub_url = stub_url.clone();
            move |(): &()| {
                Ok(MCPParams::StreamableHttp(MCPStreamableHTTPParams {
                    url: stub_url.clone(),
                    authorization: Some(format!(" bearer {AUTH_TOKEN} ")),
                }))
            }
        }),
    ));

    let session = agent
        .create_session(())
        .await
        .map_err(|err| format!("Failed to create session: {err}"))?;

    let response = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "What's running tonight?",
                )]))],
            },
            RunOptions::default(),
        )
        .await
        .map_err(|err| format!("Failed to run agent: {err}"))?;

    let expected = AgentResponse {
        status: AgentResponseStatus::Completed,
        content: vec![Part::text("Ready to roll.")],
        output: vec![
            AgentItem::Model(ModelResponse {
                content: vec![Part::tool_call(
                    "call_1",
                    "list_shuttles",
                    tool_args.clone(),
                )],
                ..Default::default()
            }),
            AgentItem::Tool(llm_agent::AgentItemTool {
                tool_call_id: "call_1".to_string(),
                tool_name: "list_shuttles".to_string(),
                input: tool_args.clone(),
                output: vec![
                    Part::text("Shuttle summary for evening shift."),
                    Part::image(IMAGE_DATA, "image/png"),
                    Part::audio(AUDIO_DATA, AudioFormat::Mp3),
                ],
                status: ToolResultStatus::Completed,
            }),
            AgentItem::Model(ModelResponse {
                content: vec![Part::text("Ready to roll.")],
                ..Default::default()
            }),
        ],
    };

    assert_eq!(response, expected);

    session
        .close()
        .await
        .map_err(|err| format!("Failed to close session: {err}"))?;
    stub.stop().await?;
    Ok(())
}

#[tokio::test]
async fn agent_cancels_in_flight_mcp_tool_requests() -> Result<(), BoxedError> {
    let stub = start_stub_mcp_server()
        .await
        .map_err(|err| format!("Failed to start stub MCP server: {err}"))?;
    stub.block_tool_calls();

    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call(
            "call_1",
            "list_shuttles",
            json!({ "shift": "evening" }),
        )],
        ..Default::default()
    });

    let stub_url = stub.url().to_string();
    let agent = Agent::new(
        AgentParams::new("mcp-cancellation-test", model).add_toolkit(MCPToolkit::new(
            move |(): &()| {
                Ok(MCPParams::StreamableHttp(MCPStreamableHTTPParams {
                    url: stub_url.clone(),
                    authorization: Some(AUTH_TOKEN.to_string()),
                }))
            },
        )),
    );
    let session = agent
        .create_session(())
        .await
        .map_err(|err| format!("Failed to create session: {err}"))?;
    let cancellation_token = CancellationToken::new();
    let mut run = Box::pin(session.run(
        RunSessionRequest {
            input: vec![AgentItem::Message(Message::user(vec![Part::text(
                "Wait for the shuttle list",
            )]))],
        },
        RunOptions::default().with_cancellation_token(cancellation_token.clone()),
    ));

    tokio::select! {
        () = stub.wait_for_tool_call() => {}
        result = &mut run => {
            return Err(format!("Run ended before the MCP tool call started: {result:?}").into());
        }
        () = tokio::time::sleep(Duration::from_secs(5)) => {
            return Err("Timed out waiting for the MCP tool call to start".into());
        }
    }
    cancellation_token.cancel();

    let response = timeout(Duration::from_secs(5), &mut run)
        .await
        .map_err(|_| "Timed out waiting for the cancelled run to finish")?
        .map_err(|err| format!("Cancelled run failed: {err}"))?;
    assert_eq!(response.status, AgentResponseStatus::Cancelled);
    timeout(Duration::from_secs(5), stub.wait_for_tool_cancellation())
        .await
        .map_err(|_| "MCP server did not receive request cancellation")?;
    drop(run);

    session
        .close()
        .await
        .map_err(|err| format!("Failed to close session: {err}"))?;
    stub.stop().await?;
    Ok(())
}

#[tokio::test]
async fn agent_rejects_unsupported_mcp_audio_content() -> Result<(), BoxedError> {
    let stub = start_stub_mcp_server()
        .await
        .map_err(|err| format!("Failed to start stub MCP server: {err}"))?;
    stub.set_tool(ToolDefinition::new(
        "list_shuttles",
        "List active shuttle routes for a shift",
        |_| vec![audio_content(AUDIO_DATA, "audio/unknown")],
    ))
    .await;

    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call(
            "call_1",
            "list_shuttles",
            json!({ "shift": "evening" }),
        )],
        ..Default::default()
    });
    let stub_url = stub.url().to_string();
    let agent = Agent::new(
        AgentParams::new("mcp-test", model).add_toolkit(MCPToolkit::new(move |(): &()| {
            Ok(MCPParams::StreamableHttp(MCPStreamableHTTPParams {
                url: stub_url.clone(),
                authorization: Some(AUTH_TOKEN.to_string()),
            }))
        })),
    );
    let session = agent
        .create_session(())
        .await
        .map_err(|err| format!("Failed to create session: {err}"))?;

    let result = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "What's running tonight?",
                )]))],
            },
            RunOptions::default(),
        )
        .await;

    assert!(matches!(result, Err(AgentError::ToolExecution { .. })));
    session
        .close()
        .await
        .map_err(|err| format!("Failed to close session: {err}"))?;
    stub.stop().await?;
    Ok(())
}

#[tokio::test]
#[allow(clippy::too_many_lines)]
async fn agent_refreshes_tools_on_mcp_list_change() -> Result<(), BoxedError> {
    let stub = start_stub_mcp_server()
        .await
        .map_err(|err| format!("Failed to start stub MCP server: {err}"))?;

    let model = Arc::new(RefreshAwareModel::new());
    let tool_args = json!({ "shift": "evening" });

    let stub_url = stub.url().to_string();
    let agent = Agent::new(AgentParams::new("mcp-test", model.clone()).add_toolkit(
        MCPToolkit::new({
            let stub_url = stub_url.clone();
            move |(): &()| {
                Ok(MCPParams::StreamableHttp(MCPStreamableHTTPParams {
                    url: stub_url.clone(),
                    authorization: Some(AUTH_TOKEN.to_string()),
                }))
            }
        }),
    ));

    let session = agent
        .create_session(())
        .await
        .map_err(|err| format!("Failed to create session: {err}"))?;

    let first_response = session
        .run(
            RunSessionRequest {
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "What's running tonight?",
                )]))],
            },
            RunOptions::default(),
        )
        .await
        .map_err(|err| format!("Failed to run session: {err}"))?;

    let expected_first = AgentResponse {
        status: AgentResponseStatus::Completed,
        content: vec![Part::text("Ready to roll.")],
        output: vec![
            AgentItem::Model(ModelResponse {
                content: vec![Part::tool_call(
                    "call_1",
                    "list_shuttles",
                    tool_args.clone(),
                )],
                ..Default::default()
            }),
            AgentItem::Tool(llm_agent::AgentItemTool {
                tool_call_id: "call_1".to_string(),
                tool_name: "list_shuttles".to_string(),
                input: tool_args.clone(),
                output: vec![
                    Part::text("Shuttle summary for evening shift."),
                    Part::image(IMAGE_DATA, "image/png"),
                    Part::audio(AUDIO_DATA, AudioFormat::Mp3),
                ],
                status: ToolResultStatus::Completed,
            }),
            AgentItem::Model(ModelResponse {
                content: vec![Part::text("Ready to roll.")],
                ..Default::default()
            }),
        ],
    };

    assert_eq!(first_response, expected_first);

    stub.set_tool(ToolDefinition::new(
        "list_shuttles_v2",
        "List active shuttle routes with live updates",
        |args| {
            vec![rmcp::model::Content::text(format!(
                "Updated shuttle roster for {} shift.",
                args.shift.as_str()
            ))]
        },
    ))
    .await;

    let second_response = timeout(Duration::from_secs(5), async {
        loop {
            let response = session
                .run(
                    RunSessionRequest {
                        input: vec![AgentItem::Message(Message::user(vec![Part::text(
                            "How about now?",
                        )]))],
                    },
                    RunOptions::default(),
                )
                .await?;

            if response.content == vec![Part::text("Routes synced.")] {
                return Ok::<AgentResponse, llm_agent::AgentError>(response);
            }

            tokio::task::yield_now().await;
        }
    })
    .await
    .map_err(|_| "Timed out waiting for the MCP tool list to refresh")?
    .map_err(|err| format!("Failed to run session: {err}"))?;

    let expected_second = AgentResponse {
        status: AgentResponseStatus::Completed,
        content: vec![Part::text("Routes synced.")],
        output: vec![
            AgentItem::Model(ModelResponse {
                content: vec![Part::tool_call(
                    "call_2",
                    "list_shuttles_v2",
                    tool_args.clone(),
                )],
                ..Default::default()
            }),
            AgentItem::Tool(llm_agent::AgentItemTool {
                tool_call_id: "call_2".to_string(),
                tool_name: "list_shuttles_v2".to_string(),
                input: tool_args.clone(),
                output: vec![Part::text("Updated shuttle roster for evening shift.")],
                status: ToolResultStatus::Completed,
            }),
            AgentItem::Model(ModelResponse {
                content: vec![Part::text("Routes synced.")],
                ..Default::default()
            }),
        ],
    };

    assert_eq!(second_response, expected_second);

    session
        .close()
        .await
        .map_err(|err| format!("Failed to close session: {err}"))?;
    stub.stop().await?;
    Ok(())
}

#[derive(Default)]
struct RefreshAwareModel {
    phase: Mutex<RefreshModelPhase>,
}

impl RefreshAwareModel {
    fn new() -> Self {
        Self::default()
    }
}

#[derive(Default)]
enum RefreshModelPhase {
    #[default]
    InitialToolCall,
    InitialResponse,
    AwaitingRefresh,
    UpdatedResponse,
    Complete,
}

impl LanguageModel for RefreshAwareModel {
    fn provider(&self) -> &'static str {
        "mcp-refresh-test"
    }

    fn model_id(&self) -> String {
        "mcp-refresh-test".to_string()
    }

    fn metadata(&self) -> Option<&LanguageModelMetadata> {
        None
    }

    fn generate(
        &self,
        input: LanguageModelInput,
    ) -> futures::future::BoxFuture<'_, LanguageModelResult<ModelResponse>> {
        Box::pin(async move {
            let mut phase = self.phase.lock().expect("refresh model state poisoned");
            let response = match *phase {
                RefreshModelPhase::InitialToolCall => {
                    *phase = RefreshModelPhase::InitialResponse;
                    ModelResponse {
                        content: vec![Part::tool_call(
                            "call_1",
                            "list_shuttles",
                            json!({ "shift": "evening" }),
                        )],
                        ..Default::default()
                    }
                }
                RefreshModelPhase::InitialResponse => {
                    *phase = RefreshModelPhase::AwaitingRefresh;
                    ModelResponse {
                        content: vec![Part::text("Ready to roll.")],
                        ..Default::default()
                    }
                }
                RefreshModelPhase::AwaitingRefresh => {
                    let has_updated_tool = input.tools.as_ref().is_some_and(|tools| {
                        tools.iter().any(|tool| {
                            matches!(
                                tool,
                                llm_sdk::Tool::Function(function)
                                    if function.name == "list_shuttles_v2"
                            )
                        })
                    });

                    if has_updated_tool {
                        *phase = RefreshModelPhase::UpdatedResponse;
                        ModelResponse {
                            content: vec![Part::tool_call(
                                "call_2",
                                "list_shuttles_v2",
                                json!({ "shift": "evening" }),
                            )],
                            ..Default::default()
                        }
                    } else {
                        ModelResponse {
                            content: vec![Part::text("Tool refresh pending.")],
                            ..Default::default()
                        }
                    }
                }
                RefreshModelPhase::UpdatedResponse => {
                    *phase = RefreshModelPhase::Complete;
                    ModelResponse {
                        content: vec![Part::text("Routes synced.")],
                        ..Default::default()
                    }
                }
                RefreshModelPhase::Complete => {
                    return Err(LanguageModelError::Invariant(
                        self.provider(),
                        "refresh test model called after completion".into(),
                    ));
                }
            };

            Ok(response)
        })
    }

    fn stream(
        &self,
        _input: LanguageModelInput,
    ) -> futures::future::BoxFuture<'_, LanguageModelResult<LanguageModelStream>> {
        Box::pin(async move {
            Err(LanguageModelError::Invariant(
                self.provider(),
                "streaming is not supported by the refresh test model".into(),
            ))
        })
    }
}

struct StubServer {
    url: String,
    state: Arc<SharedState>,
    shutdown: Option<oneshot::Sender<()>>,
    handle: tokio::task::JoinHandle<()>,
}

impl StubServer {
    fn url(&self) -> &str {
        &self.url
    }

    async fn set_tool(&self, definition: ToolDefinition) {
        self.state.set_tool(definition).await;
        self.state.broadcast_tool_list_changed().await;
    }

    fn block_tool_calls(&self) {
        self.state.block_tool_calls.store(true, Ordering::SeqCst);
    }

    async fn wait_for_tool_call(&self) {
        self.state.tool_call_started.notified().await;
    }

    async fn wait_for_tool_cancellation(&self) {
        self.state.tool_call_cancelled.notified().await;
    }

    async fn stop(self) -> Result<(), BoxedError> {
        if let Some(tx) = self.shutdown {
            let _ = tx.send(());
        }

        self.handle
            .await
            .map_err(|err| format!("Failed to join stub MCP server task: {err}"))?;
        Ok(())
    }
}

#[derive(Clone)]
struct ToolDefinition {
    name: String,
    description: String,
    responder: Arc<dyn Fn(ListShuttlesArgs) -> Vec<rmcp::model::Content> + Send + Sync>,
}

impl ToolDefinition {
    fn new(
        name: impl Into<String>,
        description: impl Into<String>,
        responder: impl Fn(ListShuttlesArgs) -> Vec<rmcp::model::Content> + Send + Sync + 'static,
    ) -> Self {
        Self {
            name: name.into(),
            description: description.into(),
            responder: Arc::new(responder),
        }
    }

    fn respond(&self, args: ListShuttlesArgs) -> Vec<rmcp::model::Content> {
        (self.responder)(args)
    }
}

struct SharedState {
    tool: RwLock<ToolDefinition>,
    peers: RwLock<Vec<rmcp::service::Peer<RoleServer>>>,
    input_schema: Arc<JsonObject>,
    block_tool_calls: AtomicBool,
    tool_call_started: Notify,
    tool_call_cancelled: Notify,
}

impl SharedState {
    fn new(initial_tool: ToolDefinition) -> Self {
        let schema = schemars::schema_for!(ListShuttlesArgs);
        let schema_value = serde_json::to_value(&schema).expect("serialize schema");
        let schema_map = schema_value
            .as_object()
            .cloned()
            .expect("schema must be an object");

        Self {
            tool: RwLock::new(initial_tool),
            peers: RwLock::new(Vec::new()),
            input_schema: Arc::new(schema_map),
            block_tool_calls: AtomicBool::new(false),
            tool_call_started: Notify::new(),
            tool_call_cancelled: Notify::new(),
        }
    }

    async fn register_peer(&self, peer: rmcp::service::Peer<RoleServer>) {
        let mut peers = self.peers.write().await;
        peers.push(peer);
    }

    async fn set_tool(&self, definition: ToolDefinition) {
        let mut tool = self.tool.write().await;
        *tool = definition;
    }

    async fn current_tool(&self) -> ToolDefinition {
        self.tool.read().await.clone()
    }

    async fn broadcast_tool_list_changed(&self) {
        let peers = { self.peers.read().await.clone() };
        if peers.is_empty() {
            return;
        }

        let mut still_connected = Vec::with_capacity(peers.len());
        for peer in peers {
            if peer.notify_tool_list_changed().await.is_ok() {
                still_connected.push(peer);
            }
        }

        let mut guard = self.peers.write().await;
        *guard = still_connected;
    }
}

struct StubMcpService {
    state: Arc<SharedState>,
}

impl StubMcpService {
    fn new(state: Arc<SharedState>) -> Self {
        Self { state }
    }
}

impl ServerHandler for StubMcpService {
    fn initialize(
        &self,
        request: InitializeRequestParams,
        context: rmcp::service::RequestContext<RoleServer>,
    ) -> impl std::future::Future<Output = Result<ServerInfo, ErrorData>> + Send + '_ {
        let state = self.state.clone();
        async move {
            state.register_peer(context.peer.clone()).await;
            context.peer.set_peer_info(request);
            Ok(SharedState::server_info())
        }
    }

    fn get_info(&self) -> ServerInfo {
        SharedState::server_info()
    }

    fn list_tools(
        &self,
        _request: Option<PaginatedRequestParams>,
        _context: rmcp::service::RequestContext<RoleServer>,
    ) -> impl std::future::Future<Output = Result<ListToolsResult, ErrorData>> + Send + '_ {
        let state = self.state.clone();
        async move {
            let tool = state.current_tool().await;
            let schema = state.input_schema.clone();
            let tool_spec = Tool::new(tool.name.clone(), tool.description.clone(), schema);
            Ok(ListToolsResult::with_all_items(vec![tool_spec]))
        }
    }

    fn call_tool(
        &self,
        request: CallToolRequestParams,
        context: rmcp::service::RequestContext<RoleServer>,
    ) -> impl std::future::Future<Output = Result<CallToolResult, ErrorData>> + Send + '_ {
        let state = self.state.clone();
        async move {
            let tool = state.current_tool().await;
            if request.name.as_ref() != tool.name {
                return Err(ErrorData::invalid_params("tool not found", None));
            }

            let args_map = request.arguments.unwrap_or_default();
            let args_value = Value::Object(args_map);
            let args: ListShuttlesArgs = serde_json::from_value(args_value).map_err(|err| {
                ErrorData::invalid_params(format!("invalid arguments: {err}"), None)
            })?;

            if state.block_tool_calls.load(Ordering::SeqCst) {
                state.tool_call_started.notify_one();
                context.ct.cancelled().await;
                state.tool_call_cancelled.notify_one();
                return Err(ErrorData::internal_error("tool call cancelled", None));
            }

            let contents = tool.respond(args);
            Ok(CallToolResult::success(contents))
        }
    }
}

impl SharedState {
    fn server_info() -> ServerInfo {
        ServerInfo::new(
            ServerCapabilities::builder()
                .enable_tools()
                .enable_tool_list_changed()
                .build(),
        )
        .with_server_info(Implementation::new("stub-mcp", "1.0.0"))
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct ListShuttlesArgs {
    #[schemars(description = "Which operating window to query.")]
    shift: Shift,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "lowercase")]
enum Shift {
    Evening,
    Overnight,
}

impl Shift {
    fn as_str(&self) -> &'static str {
        match self {
            Self::Evening => "evening",
            Self::Overnight => "overnight",
        }
    }
}

fn audio_content(data: impl Into<String>, mime_type: impl Into<String>) -> rmcp::model::Content {
    rmcp::model::Content {
        raw: rmcp::model::RawContent::Audio(rmcp::model::RawAudioContent {
            data: data.into(),
            mime_type: mime_type.into(),
        }),
        annotations: None,
    }
}

fn resource_link_content(uri: impl Into<String>, name: impl Into<String>) -> rmcp::model::Content {
    rmcp::model::Content {
        raw: rmcp::model::RawContent::ResourceLink(rmcp::model::RawResource {
            meta: None,
            uri: uri.into(),
            name: name.into(),
            title: None,
            description: None,
            mime_type: None,
            size: None,
            icons: None,
        }),
        annotations: None,
    }
}

async fn start_stub_mcp_server() -> Result<StubServer, BoxedError> {
    let initial_tool = ToolDefinition::new(
        "list_shuttles",
        "List active shuttle routes for a shift",
        |args| {
            vec![
                rmcp::model::Content::text(format!(
                    "Shuttle summary for {} shift.",
                    args.shift.as_str()
                )),
                rmcp::model::Content::image(IMAGE_DATA, "image/png"),
                audio_content(AUDIO_DATA, "audio/mpeg"),
                resource_link_content("https://example.com/docs", "ignored"),
            ]
        },
    );

    let state = Arc::new(SharedState::new(initial_tool));
    let session_manager = Arc::new(LocalSessionManager::default());
    let service_state = state.clone();
    let service = StreamableHttpService::new(
        move || Ok(StubMcpService::new(service_state.clone())),
        session_manager,
        StreamableHttpServerConfig::default(),
    );

    let app = Router::new()
        .fallback_service(service)
        .layer(middleware::from_fn(require_auth));

    let listener = TcpListener::bind("127.0.0.1:0")
        .await
        .map_err(|err| Box::new(err) as BoxedError)?;
    let addr = listener
        .local_addr()
        .map_err(|err| Box::new(err) as BoxedError)?;
    let url = format!("http://{addr}");

    let (shutdown_tx, shutdown_rx) = oneshot::channel();

    let handle = tokio::spawn(async move {
        let server = axum::serve(listener, app).with_graceful_shutdown(async {
            let _ = shutdown_rx.await;
        });

        if let Err(err) = server.await {
            eprintln!("MCP stub server error: {err}");
        }
    });

    Ok(StubServer {
        url,
        state,
        shutdown: Some(shutdown_tx),
        handle,
    })
}

async fn require_auth(request: Request, next: Next) -> Result<Response, StatusCode> {
    let expected = format!("Bearer {AUTH_TOKEN}");
    if request
        .headers()
        .get(AUTHORIZATION)
        .and_then(|value| value.to_str().ok())
        != Some(expected.as_str())
    {
        return Err(StatusCode::UNAUTHORIZED);
    }
    Ok(next.run(request).await)
}
