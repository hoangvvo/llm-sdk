use crate::RunState;
use futures::future::BoxFuture;
use llm_sdk::{FunctionTool, JSONSchema, Part, Tool, ToolSearchTool, WebSearchTool};
use serde_json::Value;
use std::{error::Error, fmt::Debug, sync::Arc};

/**
 * Agent function tool that can be executed by the agent runtime.
 */
pub trait AgentFunctionTool<TCtx>: Send + Sync {
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
    fn execute<'a>(
        &'a self,
        args: Value,
        context: &'a TCtx,
        state: &'a RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn Error + Send + Sync>>>;
}

impl<TCtx> Debug for dyn AgentFunctionTool<TCtx> {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("AgentFunctionTool")
            .field("name", &self.name())
            .field("description", &self.description())
            .field("parameters", &self.parameters())
            .field("execute", &"Function")
            .finish()
    }
}

pub enum AgentTool<TCtx> {
    Function {
        tool: Arc<dyn AgentFunctionTool<TCtx>>,
        defer_loading: bool,
    },
    WebSearch(WebSearchTool),
    ToolSearch(ToolSearchTool),
}

/// A function tool with agent-specific loading behavior.
pub struct ConfiguredFunctionTool<T> {
    tool: T,
    defer_loading: bool,
}

/// Configures how a function is presented to the model when registered with an
/// agent.
pub trait AgentFunctionToolExt: Sized {
    /// Defers the function definition until hosted tool search discovers it.
    #[must_use]
    fn with_defer_loading(self, defer_loading: bool) -> ConfiguredFunctionTool<Self> {
        ConfiguredFunctionTool {
            tool: self,
            defer_loading,
        }
    }
}

impl<T> AgentFunctionToolExt for T {}

#[doc(hidden)]
pub struct AgentToolArg;
#[doc(hidden)]
pub struct WebSearchToolArg;
#[doc(hidden)]
pub struct ToolSearchToolArg;
#[doc(hidden)]
pub struct FunctionToolArg;
#[doc(hidden)]
pub struct ConfiguredFunctionToolArg;

#[doc(hidden)]
pub trait IntoAgentTool<TCtx, TArg> {
    fn into_agent_tool(self) -> AgentTool<TCtx>;
}

impl<TCtx> AgentTool<TCtx> {
    pub fn function<T>(tool: T) -> Self
    where
        T: AgentFunctionTool<TCtx> + 'static,
    {
        Self::Function {
            tool: Arc::new(tool),
            defer_loading: false,
        }
    }

    #[must_use]
    pub fn web_search(tool: WebSearchTool) -> Self {
        Self::WebSearch(tool)
    }

    #[must_use]
    pub fn name(&self) -> String {
        match self {
            Self::Function { tool, .. } => tool.name(),
            Self::WebSearch(_) => "web_search".to_string(),
            Self::ToolSearch(_) => "tool_search".to_string(),
        }
    }

    pub(crate) fn as_function_tool(&self) -> Option<&Arc<dyn AgentFunctionTool<TCtx>>> {
        match self {
            Self::Function { tool, .. } => Some(tool),
            Self::WebSearch(_) | Self::ToolSearch(_) => None,
        }
    }
}

impl<TCtx> Clone for AgentTool<TCtx> {
    fn clone(&self) -> Self {
        match self {
            Self::Function {
                tool,
                defer_loading,
            } => Self::Function {
                tool: Arc::clone(tool),
                defer_loading: *defer_loading,
            },
            Self::WebSearch(tool) => Self::WebSearch(tool.clone()),
            Self::ToolSearch(tool) => Self::ToolSearch(tool.clone()),
        }
    }
}

#[derive(Clone)]
pub struct AgentToolResult {
    pub content: Vec<Part>,
    pub is_error: bool,
}

impl<TCtx> From<WebSearchTool> for AgentTool<TCtx> {
    fn from(value: WebSearchTool) -> Self {
        Self::web_search(value)
    }
}

impl<TCtx> From<ToolSearchTool> for AgentTool<TCtx> {
    fn from(value: ToolSearchTool) -> Self {
        Self::ToolSearch(value)
    }
}

impl<TCtx, T> From<ConfiguredFunctionTool<T>> for AgentTool<TCtx>
where
    T: AgentFunctionTool<TCtx> + 'static,
{
    fn from(value: ConfiguredFunctionTool<T>) -> Self {
        Self::Function {
            tool: Arc::new(value.tool),
            defer_loading: value.defer_loading,
        }
    }
}

impl<TCtx> IntoAgentTool<TCtx, AgentToolArg> for AgentTool<TCtx> {
    fn into_agent_tool(self) -> Self {
        self
    }
}

impl<TCtx> IntoAgentTool<TCtx, WebSearchToolArg> for WebSearchTool {
    fn into_agent_tool(self) -> AgentTool<TCtx> {
        self.into()
    }
}

impl<TCtx> IntoAgentTool<TCtx, ToolSearchToolArg> for ToolSearchTool {
    fn into_agent_tool(self) -> AgentTool<TCtx> {
        self.into()
    }
}

impl<TCtx, T> IntoAgentTool<TCtx, ConfiguredFunctionToolArg> for ConfiguredFunctionTool<T>
where
    T: AgentFunctionTool<TCtx> + 'static,
{
    fn into_agent_tool(self) -> AgentTool<TCtx> {
        self.into()
    }
}

impl<TCtx, T> IntoAgentTool<TCtx, FunctionToolArg> for T
where
    T: AgentFunctionTool<TCtx> + 'static,
{
    fn into_agent_tool(self) -> AgentTool<TCtx> {
        AgentTool::function(self)
    }
}

impl<TCtx> From<&AgentTool<TCtx>> for Tool {
    fn from(agent_tool: &AgentTool<TCtx>) -> Self {
        match agent_tool {
            AgentTool::Function {
                tool,
                defer_loading,
            } => {
                let function =
                    FunctionTool::new(tool.name(), tool.description(), tool.parameters());
                Self::Function(if *defer_loading {
                    function.with_defer_loading(true)
                } else {
                    function
                })
            }
            AgentTool::WebSearch(tool) => Self::WebSearch(tool.clone()),
            AgentTool::ToolSearch(tool) => Self::ToolSearch(tool.clone()),
        }
    }
}
