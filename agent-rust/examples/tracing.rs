use async_trait::async_trait;
use dotenvy::dotenv;
use llm_agent::{Agent, AgentItem, AgentRequest, AgentTool, AgentToolResult, RunState};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    JSONSchema, Message, Part,
};
use opentelemetry::{trace::TracerProvider, KeyValue};
use opentelemetry_otlp::{SpanExporter, WithHttpConfig};
use opentelemetry_sdk::{trace::SdkTracerProvider, Resource};
use reqwest::Client;
use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{json, Value};
use std::{error::Error, sync::Arc, time::Duration};
use tokio::time::sleep;
use tracing::{info, info_span, Level};
use tracing_opentelemetry::OpenTelemetryLayer;
use tracing_subscriber::layer::SubscriberExt;

#[derive(Clone)]
struct TracingContext {
    customer_name: String,
}

#[derive(Debug, Deserialize)]
struct WeatherArgs {
    city: String,
}

struct WeatherTool;

#[async_trait]
impl AgentTool<TracingContext> for WeatherTool {
    fn name(&self) -> String {
        "get_weather".into()
    }

    fn description(&self) -> String {
        "Fetch a short weather summary for a city".into()
    }

    fn parameters(&self) -> JSONSchema {
        json!({
            "type": "object",
            "properties": {
                "city": {
                    "type": "string",
                    "description": "City to fetch the weather for"
                }
            },
            "required": ["city"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: Value,
        _context: &TracingContext,
        _state: &RunState,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: WeatherArgs = serde_json::from_value(args)?;
        // Attach a child span so downstream work is correlated with the agent span.
        let span = info_span!("tools.get_weather", city = %params.city);
        let _guard = span.enter();

        // simulate an internal dependency call while the span is active
        info!("looking up forecast");
        sleep(Duration::from_millis(120)).await;

        let payload = json!({
            "city": params.city,
            "forecast": "Sunny",
            "temperature_c": 24,
        });

        Ok(AgentToolResult {
            content: vec![Part::text(payload.to_string())],
            is_error: false,
        })
    }
}

#[derive(Debug, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct NotifyArgs {
    #[schemars(description = "The phone number to contact")]
    phone_number: String,
    #[schemars(description = "The message content")]
    message: String,
}

struct NotifyTool;

#[async_trait]
impl AgentTool<TracingContext> for NotifyTool {
    fn name(&self) -> String {
        "send_notification".into()
    }

    fn description(&self) -> String {
        "Send a short notification text message".into()
    }

    fn parameters(&self) -> JSONSchema {
        schemars::schema_for!(NotifyArgs).into()
    }

    async fn execute(
        &self,
        args: Value,
        _context: &TracingContext,
        _state: &RunState,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: NotifyArgs = serde_json::from_value(args)?;
        let span = info_span!("tools.send_notification", phone = %params.phone_number);
        let _guard = span.enter();

        // trace the internal formatting + dispatch work
        info!("formatting message");
        sleep(Duration::from_millis(80)).await;
        span.record("notification.message_length", params.message.len() as i64);
        info!("dispatching message");

        let payload = json!({
            "status": "sent",
            "phone_number": params.phone_number,
            "message": params.message,
        });

        Ok(AgentToolResult {
            content: vec![Part::text(payload.to_string())],
            is_error: false,
        })
    }
}

fn init_tracing() -> Result<SdkTracerProvider, Box<dyn Error>> {
    let http_client = Client::builder().build()?;

    let exporter = SpanExporter::builder()
        .with_http()
        .with_http_client(http_client)
        .build()
        .expect("Failed to create OTLP exporter");

    let provider = SdkTracerProvider::builder()
        .with_simple_exporter(exporter)
        .with_resource(
            Resource::builder()
                .with_attribute(KeyValue::new("service.name", "agent-rust-tracing-example"))
                .build(),
        )
        .build();

    let tracer = provider.tracer("agent-rust.examples.tracing");

    let subscriber = tracing_subscriber::registry()
        .with(tracing_subscriber::filter::LevelFilter::from_level(
            Level::DEBUG,
        ))
        .with(tracing_subscriber::fmt::layer())
        .with(OpenTelemetryLayer::new(tracer));

    tracing::subscriber::set_global_default(subscriber)?;

    Ok(provider)
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    dotenv().ok();
    let provider = init_tracing()?;

    let model = Arc::new(OpenAIModel::new(
        "gpt-4o-mini",
        OpenAIModelOptions {
            api_key: std::env::var("OPENAI_API_KEY")?,
            ..Default::default()
        },
    ));

    let agent = Agent::<TracingContext>::builder("Trace Assistant", model)
        // Mirror the guidance used across the JS/Go tracing examples.
        .add_instruction("Coordinate weather updates and notifications for clients.")
        .add_instruction("When a request needs both a forecast and a notification, call get_weather before send_notification and summarize the tool results in your reply.")
        .add_instruction(|ctx: &TracingContext| {
            Ok(format!(
                "When asked to contact someone, include a friendly note from {}.",
                ctx.customer_name
            ))
        })
        .add_tool(WeatherTool)
        .add_tool(NotifyTool)
        .build();

    let context = TracingContext {
        customer_name: "Skyline Tours".into(),
    };

    let query =
        "Please check the weather for Seattle today and text Mia at +1-555-0100 with the summary.";
    let request = AgentRequest {
        context,
        input: vec![AgentItem::Message(Message::user(vec![Part::text(query)]))],
    };

    let response = agent.run(request).await?;
    println!("Agent response: {:#?}", response);

    provider.force_flush().ok();

    drop(provider); // ensure all spans are exported before exit

    Ok(())
}
