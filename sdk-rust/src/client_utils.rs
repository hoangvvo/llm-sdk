use crate::LanguageModelError;
use eventsource_stream::Eventsource;
use futures::{stream::StreamExt, Stream};
use reqwest::Client;
use serde::{de::DeserializeOwned, Serialize};
use std::pin::Pin;

/// Create a JSON request, parse the response.
/// Throws error on non OK status code.
pub async fn send_json<T: Serialize, R: DeserializeOwned>(
    client: &Client,
    url: &str,
    data: &T,
    headers: reqwest::header::HeaderMap,
) -> Result<R, LanguageModelError> {
    let response = client.post(url).headers(headers).json(data).send().await?;
    if response.status().is_client_error() {
        Err(LanguageModelError::StatusCode(
            response.status(),
            response.text().await.unwrap_or_default(),
        ))
    } else {
        Ok(response.json::<R>().await?)
    }
}

/// Create a JSON request that returns an SSE stream.
/// Throws error on non OK status code.
async fn send_sse<T: Serialize>(
    client: &Client,
    url: &str,
    data: &T,
    headers: reqwest::header::HeaderMap,
) -> Result<
    impl StreamExt<
        Item = Result<
            eventsource_stream::Event,
            eventsource_stream::EventStreamError<reqwest::Error>,
        >,
    >,
    LanguageModelError,
> {
    let response = client.post(url).headers(headers).json(data).send().await?;

    if response.status().is_client_error() {
        Err(LanguageModelError::StatusCode(
            response.status(),
            response.text().await.unwrap_or_default(),
        ))
    } else {
        Ok(response.bytes_stream().eventsource())
    }
}

/// Create a JSON request that returns a typed stream of parsed chunks.
/// Handles SSE parsing, JSON deserialization, and error conversion.
/// Automatically handles "[DONE]" termination.
pub async fn send_sse_stream<T: Serialize + 'static, R: DeserializeOwned + Send + 'static>(
    client: &Client,
    url: &str,
    data: &T,
    headers: reqwest::header::HeaderMap,
    provider: &'static str,
) -> Result<Pin<Box<dyn Stream<Item = Result<R, LanguageModelError>> + Send>>, LanguageModelError> {
    let mut sse_stream = send_sse(client, url, data, headers).await?;

    let stream = async_stream::try_stream! {
        while let Some(event) = sse_stream.next().await {
            match event {
                Ok(event) => {
                    if event.data.is_empty() {
                        continue; // Skip empty events
                    }
                    if event.data == "[DONE]" {
                        break; // End of stream
                    }

                    let chunk: R = serde_json::from_str(&event.data)
                        .map_err(|e| {
                            LanguageModelError::Invariant(
                                provider,
                                format!("Failed to parse stream chunk: {e}")
                            )
                        })?;

                    yield chunk;
                }
                Err(e) => {
                    match e {
                        eventsource_stream::EventStreamError::Utf8(_) => {
                            Err(LanguageModelError::Invariant(
                                provider,
                                "Receive invalid UTF-8 sequence for stream data".to_string()
                            ))?;
                        }
                        eventsource_stream::EventStreamError::Parser(error) => {
                            Err(LanguageModelError::Invariant(
                                provider,
                                format!("Receive invalid EventStream data: {error}")
                            ))?;
                        },
                        eventsource_stream::EventStreamError::Transport(e) => {
                            Err(LanguageModelError::Transport(e))?;
                        }
                    }
                }
            }
        }
    };

    Ok(Box::pin(stream))
}
