use axum::{
    extract::Request,
    http::{header, HeaderMap, Method, StatusCode},
    middleware::{self, Next},
    response::{IntoResponse, Response},
    routing::get,
    Json, Router,
};
use dotenvy::dotenv;
use llm_agent::{
    mcp::{MCPParams, MCPStreamableHTTPParams, MCPToolkit},
    Agent, AgentItem, AgentRequest, BoxedError,
};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    Message, Part,
};
use rmcp::{
    handler::server::{router::tool::ToolRouter, wrapper::Parameters},
    model::{ServerCapabilities, ServerInfo},
    schemars, tool, tool_handler, tool_router,
    transport::streamable_http_server::{
        session::local::LocalSessionManager,
        tower::{StreamableHttpServerConfig, StreamableHttpService},
    },
    ServerHandler,
};
use serde::Deserialize;
use serde_json::json;
use std::{
    env,
    io::{Error as IoError, ErrorKind},
    sync::Arc,
    time::Duration,
};
use tokio::{net::TcpListener, sync::oneshot, task::JoinHandle, time::sleep};

// This example demonstrates:
// 1. Launching a minimal streamable HTTP MCP server using the official Rust
//    SDK.
// 2. Registering that server through the MCP toolkit primitive.
// 3. Having the agent call the remote tool during a conversation.

const SERVER_ADDR: &str = "127.0.0.1:39811";
const SERVER_URL: &str = "http://127.0.0.1:39811";
const AUTH_TOKEN: &'static str = "transit-hub-secret";

#[derive(Clone)]
struct SessionContext {
    rider_name: String,
    authorization: String,
}

#[tokio::main]
async fn main() -> Result<(), BoxedError> {
    dotenv().ok();

    let server = start_stub_mcp_server().await?;
    let run_result = run_agent_demo().await;
    server.shutdown().await?;
    run_result
}

async fn run_agent_demo() -> Result<(), BoxedError> {
    let api_key = env::var("OPENAI_API_KEY").map_err(|_| missing_env("OPENAI_API_KEY"))?;

    let model = Arc::new(OpenAIModel::new(
        "gpt-4o-mini",
        OpenAIModelOptions {
            api_key,
            ..Default::default()
        },
    ));

    let agent = Agent::<SessionContext>::builder("Sage", model)
        .add_instruction("You are Sage, the shuttle concierge for the Transit Hub.")
        .add_instruction(
            "Lean on connected transit systems before guessing, and tailor advice to the rider's \
             shift.",
        )
        .add_instruction(|context: &SessionContext| {
            Ok(format!(
                "You are assisting {} with tonight's shuttle planning.",
                context.rider_name
            ))
        })
        // The MCP toolkit primitive resolves transport params per session. Here we pull the
        // rider-specific authorization token from context so each agent session connects
        // with the correct credentials.
        .add_toolkit(MCPToolkit::new(|context: &SessionContext| {
            Ok(MCPParams::StreamableHttp(MCPStreamableHTTPParams {
                url: SERVER_URL.to_string(),
                authorization: Some(context.authorization.clone()),
            }))
        }))
        .build();

    let request = AgentRequest {
        context: SessionContext {
            rider_name: "Avery".to_string(),
            authorization: AUTH_TOKEN.to_string(),
        },
        input: vec![AgentItem::Message(Message::user(vec![Part::text(
            "What shuttles are running tonight?",
        )]))],
    };

    let response = agent
        .run(request)
        .await
        .map_err(|err| Box::new(err) as BoxedError)?;

    println!("=== Agent Response ===");
    let reply = response.text();
    if reply.is_empty() {
        println!("{:?}", response.content);
    } else {
        println!("{}", reply);
    }

    Ok(())
}

