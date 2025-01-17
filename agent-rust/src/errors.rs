use thiserror::Error;

#[derive(Debug, Error)]
pub enum AgentError {
    #[error("Language model error: {0}")]
    LanguageModel(#[from] llm_sdk::LanguageModelError),
    #[error("Invariant: {0}")]
    Invariant(String),
    #[error("Tool execution error: {0}")]
    ToolExecution(Box<dyn std::error::Error + Send + Sync>),
}
