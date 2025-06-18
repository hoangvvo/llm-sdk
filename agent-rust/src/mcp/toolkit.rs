use super::{
    content::convert_mcp_content, MCPInit, MCPParams, MCPStdioParams, MCPStreamableHTTPParams,
};
use crate::{
    errors::BoxedError,
    tool::{AgentTool, AgentToolResult},
    toolkit::{Toolkit, ToolkitSession},
    RunState,
};
use futures::future::BoxFuture;
use llm_sdk;
use rmcp::{
    handler::client::ClientHandler,
    model::{CallToolRequestParam, CallToolResult, Tool},
    service::{serve_client, NotificationContext, RoleClient, RunningService},
    transport::{
        child_process::TokioChildProcess,
        streamable_http_client::{
            StreamableHttpClientTransport, StreamableHttpClientTransportConfig,
        },
    },
};
use serde_json::Value;
use std::{
    io::{Error as IoError, ErrorKind},
    sync::{Arc, OnceLock, RwLock, Weak},
};
use tokio::process::Command;

type MCPRunningService<TCtx> = RunningService<RoleClient, MCPToolkitState<TCtx>>;

/// Toolkit implementation backed by the Model Context Protocol.
pub struct MCPToolkit<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    init: MCPInit<TCtx>,
}

impl<TCtx> MCPToolkit<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    pub fn new(init: impl Into<MCPInit<TCtx>>) -> Self {
        Self { init: init.into() }
    }
}

impl<TCtx> Toolkit<TCtx> for MCPToolkit<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    fn create_session<'a>(
        &'a self,
        context: &'a TCtx,
    ) -> BoxFuture<'a, Result<Box<dyn ToolkitSession<TCtx> + Send + Sync>, BoxedError>> {
        Box::pin(async move {
            let params = self.init.resolve(context).await?;
            let session = MCPToolkitSession::new(params).await?;
            let boxed: Box<dyn ToolkitSession<TCtx> + Send + Sync> = Box::new(session);
            Ok(boxed)
        })
    }
}

/// `ToolkitSession` implementation that exposes MCP tools to the agent runtime.
struct MCPToolkitSession<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    service: Arc<MCPRunningService<TCtx>>,
    state: MCPToolkitState<TCtx>,
}

impl<TCtx> MCPToolkitSession<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    async fn new(params: MCPParams) -> Result<Self, BoxedError> {
        let state = MCPToolkitState::new();
        let handler = state.clone();

        let service = match params {
            MCPParams::Stdio(MCPStdioParams { command, args }) => {
                let mut cmd = Command::new(command);
                cmd.args(args);
                let transport = TokioChildProcess::new(cmd)?;
                serve_client(handler, transport).await?
            }
            MCPParams::StreamableHttp(MCPStreamableHTTPParams { url, authorization }) => {
                let mut config = StreamableHttpClientTransportConfig::with_uri(url.clone());
                if let Some(token) = authorization.as_deref() {
                    config = config.auth_header(strip_bearer_prefix(token));
                }
                let transport = StreamableHttpClientTransport::from_config(config);
                serve_client(handler, transport).await?
            }
        };

        let service = Arc::new(service);
        state.register_service(&service);
        state.refresh_with(&service).await?;

        Ok(Self { service, state })
    }
}

impl<TCtx> ToolkitSession<TCtx> for MCPToolkitSession<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    fn system_prompt(&self) -> Option<String> {
        None
    }

    fn tools(&self) -> Vec<Arc<dyn AgentTool<TCtx>>> {
        self.state.tools()
    }

    fn close(self: Box<Self>) -> BoxFuture<'static, Result<(), BoxedError>> {
        Box::pin(async move {
            match Arc::try_unwrap(self.service) {
                Ok(service) => {
                    let _ = service.cancel().await;
                }
                Err(arc) => {
                    arc.cancellation_token().cancel();
                }
            }
            Ok(())
        })
    }
}

struct MCPRemoteTool<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    service: Weak<MCPRunningService<TCtx>>,
    name: String,
    description: String,
    parameters: llm_sdk::JSONSchema,
}

impl<TCtx> MCPRemoteTool<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    fn new(service: &Arc<MCPRunningService<TCtx>>, tool: Tool) -> Self {
        let parameters =
            Value::Object(Arc::try_unwrap(tool.input_schema).unwrap_or_else(|arc| (*arc).clone()));
        let description = tool
            .description
            .map(std::borrow::Cow::into_owned)
            .unwrap_or_default();
        Self {
            service: Arc::downgrade(service),
            name: tool.name.into_owned(),
            description,
            parameters,
        }
    }
}

