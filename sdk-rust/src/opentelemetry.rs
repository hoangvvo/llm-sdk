use crate::{LanguageModelInput, ModelResponse, ModelUsage, PartialModelResponse};
use opentelemetry::{
    global::{self, BoxedSpan, BoxedTracer},
    trace::{Span, SpanKind, Status, Tracer},
    Context, KeyValue,
};
use std::{sync::LazyLock, time::Instant};

pub struct SDKSpan {
    span: BoxedSpan,
    start_time: Instant,
    stream_partial_usage: Option<ModelUsage>,
    time_to_first_token: Option<f64>,
}

static TRACER: LazyLock<BoxedTracer> = LazyLock::new(|| global::tracer("llm-sdk-rs"));

impl SDKSpan {
    pub fn new(
        provider: &str,
        model_id: &str,
        method: &str,
        input: &LanguageModelInput,
    ) -> (Context, Self) {
        let mut span = TRACER
            .span_builder(format!("llm_sdk.{method}"))
            .with_kind(SpanKind::Client)
            .with_attributes(vec![
                // https://opentelemetry.io/docs/specs/semconv/gen-ai/
                KeyValue::new("gen_ai.operation.name", "generate_content"),
                KeyValue::new("gen_ai.provider.name", provider.to_string()),
                KeyValue::new("gen_ai.request.model", model_id.to_string()),
            ])
            .start(&*TRACER);

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

    pub fn on_end(&mut self, response: &ModelResponse) {
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
        self.span.end();
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

    pub fn on_stream_end(&mut self) {
        self.span.end();
    }

    pub fn on_error(&mut self, error: &dyn std::error::Error) {
        self.span.record_error(error);
        self.span.set_status(Status::error(error.to_string()));
        self.span.end();
    }
}
