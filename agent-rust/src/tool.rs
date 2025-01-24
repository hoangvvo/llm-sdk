use crate::RunState;
use async_trait::async_trait;
use futures::lock::Mutex;
use llm_sdk::{JSONSchema, Part, Tool};
use serde_json::Value;
use std::{error::Error, fmt::Debug, sync::Arc};

/**
 * Agent tool that can be used by the agent to perform specific tasks. Any
 * type that implements the `AgentTool` trait can be used as a tool.
 */
#[async_trait]
pub trait AgentTool<TCtx>: Send + Sync {
    /// Name of the tool.
    fn name(&self) -> String;
    /// A description of the tool to instruct the model how and when to use it.
    fn description(&self) -> String;
    /// The JSON schema of the parameters that the tool accepts. The type must
    /// be "object".
    fn parameters(&self) -> JSONSchema;
    /// The function that will be called to execute the tool with given
    /// parameters and context.
    ///
    /// If the tool throws an error, the agent will be interrupted and the error
    /// will be propagated. To avoid interrupting the agent, the tool must
    /// return an `AgentToolResult` with `is_error` set to true.
    async fn execute(
        &self,
        args: Value,
        context: &TCtx,
        state: Arc<Mutex<RunState>>,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>>;
}

impl<TCtx> Debug for dyn AgentTool<TCtx> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("AgentTool")
            .field("name", &self.name())
            .field("description", &self.description())
            .field("parameters", &self.parameters())
            .field("execute", &"Function")
            .finish()
    }
}

pub struct AgentToolResult {
    pub content: Vec<Part>,
    pub is_error: bool,
}

impl<TCtx> From<&dyn AgentTool<TCtx>> for Tool {
    fn from(agent_tool: &dyn AgentTool<TCtx>) -> Self {
        Self {
            name: agent_tool.name(),
            description: agent_tool.description(),
            parameters: agent_tool.parameters(),
        }
    }
}