impl<TCtx> AgentTool<TCtx> for MCPRemoteTool<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    fn name(&self) -> String {
        self.name.clone()
    }

    fn description(&self) -> String {
        self.description.clone()
    }

    fn parameters(&self) -> llm_sdk::JSONSchema {
        self.parameters.clone()
    }

    fn execute(
        &self,
        args: Value,
        _context: &TCtx,
        _state: &RunState,
    ) -> BoxFuture<'_, Result<AgentToolResult, BoxedError>> {
        Box::pin(async move {
            let arguments = match args {
                Value::Null => None,
                Value::Object(map) => Some(map),
                other => {
                    let message = format!("MCP tool arguments must be an object, received {other}");
                    return Err(
                        Box::new(IoError::new(ErrorKind::InvalidInput, message)) as BoxedError
                    );
                }
            };

            let request = CallToolRequestParam {
                name: self.name.clone().into(),
                arguments,
            };

            let Some(service) = self.service.upgrade() else {
                return Err(Box::new(IoError::new(
                    ErrorKind::NotConnected,
                    "MCP service not initialised",
                )) as BoxedError);
            };
            let result = service
                .call_tool(request)
                .await
                .map_err(|err| Box::new(err) as BoxedError)?;

            let CallToolResult {
                content, is_error, ..
            } = result;

            let content = convert_mcp_content(content)?;
            let is_error = is_error.unwrap_or(false);

            Ok(AgentToolResult { content, is_error })
        })
    }
}

// Remove "Bearer " or "bearer " prefix if present because the rmcp library
// already adds it.
fn strip_bearer_prefix(token: &str) -> String {
    let trimmed = token.trim();
    if let Some(rest) = trimmed.strip_prefix("Bearer ") {
        rest.to_string()
    } else if let Some(rest) = trimmed.strip_prefix("bearer ") {
        rest.to_string()
    } else {
        trimmed.to_string()
    }
}

struct MCPToolkitState<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    service: Arc<OnceLock<Weak<MCPRunningService<TCtx>>>>,
    tools: Arc<RwLock<Result<Vec<Arc<dyn AgentTool<TCtx>>>, String>>>,
}

impl<TCtx> Clone for MCPToolkitState<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    fn clone(&self) -> Self {
        Self {
            service: Arc::clone(&self.service),
            tools: Arc::clone(&self.tools),
        }
    }
}

impl<TCtx> MCPToolkitState<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    fn new() -> Self {
        Self {
            service: Arc::new(OnceLock::new()),
            tools: Arc::new(RwLock::new(Ok(Vec::new()))),
        }
    }

    fn register_service(&self, service: &Arc<MCPRunningService<TCtx>>) {
        let _ = self.service.set(Arc::downgrade(service));
    }

    async fn refresh(&self) -> Result<(), BoxedError> {
        let service = self.service()?;
        self.refresh_with(&service).await
    }

    async fn refresh_with(&self, service: &Arc<MCPRunningService<TCtx>>) -> Result<(), BoxedError> {
        let specs = service
            .peer()
            .list_all_tools()
            .await
            .map_err(|err| Box::new(err) as BoxedError)?;

        let mut new_tools: Vec<Arc<dyn AgentTool<TCtx>>> = Vec::with_capacity(specs.len());
        for spec in specs {
            let remote = MCPRemoteTool::new(service, spec);
            new_tools.push(Arc::new(remote));
        }

        let mut guard = self.tools.write().expect("tool registry lock poisoned");
        *guard = Ok(new_tools);
        Ok(())
    }

    fn tools(&self) -> Vec<Arc<dyn AgentTool<TCtx>>> {
        let guard = self.tools.read().expect("tool registry lock poisoned");
        match guard.as_ref() {
            Ok(tools) => tools.clone(),
            Err(message) => panic!("mcp tool discovery failed: {message}"),
        }
    }

    fn record_error<E>(&self, err: E)
    where
        E: std::fmt::Display,
    {
        if let Ok(mut guard) = self.tools.write() {
            *guard = Err(err.to_string());
        }
    }

    fn service(&self) -> Result<Arc<MCPRunningService<TCtx>>, BoxedError> {
        self.service
            .get()
            .and_then(Weak::upgrade)
            .ok_or_else(|| -> BoxedError {
                Box::new(IoError::new(
                    ErrorKind::NotConnected,
                    "MCP service not initialised",
                ))
            })
    }
}

impl<TCtx> ClientHandler for MCPToolkitState<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    fn on_tool_list_changed(
        &self,
        _context: NotificationContext<RoleClient>,
    ) -> impl std::future::Future<Output = ()> + Send + '_ {
        let state = self.clone();
        async move {
            if let Err(err) = state.refresh().await {
                state.record_error(err);
            }
        }
    }
}
