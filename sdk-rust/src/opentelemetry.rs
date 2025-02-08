use crate::{
    LanguageModelInput, LanguageModelResult, LanguageModelStream, ModelResponse, ModelUsage,
    PartialModelResponse,
};
use opentelemetry::{
    global::{self, BoxedSpan, BoxedTracer},
    trace::{Span, SpanKind, Status, Tracer},
    Context, KeyValue,
};
use std::time::Instant;

pub struct LMSpan {
    span: BoxedSpan,
    start_time: Instant,
    stream_partial_usage: Option<ModelUsage>,
    time_to_first_token: Option<f64>,
}

fn get_tracer() -> BoxedTracer {
    global::tracer("llm-sdk-rs")
}
impl LMSpan {
    pub fn new(
        provider: &str,
        model_id: &str,
        method: &str,
        input: &LanguageModelInput,
    ) -> (Context, Self) {
        let tracer = get_tracer();
        let mut span = tracer
            .span_builder(format!("llm_sdk.{method}"))
            .with_kind(SpanKind::Client)
            .with_attributes(vec![
                // https://opentelemetry.io/docs/specs/semconv/gen-ai/
                KeyValue::new("gen_ai.operation.name", "generate_content"),
                KeyValue::new("gen_ai.provider.name", provider.to_string()),
                KeyValue::new("gen_ai.request.model", model_id.to_string()),
            ])
            .start(&tracer);

        // Add optional attributes if they exist
        if let Some(seed) = input.seed {
            span.set_attribute(KeyValue::new("gen_ai.request.seed", seed));
        }
        if let Some(frequency_penalty) = input.frequency_penalty {
            span.set_attribute(KeyValue::new(
                "gen_ai.request.frequency_penalty",
                frequency_penalty,
            ));
        }
        if let Some(max_tokens) = input.max_tokens {
            span.set_attribute(KeyValue::new(
                "gen_ai.request.max_tokens",
                i64::from(max_tokens),
            ));
        }
        if let Some(presence_penalty) = input.presence_penalty {
            span.set_attribute(KeyValue::new(
                "gen_ai.request.presence_penalty",
                presence_penalty,
            ));
        }
        if let Some(temperature) = input.temperature {
            span.set_attribute(KeyValue::new("gen_ai.request.temperature", temperature));
        }
        if let Some(top_k) = input.top_k {
            span.set_attribute(KeyValue::new("gen_ai.request.top_k", top_k));
        }
        if let Some(top_p) = input.top_p {
            span.set_attribute(KeyValue::new("gen_ai.request.top_p", top_p));
        }

        let cx = Context::current();

        (
            cx,
            Self {
                span,
                start_time: Instant::now(),
                stream_partial_usage: None,
                time_to_first_token: None,
            },
        )
    }

    pub fn on_end(&mut self) {
        self.span.end();
    }

    pub fn on_response(&mut self, response: &ModelResponse) {
        if let Some(usage) = &response.usage {
            self.span.set_attribute(KeyValue::new(
                "gen_ai.usage.input_tokens",
                i64::from(usage.input_tokens),
            ));
            self.span.set_attribute(KeyValue::new(
                "gen_ai.usage.output_tokens",
                i64::from(usage.output_tokens),
            ));
        }
    }

    pub fn on_stream_partial(&mut self, partial: &PartialModelResponse) {
        if let Some(usage) = &partial.usage {
            if self.stream_partial_usage.is_none() {
                self.stream_partial_usage = Some(ModelUsage {
                    input_tokens: 0,
                    output_tokens: 0,
                    input_tokens_details: None,
                    output_tokens_details: None,
                });
            }
            if let Some(ref mut stream_usage) = self.stream_partial_usage {
                stream_usage.input_tokens += usage.input_tokens;
                stream_usage.output_tokens += usage.output_tokens;
                self.span.set_attribute(KeyValue::new(
                    "gen_ai.usage.input_tokens",
                    i64::from(stream_usage.input_tokens),
                ));
                self.span.set_attribute(KeyValue::new(
                    "gen_ai.usage.output_tokens",
                    i64::from(stream_usage.output_tokens),
                ));
            }
        }
        if partial.delta.is_some() && self.time_to_first_token.is_none() {
            let elapsed = self.start_time.elapsed().as_secs_f64();
            self.time_to_first_token = Some(elapsed);
            self.span
                .set_attribute(KeyValue::new("gen_ai.server.time_to_first_token", elapsed));
        }
    }

    pub fn on_error(&mut self, error: &dyn std::error::Error) {
        self.span.record_error(error);
        self.span.set_status(Status::error(error.to_string()));
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

