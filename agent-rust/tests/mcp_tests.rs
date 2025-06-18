use axum::Router;
use llm_agent::{
    mcp::{MCPParams, MCPStreamableHTTPParams, MCPToolkit},
    Agent, AgentItem, AgentParams, AgentResponse, BoxedError, RunSessionRequest,
};
use llm_sdk::{
    llm_sdk_test::MockLanguageModel, AudioFormat, Message, ModelResponse, Part, UserMessage,
};
use rmcp::{
    handler::server::ServerHandler,
    model::{
        CallToolRequestParam, CallToolResult, Implementation, InitializeRequestParam, JsonObject,
        ListToolsResult, ServerCapabilities, ServerInfo, Tool,
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
use std::{sync::Arc, time::Duration};
use tokio::{
    net::TcpListener,
    sync::{oneshot, RwLock},
    time::sleep,
};

const IMAGE_DATA: &str = "AAEC";
const AUDIO_DATA: &str = "AwQ=";

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
                    authorization: None,
                }))
            }
        }),
    ));

    let session = agent
        .create_session(())
        .await
        .map_err(|err| format!("Failed to create session: {err}"))?;

    let response = session
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage::new(vec![
                Part::text("What's running tonight?"),
            ])))],
        })
        .await
        .map_err(|err| format!("Failed to run agent: {err}"))?;

    let expected = AgentResponse {
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
                is_error: false,
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
async fn agent_refreshes_tools_on_mcp_list_change() -> Result<(), BoxedError> {
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
    model.enqueue_generate(ModelResponse {
        content: vec![Part::tool_call(
            "call_2",
            "list_shuttles_v2",
            tool_args.clone(),
        )],
        ..Default::default()
    });
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Routes synced.")],
        ..Default::default()
    });

    let stub_url = stub.url().to_string();
    let agent = Agent::new(AgentParams::new("mcp-test", model.clone()).add_toolkit(
        MCPToolkit::new({
            let stub_url = stub_url.clone();
            move |(): &()| {
                Ok(MCPParams::StreamableHttp(MCPStreamableHTTPParams {
                    url: stub_url.clone(),
                    authorization: None,
                }))
            }
        }),
    ));

    let session = agent
        .create_session(())
        .await
        .map_err(|err| format!("Failed to create session: {err}"))?;

    let first_response = session
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage::new(vec![
                Part::text("What's running tonight?"),
            ])))],
        })
        .await
        .map_err(|err| format!("Failed to run session: {err}"))?;

    let expected_first = AgentResponse {
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
                is_error: false,
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

    sleep(Duration::from_millis(20)).await;

    let second_response = session
        .run(RunSessionRequest {
            input: vec![AgentItem::Message(Message::User(UserMessage::new(vec![
                Part::text("How about now?"),
            ])))],
        })
        .await
        .map_err(|err| format!("Failed to run session: {err}"))?;

    let expected_second = AgentResponse {
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
                is_error: false,
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
        request: InitializeRequestParam,
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
        _request: Option<rmcp::model::PaginatedRequestParam>,
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
        request: CallToolRequestParam,
        _context: rmcp::service::RequestContext<RoleServer>,
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

            let contents = tool.respond(args);
            Ok(CallToolResult::success(contents))
        }
    }
}

impl SharedState {
    fn server_info() -> ServerInfo {
        ServerInfo {
            server_info: Implementation {
                name: "stub-mcp".to_string(),
                version: "1.0.0".to_string(),
                title: None,
                icons: None,
                website_url: None,
            },
            capabilities: ServerCapabilities::builder()
                .enable_tools()
                .enable_tool_list_changed()
                .build(),
            ..Default::default()
        }
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

    let app = Router::new().fallback_service(service);

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
