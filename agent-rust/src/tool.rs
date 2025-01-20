use anyhow::Error;
use futures::FutureExt;
use futures_core::future::BoxFuture;
use llm_sdk::{Part, Tool};
use serde::de::DeserializeOwned;
use serde_json::Value;
use std::{fmt::Debug, future::Future, sync::Arc};

use crate::AgentError;

pub type AgentToolFn<TCtx> =
    dyn Fn(Value, Arc<TCtx>) -> BoxFuture<'static, Result<AgentToolResult, Error>> + Send + Sync;

pub struct AgentTool<TCtx> {
    /// Name of the tool.
    /// The name can only contain letters and underscores.
    pub name: String,
    /// The description of the tool, when to use, how to use, and what it does.
    pub description: String,
    /// The JSON schema of the parameters that the tool accepts. The type must
    /// be "object".
    pub parameters: Value,
    /// The function that will be called to execute the tool.
    pub execute: Box<AgentToolFn<TCtx>>,
}

impl<TCtx> AgentTool<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    pub fn new<TArgs, F, Fut>(
        name: impl Into<String>,
        description: impl Into<String>,
        parameters: Value,
        execute: F,
    ) -> Self
    where
        F: Fn(TArgs, Arc<TCtx>) -> Fut + Send + Sync + 'static,
        Fut: Future<Output = Result<AgentToolResult, Error>> + Send + 'static,
        TArgs: DeserializeOwned + Send + 'static,
    {
        let execute_arc = Arc::new(execute);

        Self {
            name: name.into(),
            description: description.into(),
            parameters,
            execute: Box::new(move |arg, ctx| {
                let execute = execute_arc.clone();
                async move {
                    let params: TArgs = serde_json::from_value(arg).map_err(|e| {
                        AgentError::Invariant(format!("Failed to deserialize tool arguments: {e}"))
                    })?;
                    execute(params, ctx).await
                }
                .boxed()
            }),
        }
    }
    /// Executes the tool with the given arguments and context.
    /// The arguments are deserialized from JSON to the specified type.
    pub async fn call(&self, args: Value, context: Arc<TCtx>) -> Result<AgentToolResult, Error> {
        (self.execute)(args, context).await
    }
}

impl<TCtx> Debug for AgentTool<TCtx> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("AgentTool")
            .field("name", &self.name)
            .field("description", &self.description)
            .field("parameters", &self.parameters)
            .field("execute", &"Function")
            .finish()
    }
}

pub struct AgentToolResult {
    pub content: Vec<Part>,
    pub is_error: bool,
}

impl<TCtx> From<&AgentTool<TCtx>> for Tool {
    fn from(agent_tool: &AgentTool<TCtx>) -> Self {
        Self {
            name: agent_tool.name.clone(),
            description: agent_tool.description.clone(),
            parameters: agent_tool.parameters.clone(),
        }
    }
}
