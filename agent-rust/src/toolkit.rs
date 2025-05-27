use crate::{errors::BoxedError, AgentTool};
use async_trait::async_trait;
use std::sync::Arc;

/// Toolkit produces a per-session toolkit session that can provide dynamic
/// prompt and tool data.
#[async_trait]
pub trait Toolkit<TCtx>: Send + Sync {
    /// Create a new toolkit session for the supplied context value.
    /// Implementations should also initialize the session with instructions and
    /// tools.
    async fn create_session(
        &self,
        context: &TCtx,
    ) -> Result<Box<dyn ToolkitSession<TCtx> + Send + Sync>, BoxedError>;
}

/// ToolkitSession exposes dynamically resolved tools and system prompt data for
/// a run session.
#[async_trait]
pub trait ToolkitSession<TCtx>: Send + Sync {
    /// Retrieve the current system prompt for the session, if available.
    fn system_prompt(&self) -> Option<String>;
    /// Retrieve the current set of tools that should be available to the
    /// session.
    fn tools(&self) -> Vec<Arc<dyn AgentTool<TCtx>>>;
    /// Release any resources that were allocated for the session.
    async fn close(self: Box<Self>) -> Result<(), BoxedError>;
}
