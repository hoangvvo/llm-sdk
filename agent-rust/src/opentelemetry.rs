use crate::{AgentError, AgentItem, AgentResponse, AgentStream, AgentStreamEvent, AgentToolResult};
use async_stream::try_stream;
use futures::{pin_mut, StreamExt};
use llm_sdk::ModelUsage;
use opentelemetry::trace::Status;
use std::{error::Error, future::Future};
use tracing::{info_span, Span};
use tracing_futures::Instrument;
use tracing_opentelemetry::OpenTelemetrySpanExt;

#[derive(Clone, Copy)]
pub enum AgentSpanMethod {
    Run,
    RunStream,
}

impl AgentSpanMethod {
    fn as_str(self) -> &'static str {
        match self {
            Self::Run => "run",
            Self::RunStream => "run_stream",
        }
    }
}

pub struct AgentSpan {
    span: Span,
    usage: Option<ModelUsage>,
    cost: Option<f64>,
}

impl AgentSpan {
    pub fn new(agent_name: &str, method: AgentSpanMethod) -> Self {
        let method_value = method.as_str();
        let span = match method {
            AgentSpanMethod::Run => info_span!("llm_agent.run"),
            AgentSpanMethod::RunStream => info_span!("llm_agent.run_stream"),
        };
        span.set_attribute("gen_ai.operation.name", "invoke_agent");
        span.set_attribute("gen_ai.agent.name", agent_name.to_string());
        span.set_attribute("llm_agent.method", method_value);

        Self {
            span,
            usage: None,
            cost: None,
        }
    }

    pub fn span(&self) -> Span {
        self.span.clone()
    }

    pub fn on_response(&mut self, response: &AgentResponse) {
        for item in &response.output {
            if let AgentItem::Model(model_response) = item {
                if let Some(usage) = &model_response.usage {
                    let total = self.usage.get_or_insert_with(ModelUsage::default);
                    total.add(usage);
                }
                if let Some(cost) = model_response.cost {
                    *self.cost.get_or_insert(0.0) += cost;
                }
            }
        }
    }

    pub fn on_error(&mut self, error: &(dyn Error + 'static)) {
        self.span
            .set_attribute("exception.message", error.to_string());
        self.span.set_status(Status::error(error.to_string()));
    }

    pub fn on_end(&mut self) {
        if let Some(usage) = &self.usage {
            self.span
                .set_attribute("gen_ai.model.input_tokens", i64::from(usage.input_tokens));
            self.span
                .set_attribute("gen_ai.model.output_tokens", i64::from(usage.output_tokens));
        }

        if let Some(cost) = self.cost {
            self.span.set_attribute("llm_agent.cost", cost);
        }
    }
}

impl Drop for AgentSpan {
    fn drop(&mut self) {
        self.on_end();
    }
}

pub async fn start_tool_span<Fut>(
    tool_call_id: &str,
    tool_name: &str,
    tool_description: &str,
    future: Fut,
) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>>
where
    Fut: Future<Output = Result<AgentToolResult, Box<dyn Error + Send + Sync>>> + Send,
{
    let span = info_span!("llm_agent.tool");
    span.set_attribute("gen_ai.operation.name", "execute_tool");
    span.set_attribute("gen_ai.tool.call.id", tool_call_id.to_string());
    span.set_attribute("gen_ai.tool.name", tool_name.to_string());
    span.set_attribute("gen_ai.tool.description", tool_description.to_string());
    span.set_attribute("gen_ai.tool.type", "function");

    match future.instrument(span.clone()).await {
        Ok(result) => Ok(result),
        Err(err) => {
            span.set_attribute("exception.message", err.to_string());
            span.set_status(Status::error(err.to_string()));
            Err(err)
        }
    }
}

pub async fn trace_agent_run<Fut>(
    agent_name: &str,
    method: AgentSpanMethod,
    future: Fut,
) -> Result<AgentResponse, AgentError>
where
    Fut: Future<Output = Result<AgentResponse, AgentError>> + Send,
{
    let mut span = AgentSpan::new(agent_name, method);
    let result = future.instrument(span.span()).await;

    match &result {
        Ok(response) => span.on_response(response),
        Err(error) => span.on_error(error),
    }

    span.on_end();
    result
}

pub fn trace_agent_stream<S>(agent_name: &str, stream: S) -> AgentStream
where
    S: futures::Stream<Item = Result<AgentStreamEvent, AgentError>> + Send + 'static,
{
    let agent_span = AgentSpan::new(agent_name, AgentSpanMethod::RunStream);
    let span_handle = agent_span.span();

    let instrumented = try_stream! {
        let mut span = agent_span;
        let stream_pin = stream;
        pin_mut!(stream_pin);

        while let Some(event_result) = stream_pin.next().await {
            match event_result {
                Ok(AgentStreamEvent::Response(response)) => {
                    span.on_response(&response);
                    span.on_end();
                    yield AgentStreamEvent::Response(response);
                    return;
                }
                Ok(event) => {
                    yield event;
                }
                Err(err) => {
                    span.on_error(&err);
                    span.on_end();
                    Err(err)?;
                }
            }
        }

        span.on_end();
    }
    .instrument(span_handle);

    AgentStream::from_stream(instrumented)
}
