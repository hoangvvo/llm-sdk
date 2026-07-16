use dotenvy::dotenv;
use futures::future::BoxFuture;
use llm_agent::{
    Agent, AgentFunctionTool, AgentItem, AgentRequest, AgentResponseStatus, AgentToolResult,
    RunOptions, RunState,
};
use llm_sdk::{JSONSchema, Message, Part};
use serde::Deserialize;
use serde_json::{json, Value};
use std::error::Error;
use tokio::time::{sleep, Duration};
use tokio_util::sync::CancellationToken;

mod common;

struct WaitTool;

#[derive(Deserialize)]
struct WaitArgs {
    seconds: u64,
}

impl AgentFunctionTool<()> for WaitTool {
    fn name(&self) -> String {
        "wait".into()
    }

    fn description(&self) -> String {
        "Wait for a requested number of seconds".into()
    }

    fn parameters(&self) -> JSONSchema {
        json!({
            "type": "object",
            "properties": {
                "seconds": { "type": "integer", "minimum": 1 }
            },
            "required": ["seconds"],
            "additionalProperties": false
        })
    }

    fn execute<'a>(
        &'a self,
        args: Value,
        _context: &'a (),
        state: &'a RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn Error + Send + Sync>>> {
        let cancellation_token = state.cancellation_token().clone();

        Box::pin(async move {
            let args: WaitArgs = serde_json::from_value(args)?;

            // This timer is only for demonstration. Pass the cancellation
            // token to external APIs that accept it directly.
            tokio::select! {
                () = cancellation_token.cancelled() => {
                    Err(std::io::Error::new(
                        std::io::ErrorKind::Interrupted,
                        "wait cancelled",
                    ).into())
                }
                () = sleep(Duration::from_secs(args.seconds)) => {
                    Ok(AgentToolResult {
                        content: vec![Part::text("Finished waiting")],
                        is_error: false,
                    })
                }
            }
        })
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error + Send + Sync>> {
    dotenv().ok();

    let provider = std::env::var("PROVIDER").unwrap_or_else(|_| "openai".to_string());
    let model_id = std::env::var("MODEL").unwrap_or_else(|_| "gpt-5.6-terra".to_string());
    let model = common::get_model(
        &provider,
        &model_id,
        llm_sdk::LanguageModelMetadata::default(),
        None,
    )?;

    let agent = Agent::<()>::builder("CancellableAssistant", model)
        .add_tool(WaitTool)
        .build();

    let cancellation_token = CancellationToken::new();
    let cancel_from_ui = cancellation_token.clone();

    // A Stop button or client disconnect would call cancel().
    tokio::spawn(async move {
        sleep(Duration::from_secs(2)).await;
        cancel_from_ui.cancel();
    });

    let response = agent
        .run(
            AgentRequest {
                context: (),
                input: vec![AgentItem::Message(Message::user(vec![Part::text(
                    "Use the wait tool to wait for 30 seconds.",
                )]))],
            },
            RunOptions::default().with_cancellation_token(cancellation_token),
        )
        .await?;

    match response.status {
        AgentResponseStatus::Cancelled => println!("Run cancelled safely."),
        AgentResponseStatus::Completed => println!("{}", response.text()),
    }

    Ok(())
}
