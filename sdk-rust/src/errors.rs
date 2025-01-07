use thiserror::Error;

#[derive(Error, Debug)]
pub enum LanguageModelError {
    #[error("Invalid input: {0}")]
    InvalidInput(String),
    /// The request to the provider failed or the parsing of the response
    /// failed.
    #[error("Transport error: {0}")]
    Transport(#[from] reqwest::Error),
    /// The request returns a non-OK status code
    #[error("Status error: {1} (Status {0})")]
    StatusCode(reqwest::StatusCode, String),
    /// The input is not supported by or is incompatible with the model
    /// (e.g. using non text for assistant message parts)
    #[error("Unsupported by {0}: {1}")]
    Unsupported(&'static str, String),
    /// An output from the model is not recognized by the library.
    /// Please report this issue to the library maintainers.
    #[error("Not implemented for {0}: {1}")]
    NotImplemented(&'static str, String),
    /// The response from the provider was unexpected. (e.g. no choices returned
    /// in an `OpenAI` completion)
    #[error("Invariant from {0}: {1}")]
    Invariant(&'static str, String),
    /// The model refused to process the input. (e.g. `OpenAI` refusal)
    #[error("Refusal: {0}")]
    Refusal(String),
}

pub type LanguageModelResult<T> = Result<T, LanguageModelError>;
