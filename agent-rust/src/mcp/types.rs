use crate::errors::BoxedError;
use futures::future::BoxFuture;
use serde::{Deserialize, Serialize};
use std::sync::Arc;

/// Either a fixed MCP configuration or a resolver that derives parameters from
/// the agent context.
#[derive(Clone)]
#[allow(clippy::type_complexity)]
pub enum MCPInit<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    Params(MCPParams),
    Func(Arc<dyn Fn(&TCtx) -> Result<MCPParams, BoxedError> + Send + Sync>),
    AsyncFunc(
        Arc<dyn Fn(&TCtx) -> BoxFuture<'static, Result<MCPParams, BoxedError>> + Send + Sync>,
    ),
}

impl<TCtx> MCPInit<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    /// Returns an init that always yields the supplied parameters.
    #[must_use]
    pub fn from_params(params: MCPParams) -> Self {
        Self::Params(params)
    }

    /// Returns an init backed by the provided synchronous resolver function.
    pub fn from_fn<F>(func: F) -> Self
    where
        F: Fn(&TCtx) -> Result<MCPParams, BoxedError> + Send + Sync + 'static,
    {
        Self::Func(Arc::new(func))
    }

    /// Convenience helper to build an init from an async closure.
    pub fn from_async_fn<F, Fut>(func: F) -> Self
    where
        F: Fn(&TCtx) -> Fut + Send + Sync + 'static,
        Fut: std::future::Future<Output = Result<MCPParams, BoxedError>> + Send + 'static,
    {
        Self::AsyncFunc(Arc::new(move |ctx| Box::pin(func(ctx))))
    }

    /// Resolve parameters for the supplied context.
    pub(crate) async fn resolve(&self, context: &TCtx) -> Result<MCPParams, BoxedError> {
        match self {
            Self::Params(params) => Ok(params.clone()),
            Self::Func(func) => func(context),
            Self::AsyncFunc(func) => func(context).await,
        }
    }
}

impl<TCtx> From<MCPParams> for MCPInit<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    fn from(value: MCPParams) -> Self {
        Self::from_params(value)
    }
}

impl<TCtx, F> From<F> for MCPInit<TCtx>
where
    TCtx: Send + Sync + 'static,
    F: Fn(&TCtx) -> Result<MCPParams, BoxedError> + Send + Sync + 'static,
{
    fn from(value: F) -> Self {
        Self::from_fn(value)
    }
}

// For async resolvers use `MCPInit::from_async_fn` explicitly to preserve
// clarity.

/// `MCPParams` describes how to reach an MCP server.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "kebab-case")]
pub enum MCPParams {
    Stdio(MCPStdioParams),
    StreamableHttp(MCPStreamableHTTPParams),
}

/// Launches a local MCP server via stdio.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MCPStdioParams {
    /// Executable to spawn (e.g. "uvx").
    pub command: String,
    /// Optional arguments passed to the command.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub args: Vec<String>,
}

/// Connects to a remote MCP server exposing the streamable HTTP transport.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MCPStreamableHTTPParams {
    /// Base URL for the MCP server.
    pub url: String,
    /// Authorization header value when required. OAuth flows are not automated,
    /// so supply a token directly.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub authorization: Option<String>,
}
