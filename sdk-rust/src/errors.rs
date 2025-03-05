use thiserror::Error;

#[derive(Error, Debug)]
pub enum LanguageModelError {
    #[error("Invalid input: {0}")]
    InvalidInput(String),
    /// The request to the provider failed or the parsing of the response
    /// failed.
    #[error("HTTP error: {0}")]
    Request(#[from] reqwest::Error),
    /// The provider returns a 4xx or 5xx error.
    #[error("Client error: {0}")]
    ClientError(String),
    /// The response from the provider was unexpected. (e.g. no choices returned
    /// in an OpenAI completion)
    #[error("Invariant: {0}")]
    Invariant(String),
    /// The model refused to process the input. (e.g. OpenAI refusal)
    #[error("Refusal: {0}")]
    Refusal(String),
}

pub type LanguageModelResult<T> = Result<T, LanguageModelError>;
