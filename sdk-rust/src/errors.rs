use thiserror::Error;

#[derive(Error, Debug)]
pub enum LanguageModelError {
    #[error("Invalid input: {0}")]
    InvalidInput(String),
    /// The request to the provider failed or the parsing of the response
    /// failed.
    #[error("HTTP error: {0}")]
    Request(#[from] reqwest::Error),
    /// The request returns a non-OK status code
    #[error("HTTP status error: {0}")]
    StatusCode(reqwest::StatusCode, reqwest::Response),
    /// The input is not supported by or is incompatible with the model
    /// (e.g. using non text for assistant message parts)
    #[error("Unsupported feature: {0}")]
    Unsupported(String),
    /// The response from the provider was unexpected. (e.g. no choices returned
    /// in an OpenAI completion)
    #[error("Invariant: {0}")]
    Invariant(String),
    /// The model refused to process the input. (e.g. OpenAI refusal)
    #[error("Refusal: {0}")]
    Refusal(String),
}

pub type LanguageModelResult<T> = Result<T, LanguageModelError>;
