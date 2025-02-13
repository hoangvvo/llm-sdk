use thiserror::Error;

#[derive(Debug, Error)]
pub enum AgentError {
    #[error("Language model error: {0}")]
    LanguageModel(#[from] llm_sdk::LanguageModelError),
    #[error("Invariant: {0}")]
    Invariant(String),
    #[error("Tool execution error: {0}")]
    ToolExecution(#[source] BoxedError),
    #[error("Run initialization error: {0}")]
    Init(#[source] BoxedError),
    #[error("The maximum number of turns ({0}) has been exceeded.")]
    MaxTurnsExceeded(usize),
}

pub(crate) type BoxedError = Box<dyn std::error::Error + Send + Sync>;
