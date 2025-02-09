use crate::{
    LanguageModelInput, LanguageModelResult, LanguageModelStream, ModelResponse, ModelUsage,
    PartialModelResponse,
};
use opentelemetry::{
    global::{self, BoxedSpan, BoxedTracer},
    trace::{Span, Status, Tracer},
    Context, KeyValue,
};
use serde::Serialize;
use std::{sync::LazyLock, time::SystemTime};

#[derive(Serialize)]
pub struct LMSpan {
    provider: String,
    model_id: String,
    usage: Option<ModelUsage>,
    cost: Option<f64>,
    start_time: SystemTime,
    /// Time to first token, in seconds
    time_to_first_token: Option<f64>,
    max_tokens: Option<u32>,
    temperature: Option<f64>,
    top_p: Option<f64>,
    top_k: Option<f64>,
    presence_penalty: Option<f64>,
    frequency_penalty: Option<f64>,
    seed: Option<i64>,

    #[serde(skip)]
    span: BoxedSpan,
}

static TRACER: LazyLock<BoxedTracer> = LazyLock::new(|| global::tracer("llm-sdk-rs"));

impl LMSpan {
    pub fn new(
        provider: &str,
        model_id: &str,
        method: &str,
        input: &LanguageModelInput,
    ) -> (Context, Self) {
        let span = TRACER
            .span_builder(format!("llm_sdk.{method}"))
            .start(&*TRACER);

        let cx = Context::current();

        (
            cx,
            Self {
                provider: provider.to_string(),
                model_id: model_id.to_string(),
                usage: None,
                cost: None,
                start_time: SystemTime::now(),
                time_to_first_token: None,
                max_tokens: input.max_tokens,
                temperature: input.temperature,
                top_p: input.top_p,
                top_k: input.top_k,
                presence_penalty: input.presence_penalty,
                frequency_penalty: input.frequency_penalty,
                seed: input.seed,
                span,
            },
        )
    }

    pub fn on_response(&mut self, response: &ModelResponse) {
        if let Some(usage) = &response.usage {
            self.usage = Some(usage.clone());
        }
    }

    pub fn on_stream_partial(&mut self, partial: &PartialModelResponse) {
        if let Some(usage) = &partial.usage {
            if let Some(ref mut usage) = self.usage {
                usage.input_tokens += usage.input_tokens;
                usage.output_tokens += usage.output_tokens;
            } else {
                self.usage = Some(usage.clone());
            }
        }
        if partial.delta.is_some() && self.time_to_first_token.is_none() {
            self.time_to_first_token = Some(
                self.start_time
                    .elapsed()
                    .map(|d| d.as_secs_f64())
                    .unwrap_or(0.0),
            );
        }
    }

    pub fn on_error(&mut self, error: &dyn std::error::Error) {
        self.span.record_error(error);
        self.span.set_status(Status::error(error.to_string()));
    }

