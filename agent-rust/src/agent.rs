use crate::{
    run::{RunSession, RunSessionRequest},
    types::AgentStream,
    AgentError, AgentParams, AgentRequest, AgentResponse,
};
use futures::stream::StreamExt;
use llm_sdk::LanguageModel;
use std::sync::Arc;

pub struct Agent<TCtx> {
    /// A unique name for the agent.
    /// The name can only contain letters and underscores.
    pub name: String,
    params: Arc<AgentParams<TCtx>>,
}

impl<TCtx> Agent<TCtx>
where
    TCtx: Send + Sync + 'static,
{
    #[must_use]
    pub fn new(params: AgentParams<TCtx>) -> Self {
        Self {
            name: params.name.clone(),
            params: Arc::new(params),
        }
    }
    /// Create a one-time run of the agent and generate a response.
    /// A session is created for the run and cleaned up afterwards.
    pub async fn run(&self, request: AgentRequest<TCtx>) -> Result<AgentResponse, AgentError> {
        let AgentRequest { input, context } = request;
        let run_session = self.create_session(context).await?;
        let res = run_session.run(RunSessionRequest { input }).await?;
        run_session.finish();
        Ok(res)
    }

    /// Create a one-time streaming run of the agent and generate a response.
    /// A session is created for the run and cleaned up afterwards.
    pub async fn run_stream(&self, request: AgentRequest<TCtx>) -> Result<AgentStream, AgentError> {
        let AgentRequest { input, context } = request;
        let run_session = Arc::new(self.create_session(context).await?);
        let mut stream = run_session
            .clone()
            .run_stream(RunSessionRequest { input })
            .await?;

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

    /// Create a session for stateful multiple runs of the agent.
    pub async fn create_session(&self, context: TCtx) -> Result<RunSession<TCtx>, AgentError> {
        RunSession::new(self.params.clone(), context).await
    }

    pub fn builder(name: &str, model: Arc<dyn LanguageModel + Send + Sync>) -> AgentParams<TCtx> {
        AgentParams::new(name, model)
    }
}
