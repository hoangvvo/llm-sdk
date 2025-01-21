use crate::{
    run::RunSession, tool::AgentTool, types::AgentStream, AgentError, AgentRequest, AgentResponse,
    InstructionParam,
};
use futures::stream::StreamExt;
use llm_sdk::{LanguageModel, ResponseFormatOption};
use std::sync::Arc;

pub struct Agent<TCtx> {
    /// A unique name for the agent.
    /// The name can only contain letters and underscores.
    pub name: String,
    model: Arc<dyn LanguageModel + Send + Sync>,
    instructions: Arc<Vec<InstructionParam<TCtx>>>,
    tools: Arc<Vec<AgentTool<TCtx>>>,
    response_format: ResponseFormatOption,
    max_turns: usize,
}

impl<TCtx> Agent<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    #[must_use]
    pub fn new(params: AgentParams<TCtx>) -> Self {
        Self {
            name: params.name,
            model: params.model,
            instructions: Arc::new(params.instructions),
            tools: Arc::new(params.tools),
            response_format: params.response_format,
            max_turns: params.max_turns,
        }
    }
    /// Create a stateless one-time run of the agent
    pub async fn run(&self, request: AgentRequest<TCtx>) -> Result<AgentResponse, AgentError> {
        let run_session = self.create_session().await;
        let res = run_session.run(request).await?;
        run_session.finish();
        Ok(res)
    }

    /// Create a stateless one-time streaming run of the agent
    pub async fn run_stream(&self, request: AgentRequest<TCtx>) -> Result<AgentStream, AgentError> {
        let run_session = Arc::new(self.create_session().await);
        let mut stream = run_session.clone().run_stream(request)?;

        let wrapped_stream = async_stream::stream! {
            let run_session = run_session;
            while let Some(item) = stream.next().await {
                yield item;
            }

            if let Ok(session) = Arc::try_unwrap(run_session) {
                session.finish();
            }
        };

        Ok(AgentStream::from_stream(wrapped_stream))
    }

    /// Create a session for stateful multiple runs of the agent
    pub async fn create_session(&self) -> RunSession<TCtx> {
        RunSession::new(
            self.model.clone(),
            self.instructions.clone(),
            self.tools.clone(),
            self.response_format.clone(),
            self.max_turns,
        )
        .await
    }

    pub fn builder(
        name: impl ToString,
        model: Arc<dyn LanguageModel + Send + Sync>,
    ) -> AgentParams<TCtx> {
        AgentParams::new(name, model)
    }
}

/// Parameters required to create a new agent.
/// # Default Values
/// - `instructions`: `vec![]`
/// - `tools`: `vec![]`
/// - `response_format`: `ResponseFormatOption::Text`
/// - `max_turns`: 10
pub struct AgentParams<TCtx> {
    pub name: String,
    /// The default language model to use for the agent.
    pub model: Arc<dyn LanguageModel + Send + Sync>,
    /// Instructions to be added to system messages when executing the agent.
    /// This can include formatting instructions or other guidance for the
    /// agent.
    pub instructions: Vec<InstructionParam<TCtx>>,
    /// The tools that the agent can use to perform tasks.
    pub tools: Vec<AgentTool<TCtx>>,
    /// The expected format of the response. Either text or json.
    pub response_format: ResponseFormatOption,
    /// Max number of turns for agent to run to protect against infinite loops.
    pub max_turns: usize,
}

impl<TCtx> AgentParams<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    pub fn new(name: impl ToString, model: Arc<dyn LanguageModel + Send + Sync>) -> Self {
        Self {
            name: name.to_string(),
            model,
            instructions: Vec::new(),
            tools: Vec::new(),
            response_format: ResponseFormatOption::Text,
            max_turns: 10,
        }
    }

    /// Add an instruction
    pub fn add_instruction(mut self, instruction: impl Into<InstructionParam<TCtx>>) -> Self {
        self.instructions.push(instruction.into());
        self
    }

    /// Set the tools
    pub fn tools(mut self, tools: impl IntoIterator<Item = AgentTool<TCtx>>) -> Self {
        self.tools = tools.into_iter().collect();
        self
    }

    /// Set the response format
    #[must_use]
    pub fn response_format(mut self, response_format: ResponseFormatOption) -> Self {
        self.response_format = response_format;
        self
    }

    /// Set the max turns
    #[must_use]
    pub fn max_turns(mut self, max_turns: usize) -> Self {
        self.max_turns = max_turns;
        self
    }

    #[must_use]
    pub fn build(self) -> Agent<TCtx> {
        Agent::new(self)
    }
}
