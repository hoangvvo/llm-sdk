use crate::AgentRunSnapshot;
use llm_sdk::LanguageModelError;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum AgentError {
    #[error("Language model error: {source}")]
    LanguageModel {
        #[source]
        source: LanguageModelError,
        snapshot: Option<Box<AgentRunSnapshot>>,
    },
    #[error("Invariant: {message}")]
    Invariant {
        message: String,
        snapshot: Option<Box<AgentRunSnapshot>>,
    },
    #[error("Tool execution error: {source}")]
    ToolExecution {
        #[source]
        source: BoxedError,
        snapshot: Option<Box<AgentRunSnapshot>>,
    },
    #[error("Run initialization error: {source}")]
    Init {
        #[source]
        source: BoxedError,
        snapshot: Option<Box<AgentRunSnapshot>>,
    },
    #[error("Run cleanup error: {source}")]
    Cleanup {
        #[source]
        source: BoxedError,
        snapshot: Option<Box<AgentRunSnapshot>>,
    },
    #[error("The maximum number of turns ({max_turns}) has been exceeded.")]
    MaxTurnsExceeded {
        max_turns: usize,
        snapshot: Option<Box<AgentRunSnapshot>>,
    },
}

impl AgentError {
    #[must_use]
    pub fn language_model(source: LanguageModelError) -> Self {
        Self::LanguageModel {
            source,
            snapshot: None,
        }
    }

    #[must_use]
    pub fn invariant(message: impl Into<String>) -> Self {
        Self::Invariant {
            message: message.into(),
            snapshot: None,
        }
    }

    #[must_use]
    pub fn tool_execution(source: BoxedError) -> Self {
        Self::ToolExecution {
            source,
            snapshot: None,
        }
    }

    #[must_use]
    pub fn init(source: BoxedError) -> Self {
        Self::Init {
            source,
            snapshot: None,
        }
    }

    #[must_use]
    pub fn cleanup(source: BoxedError) -> Self {
        Self::Cleanup {
            source,
            snapshot: None,
        }
    }

    #[must_use]
    pub fn max_turns_exceeded(max_turns: usize) -> Self {
        Self::MaxTurnsExceeded {
            max_turns,
            snapshot: None,
        }
    }

    #[must_use]
    pub fn snapshot(&self) -> Option<&AgentRunSnapshot> {
        match self {
            Self::LanguageModel { snapshot, .. }
            | Self::Invariant { snapshot, .. }
            | Self::ToolExecution { snapshot, .. }
            | Self::Init { snapshot, .. }
            | Self::Cleanup { snapshot, .. }
            | Self::MaxTurnsExceeded { snapshot, .. } => snapshot.as_deref(),
        }
    }

    pub(crate) fn with_snapshot(mut self, value: AgentRunSnapshot) -> Self {
        let snapshot = match &mut self {
            Self::LanguageModel { snapshot, .. }
            | Self::Invariant { snapshot, .. }
            | Self::ToolExecution { snapshot, .. }
            | Self::Init { snapshot, .. }
            | Self::Cleanup { snapshot, .. }
            | Self::MaxTurnsExceeded { snapshot, .. } => snapshot,
        };
        if snapshot.is_none() {
            *snapshot = Some(Box::new(value));
        }
        self
    }
}

impl From<LanguageModelError> for AgentError {
    fn from(source: LanguageModelError) -> Self {
        Self::language_model(source)
    }
}

pub type BoxedError = Box<dyn std::error::Error + Send + Sync>;
