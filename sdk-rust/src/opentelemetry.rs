use crate::{
    LanguageModelInput, LanguageModelResult, LanguageModelStream, ModelResponse, ModelUsage,
    PartialModelResponse,
};
use futures::StreamExt;
use opentelemetry::trace::Status;
use std::time::Instant;
use tracing::{info_span, Span};
use tracing_futures::Instrument;
use tracing_opentelemetry::OpenTelemetrySpanExt;

pub struct LmSpan {
    span: Span,
    usage: Option<ModelUsage>,
    cost: Option<f64>,
    start_time: Instant,
    time_to_first_token: Option<f64>,
    max_tokens: Option<u32>,
    temperature: Option<f64>,
    top_p: Option<f64>,
    top_k: Option<i32>,
    presence_penalty: Option<f64>,
    frequency_penalty: Option<f64>,
    seed: Option<i64>,
}

impl LmSpan {
    pub fn new(provider: &str, model_id: &str, method: &str, input: &LanguageModelInput) -> Self {
        let span = if method == "stream" {
            info_span!("llm_sdk.stream")
        } else {
            info_span!("llm_sdk.generate")
        };
        span.set_attribute("gen_ai.operation.name", "generate_content");
        span.set_attribute("gen_ai.provider.name", provider.to_string());
        span.set_attribute("gen_ai.request.model", model_id.to_string());
        span.set_attribute("llm_sdk.method", method.to_string());

        Self {
            span,
            usage: None,
            cost: None,
            start_time: Instant::now(),
            time_to_first_token: None,
            max_tokens: input.max_tokens,
            temperature: input.temperature,
            top_p: input.top_p,
            top_k: input.top_k,
            presence_penalty: input.presence_penalty,
            frequency_penalty: input.frequency_penalty,
            seed: input.seed,
        }
    }

    fn span(&self) -> Span {
        self.span.clone()
    }

    pub async fn instrument_future<F>(&self, future: F) -> F::Output
    where
        F: std::future::Future,
    {
        future.instrument(self.span()).await
    }

    pub fn on_response(&mut self, response: &ModelResponse) {
        if let Some(usage) = &response.usage {
            self.usage = Some(usage.clone());
        }
        if let Some(cost) = response.cost {
            self.cost = Some(cost);
        }
    }

    pub fn on_stream_partial(&mut self, partial: &PartialModelResponse) {
        if let Some(usage) = &partial.usage {
            self.usage
                .get_or_insert_with(ModelUsage::default)
                .add(usage);
        }
        if let Some(cost) = partial.cost {
            *self.cost.get_or_insert(0.0) += cost;
        }
        if partial.delta.is_some() && self.time_to_first_token.is_none() {
            self.time_to_first_token = Some(self.elapsed_seconds());
        }
    }

    pub fn on_error(&mut self, error: &(dyn std::error::Error + 'static)) {
        self.span
            .set_attribute("exception.message", error.to_string());
        self.span.set_status(Status::error(error.to_string()));
    }

    pub fn on_end(&mut self) {
        if let Some(usage) = &self.usage {
            self.span
                .set_attribute("gen_ai.usage.input_tokens", i64::from(usage.input_tokens));
            self.span
                .set_attribute("gen_ai.usage.output_tokens", i64::from(usage.output_tokens));
        }

        if let Some(cost) = self.cost {
            self.span.set_attribute("llm_sdk.cost", cost);
        }

        if let Some(time_to_first_token) = self.time_to_first_token {
            self.span
                .set_attribute("gen_ai.server.time_to_first_token", time_to_first_token);
        }

        if let Some(max_tokens) = self.max_tokens {
            self.span
                .set_attribute("gen_ai.request.max_tokens", i64::from(max_tokens));
        }
        if let Some(temperature) = self.temperature {
            self.span
                .set_attribute("gen_ai.request.temperature", temperature);
        }
        if let Some(top_p) = self.top_p {
            self.span.set_attribute("gen_ai.request.top_p", top_p);
        }
        if let Some(top_k) = self.top_k {
            self.span
                .set_attribute("gen_ai.request.top_k", i64::from(top_k));
        }
        if let Some(presence_penalty) = self.presence_penalty {
            self.span
                .set_attribute("gen_ai.request.presence_penalty", presence_penalty);
        }
        if let Some(frequency_penalty) = self.frequency_penalty {
            self.span
                .set_attribute("gen_ai.request.frequency_penalty", frequency_penalty);
        }
        if let Some(seed) = self.seed {
            self.span.set_attribute("gen_ai.request.seed", seed);
        }
    }

    fn elapsed_seconds(&self) -> f64 {
        self.start_time.elapsed().as_secs_f64()
    }
}

impl Drop for LmSpan {
    fn drop(&mut self) {
        self.on_end();
    }
}

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
    let mut span = LmSpan::new(provider, model_id, "generate", &input);
    let result = span.instrument_future(f(input)).await;

    match &result {
        Ok(response) => span.on_response(response),
        Err(error) => span.on_error(error),
    }

    span.on_end();
    result
}

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
    let mut span = LmSpan::new(provider, model_id, "stream", &input);
    let stream_result = span.instrument_future(f(input)).await;

    match stream_result {
        Ok(mut stream) => {
            let span_handle = span.span();
            let streaming_span = span;
            let instrumented = async_stream::try_stream! {
                let mut span_state = streaming_span;

                while let Some(item) = stream.next().await {
                    match item {
                        Ok(partial) => {
                            span_state.on_stream_partial(&partial);
                            yield partial;
                        }
                        Err(err) => {
                            span_state.on_error(&err);
                            Err(err)?;
                        }
                    }
                }
            }
            .instrument(span_handle);

            Ok(LanguageModelStream::from_stream(instrumented))
        }
        Err(error) => {
            span.on_error(&error);
            span.on_end();
            Err(error)
        }
    }
}