async fn start_stub_mcp_server() -> Result<ServerGuard, BoxedError> {
    let session_manager = Arc::new(LocalSessionManager::default());
    let service: StreamableHttpService<ShuttleServer, _> = StreamableHttpService::new(
        || Ok(ShuttleServer::default()),
        session_manager,
        StreamableHttpServerConfig::default(),
    );

    let app = Router::new()
        .route("/status", get(server_status))
        .fallback_service(service)
        .layer(middleware::from_fn(authenticate));

    let listener = TcpListener::bind(SERVER_ADDR)
        .await
        .map_err(|err| Box::new(err) as BoxedError)?;
    let (shutdown_tx, shutdown_rx) = oneshot::channel();

    let handle = tokio::spawn(async move {
        let server = axum::serve(listener, app).with_graceful_shutdown(async {
            let _ = shutdown_rx.await;
        });

        if let Err(err) = server.await {
            eprintln!("MCP server error: {err}");
        }
    });

    sleep(Duration::from_millis(200)).await;

    Ok(ServerGuard {
        shutdown: Some(shutdown_tx),
        handle,
    })
}

struct ShuttleServer {
    tool_router: ToolRouter<Self>,
}

impl ShuttleServer {
    fn new() -> Self {
        Self {
            tool_router: Self::tool_router(),
        }
    }
}

impl Default for ShuttleServer {
    fn default() -> Self {
        Self::new()
    }
}

#[derive(Debug, Deserialize, schemars::JsonSchema)]
#[serde(deny_unknown_fields)]
struct ListShuttlesArgs {
    #[schemars(description = "Operating window to query")]
    shift: Shift,
}

#[derive(Debug, Deserialize, schemars::JsonSchema)]
#[serde(rename_all = "lowercase")]
#[schemars(inline)]
enum Shift {
    Evening,
    Overnight,
}

#[tool_router]
impl ShuttleServer {
    #[tool(description = "List active shuttle routes for the selected shift")]
    fn list_shuttles(&self, Parameters(args): Parameters<ListShuttlesArgs>) -> String {
        let _ = &self.tool_router;
        match args.shift {
            Shift::Evening => "Midnight Loop and Harbor Express are on duty tonight.".into(),
            Shift::Overnight => {
                "Harbor Express and Dawn Flyer are staged for the overnight shift.".into()
            }
        }
    }
}

#[tool_handler(router = self.tool_router)]
impl ServerHandler for ShuttleServer {
    fn get_info(&self) -> ServerInfo {
        let mut info = ServerInfo::default();
        info.capabilities = ServerCapabilities::builder().enable_tools().build();
        info.server_info.name = "shuttle-scheduler".into();
        info.server_info.title = Some("Transit hub shuttle coordinator".into());
        info.instructions =
            Some("Authenticate with the shuttle control token before calling tools.".into());
        info
    }
}

async fn server_status() -> Json<serde_json::Value> {
    Json(json!({ "status": "ok" }))
}

async fn authenticate(req: Request, next: Next) -> Result<Response, Response> {
    let method = req.method().clone();
    let is_status = req.uri().path() == "/status";
    let authorized = has_valid_token(req.headers(), &method);
    if is_status || authorized {
        Ok(next.run(req).await)
    } else {
        Err((
            StatusCode::UNAUTHORIZED,
            Json(json!({
                "error": "unauthorized",
                "message": "Provide the shuttle access token.",
            })),
        )
            .into_response())
    }
}

fn has_valid_token(headers: &HeaderMap, method: &Method) -> bool {
    if let Some(token) = headers
        .get(header::AUTHORIZATION)
        .and_then(|value| value.to_str().ok())
    {
        if token.trim() == format!("Bearer {}", AUTH_TOKEN) {
            return true;
        }
    }

    matches!(method, &Method::GET | &Method::DELETE) && headers.contains_key("mcp-session-id")
}

struct ServerGuard {
    shutdown: Option<oneshot::Sender<()>>,
    handle: JoinHandle<()>,
}

impl ServerGuard {
    async fn shutdown(self) -> Result<(), BoxedError> {
        if let Some(tx) = self.shutdown {
            let _ = tx.send(());
        }

        match self.handle.await {
            Ok(()) => Ok(()),
            Err(err) => Err(Box::new(err) as BoxedError),
        }
    }
}

fn missing_env(var: &str) -> BoxedError {
    Box::new(IoError::new(
        ErrorKind::NotFound,
        format!("{var} environment variable must be set"),
    ))
}