    pub fn on_end(&mut self) {
        // https://opentelemetry.io/docs/specs/semconv/gen-ai/
        self.span.set_attributes(vec![
            KeyValue::new("gen_ai.operation.name", "generate_content"),
            KeyValue::new("gen_ai.provider.name", self.provider.clone()),
            KeyValue::new("gen_ai.request.model", self.model_id.clone()),
        ]);
        if let Some(usage) = &self.usage {
            self.span.set_attribute(KeyValue::new(
                "gen_ai.usage.input_tokens",
                i64::try_from(usage.input_tokens).unwrap_or_default(),
            ));
            self.span.set_attribute(KeyValue::new(
                "gen_ai.usage.output_tokens",
                i64::try_from(usage.output_tokens).unwrap_or_default(),
            ));
        }
        if let Some(time_to_first_token) = self.time_to_first_token {
            self.span.set_attribute(KeyValue::new(
                "gen_ai.server.time_to_first_token",
                time_to_first_token,
            ));
        }
        if let Some(max_tokens) = self.max_tokens {
            self.span.set_attribute(KeyValue::new(
                "gen_ai.request.max_tokens",
                i64::try_from(max_tokens).unwrap_or_default(),
            ));
        }
        if let Some(temperature) = self.temperature {
            self.span
                .set_attribute(KeyValue::new("gen_ai.request.temperature", temperature));
        }
        if let Some(top_p) = self.top_p {
            self.span
                .set_attribute(KeyValue::new("gen_ai.request.top_p", top_p));
        }
        if let Some(top_k) = self.top_k {
            self.span
                .set_attribute(KeyValue::new("gen_ai.request.top_k", top_k));
        }
        if let Some(presence_penalty) = self.presence_penalty {
            self.span.set_attribute(KeyValue::new(
                "gen_ai.request.presence_penalty",
                presence_penalty,
            ));
        }
        if let Some(frequency_penalty) = self.frequency_penalty {
            self.span.set_attribute(KeyValue::new(
                "gen_ai.request.frequency_penalty",
                frequency_penalty,
            ));
        }
        if let Some(seed) = self.seed {
            self.span
                .set_attribute(KeyValue::new("gen_ai.request.seed", seed));
        }
        self.span.end();
    }
}

/// Wrapper for streams to add tracing
pub struct TracedStream {
    inner: LanguageModelStream,
    span: Option<LMSpan>,
}

impl TracedStream {
    pub fn new(inner: LanguageModelStream, span: LMSpan) -> Self {
        Self {
            inner,
            span: Some(span),
        }
    }
}

impl futures::Stream for TracedStream {
    type Item = LanguageModelResult<PartialModelResponse>;

    fn poll_next(
        mut self: std::pin::Pin<&mut Self>,
        cx: &mut std::task::Context<'_>,
    ) -> std::task::Poll<Option<Self::Item>> {
        let poll_result = std::pin::Pin::new(&mut self.inner).poll_next(cx);

        match &poll_result {
            std::task::Poll::Ready(Some(Ok(partial))) => {
                if let Some(ref mut span) = self.span {
                    span.on_stream_partial(partial);
                }
            }
            std::task::Poll::Ready(Some(Err(error))) => {
                if let Some(ref mut span) = self.span {
                    span.on_error(error);
                }
            }
            std::task::Poll::Ready(None) => {
                // Stream ended, finish the span
                if let Some(mut span) = self.span.take() {
                    span.on_end();
                }
            }
            _ => {}
        }

        poll_result
    }
}

/// Helper function to wrap generate calls with tracing
pub async fn trace_generate<F, Fut>(
    provider: &str,
    model_id: &str,
    input: LanguageModelInput,
    f: F,
) -> LanguageModelResult<ModelResponse>
where
    F: FnOnce(LanguageModelInput) -> Fut,
    Fut: std::future::Future<Output = LanguageModelResult<ModelResponse>>,
{
    let (_ctx, mut span) = LMSpan::new(provider, model_id, "generate", &input);

    let result = f(input).await;

    match &result {
        Ok(response) => span.on_response(response),
        Err(error) => span.on_error(error),
    }

    span.on_end();
    result
}

/// Helper function to wrap stream calls with tracing
pub async fn trace_stream<F, Fut>(
    provider: &str,
    model_id: &str,
    input: LanguageModelInput,
    f: F,
) -> LanguageModelResult<LanguageModelStream>
where
    F: FnOnce(LanguageModelInput) -> Fut,
    Fut: std::future::Future<Output = LanguageModelResult<LanguageModelStream>>,
{
    let (_ctx, span) = LMSpan::new(provider, model_id, "stream", &input);

    let stream_result = f(input).await;

    match stream_result {
        Ok(stream) => {
            let traced_stream = TracedStream::new(stream, span);
            Ok(LanguageModelStream::from_stream(traced_stream))
        }
        Err(error) => {
            let mut span = span;
            span.on_error(&error);
            span.on_end();
            Err(error)
        }
    }
}
