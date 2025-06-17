use super::{
    content::convert_mcp_content, MCPInit, MCPParams, MCPStdioParams, MCPStreamableHTTPParams,
};
use crate::{
    errors::BoxedError,
    tool::{AgentTool, AgentToolResult},
    toolkit::{Toolkit, ToolkitSession},
    RunState,
};
use anyhow::anyhow;
use async_trait::async_trait;
use llm_sdk;
use rmcp::{
    handler::client::ClientHandler,
    model::{CallToolRequestParam, Tool},
    service::{serve_client, NotificationContext, RoleClient, RunningService},
    transport::{
        child_process::TokioChildProcess,
        streamable_http_client::{
            StreamableHttpClientTransport, StreamableHttpClientTransportConfig,
        },
    },
};
use serde_json::Value;
use std::sync::{Arc, Mutex};
use tokio::process::Command;

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

#[async_trait]
impl<TCtx> Toolkit<TCtx> for MCPToolkit<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    async fn create_session(
        &self,
        context: &TCtx,
    ) -> Result<Box<dyn ToolkitSession<TCtx> + Send + Sync>, BoxedError> {
        let params = self.init.resolve(context).await?;
        let session = MCPToolkitSession::new(params).await?;
        Ok(Box::new(session))
    }
}

/// ToolkitSession implementation that exposes MCP tools to the agent runtime.
struct MCPToolkitSession<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    service: Arc<RunningService<RoleClient, MCPClientHandler<TCtx>>>,
    cache: Arc<ToolCache<TCtx>>,
}

impl<TCtx> MCPToolkitSession<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    async fn new(params: MCPParams) -> Result<Self, BoxedError> {
        let cache = Arc::new(ToolCache::new());
        let handler = MCPClientHandler {
            cache: cache.clone(),
        };

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

        let service_arc = Arc::new(service);
        cache.install_service(service_arc.clone());
        cache.refresh_with_service(&service_arc).await?;

        Ok(Self {
            service: service_arc,
            cache,
        })
    }
}

#[async_trait]
impl<TCtx> ToolkitSession<TCtx> for MCPToolkitSession<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    fn system_prompt(&self) -> Option<String> {
        None
    }

    fn tools(&self) -> Vec<Arc<dyn AgentTool<TCtx>>> {
        self.cache.tools()
    }

    async fn close(self: Box<Self>) -> Result<(), BoxedError> {
        match Arc::try_unwrap(self.service) {
            Ok(service) => {
                let _ = service.cancel().await;
            }
            Err(arc) => {
                arc.cancellation_token().cancel();
            }
        }
        Ok(())
    }
}

struct MCPClientHandler<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    cache: Arc<ToolCache<TCtx>>,
}

impl<TCtx> ClientHandler for MCPClientHandler<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    fn on_tool_list_changed(
        &self,
        _context: NotificationContext<RoleClient>,
    ) -> impl std::future::Future<Output = ()> + Send + '_ {
        let cache = self.cache.clone();
        async move {
            if let Err(err) = cache.refresh().await {
                cache.record_error(err);
            } else {
                cache.clear_error();
            }
        }
    }
}

struct MCPRemoteTool<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    service: Arc<RunningService<RoleClient, MCPClientHandler<TCtx>>>,
    name: String,
    description: String,
    parameters: llm_sdk::JSONSchema,
}

impl<TCtx> MCPRemoteTool<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    fn new(service: Arc<RunningService<RoleClient, MCPClientHandler<TCtx>>>, tool: Tool) -> Self {
        let parameters = Value::Object((*tool.input_schema).clone());
        let description = tool.description.map(|d| d.to_string()).unwrap_or_default();
        Self {
            service,
            name: tool.name.to_string(),
            description,
            parameters,
        }
    }
}

#[async_trait]
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

    async fn execute(
        &self,
        args: Value,
        _context: &TCtx,
        _state: &RunState,
    ) -> Result<AgentToolResult, BoxedError> {
        let arguments = match args {
            Value::Null => None,
            Value::Object(map) => Some(map),
            other => {
                return Err(
                    anyhow!("MCP tool arguments must be an object, received {other}").into(),
                )
            }
        };

        let request = CallToolRequestParam {
            name: self.name.clone().into(),
            arguments,
        };

        let result = self
            .service
            .call_tool(request)
            .await
            .map_err(|err| Box::new(err) as BoxedError)?;

        let content = convert_mcp_content(&result.content)?;
        let is_error = result.is_error.unwrap_or(false);

        Ok(AgentToolResult { content, is_error })
    }
}

// Remove "Bearer " or "bearer " prefix if present because the rmcp library already adds it.
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

struct ToolCache<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    service: Mutex<Option<Arc<RunningService<RoleClient, MCPClientHandler<TCtx>>>>>,
    state: Mutex<ToolCacheState<TCtx>>,
}

struct ToolCacheState<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    tools: Vec<Arc<dyn AgentTool<TCtx>>>,
    error: Option<String>,
}

impl<TCtx> ToolCache<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    fn new() -> Self {
        Self {
            service: Mutex::new(None),
            state: Mutex::new(ToolCacheState {
                tools: Vec::new(),
                error: None,
            }),
        }
    }

    fn install_service(&self, service: Arc<RunningService<RoleClient, MCPClientHandler<TCtx>>>) {
        *self.service.lock().expect("service mutex poisoned") = Some(service);
    }

    async fn refresh(&self) -> Result<(), BoxedError> {
        let service = {
            let guard = self.service.lock().expect("service mutex poisoned");
            guard.clone()
        }
        .ok_or_else(|| anyhow!("MCP service not initialised"))?;

        self.refresh_with_service(&service).await
    }

    async fn refresh_with_service(
        &self,
        service: &Arc<RunningService<RoleClient, MCPClientHandler<TCtx>>>,
    ) -> Result<(), BoxedError> {
        let specs = service
            .peer()
            .list_all_tools()
            .await
            .map_err(|err| Box::new(err) as BoxedError)?;
        let mut new_tools: Vec<Arc<dyn AgentTool<TCtx>>> = Vec::with_capacity(specs.len());
        for spec in specs {
            let remote = MCPRemoteTool::new(service.clone(), spec);
            new_tools.push(Arc::new(remote));
        }

        let mut state = self.state.lock().expect("tool cache lock poisoned");
        state.tools = new_tools;
        state.error = None;
        Ok(())
    }

    fn tools(&self) -> Vec<Arc<dyn AgentTool<TCtx>>> {
        let state = self.state.lock().expect("tool cache lock poisoned");
        if let Some(message) = state.error.as_ref() {
            panic!("mcp tool discovery failed: {message}");
        }
        state.tools.clone()
    }

    fn record_error<E>(&self, err: E)
    where
        E: std::fmt::Display,
    {
        if let Ok(mut state) = self.state.lock() {
            state.error = Some(err.to_string());
        }
    }

    fn clear_error(&self) {
        if let Ok(mut state) = self.state.lock() {
            state.error = None;
        }
    }
}
