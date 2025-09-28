use agent::{create_agent, AgentOptions};
use async_stream::stream;
use axum::{
    extract::State,
    http::{HeaderMap, StatusCode},
    response::{sse::Event, Sse},
    routing::{get, post},
    Json, Router,
};
use context::MyContext;
use dotenvy::dotenv;
use futures::{stream::Stream, StreamExt};
use llm_agent::{mcp::MCPParams, AgentRequest, BoxedError};
use llm_sdk::{AudioOptions, LanguageModelMetadata, Modality, ReasoningOptions};
use serde::{Deserialize, Serialize};
use std::{env, time::Duration};
use tower_http::cors::CorsLayer;

mod agent;
mod artifacts_tools;
#[path = "../common/mod.rs"]
mod common;
mod context;
mod finance_tools;
mod information_tools;
mod weather_tools;

#[derive(Clone, Deserialize)]
struct RunStreamBody {
    provider: String,
    model_id: String,
    metadata: LanguageModelMetadata,
    input: AgentRequest<MyContext>,
    enabled_tools: Option<Vec<String>>,
    mcp_servers: Option<Vec<MCPParams>>,
    temperature: Option<f64>,
    top_p: Option<f64>,
    top_k: Option<i32>,
    frequency_penalty: Option<f64>,
    presence_penalty: Option<f64>,
    audio: Option<AudioOptions>,
    reasoning: Option<ReasoningOptions>,
    modalities: Option<Vec<Modality>>,
}

#[derive(Clone, Serialize)]
struct ToolInfo {
    name: String,
    description: String,
}

#[derive(Clone)]
struct AppState {
    available_tools: Vec<ToolInfo>,
}

async fn run_stream_handler(
    headers: HeaderMap,
    Json(body): Json<RunStreamBody>,
) -> Result<Sse<impl Stream<Item = Result<Event, axum::Error>>>, (StatusCode, String)> {
    let api_key = headers
        .get("authorization")
        .and_then(|h| h.to_str().ok())
        .map(std::string::ToString::to_string);

    let RunStreamBody {
        provider,
        model_id,
        metadata,
        input,
        enabled_tools,
        mcp_servers,
        temperature,
        top_p,
        top_k,
        frequency_penalty,
        presence_penalty,
        audio,
        reasoning,
        modalities,
    } = body;

    let model = common::get_model(&provider, &model_id, metadata, api_key)
        .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    if let Some(ref mcp_servers_list) = mcp_servers {
        for mcp_server in mcp_servers_list {
            if matches!(mcp_server, MCPParams::Stdio(_))
                && env::var("ALLOW_STDIO_MCP").unwrap_or_default() != "true"
            {
                return Err((
                    StatusCode::BAD_REQUEST,
                    "Stdio MCP server is not allowed. Set ALLOW_STDIO_MCP=true to allow it."
                        .to_string(),
                ));
            }
        }
    }

    let options = AgentOptions {
        enabled_tools,
        mcp_servers,
        temperature,
        top_p,
        top_k,
        frequency_penalty,
        presence_penalty,
        audio,
        reasoning,
        modalities,
    };

    let agent = create_agent(model, &options);

    // Create a stream that handles the agent run
    let stream = stream! {
        match agent.run_stream(input).await {
            Ok(mut agent_stream) => {
                while let Some(event_result) = agent_stream.next().await {
                    match event_result {
                        Ok(event) => {
                            match serde_json::to_string(&event) {
                                Ok(json) => {
                                    yield Ok(Event::default().data(json));
                                }
                                Err(e) => {
                                    let error_event = serde_json::json!({
                                        "event": "error",
                                        "error": e.to_string()
                                    });
                                    yield Ok(Event::default().data(error_event.to_string()));
                                    break;
                                }
                            }
                        }
                        Err(e) => {
                            let error_event = serde_json::json!({
                                "event": "error",
                                "error": e.to_string()
                            });
                            yield Ok(Event::default().data(error_event.to_string()));
                            break;
                        }
                    }
                }
            }
            Err(e) => {
                let error_event = serde_json::json!({
                    "event": "error",
                    "error": e.to_string()
                });
                yield Ok(Event::default().data(error_event.to_string()));
            }
        }
    };

    Ok(Sse::new(stream).keep_alive(
        axum::response::sse::KeepAlive::new()
            .interval(Duration::from_secs(15))
            .text("keep-alive-text"),
    ))
}

async fn list_tools_handler(State(state): State<AppState>) -> Json<Vec<ToolInfo>> {
    Json(state.available_tools.clone())
}

async fn home_handler() -> &'static str {
    "Welcome to llm-agent-rust Server!\\nGitHub: https://github.com/hoangvvo/llm-sdk"
}

#[tokio::main]
async fn main() -> Result<(), BoxedError> {
    // Load environment variables
    dotenv().ok();

    let available_tools = agent::get_available_tools()
        .iter()
        .map(|tool| ToolInfo {
            name: tool.name(),
            description: tool.description(),
        })
        .collect();

    let state = AppState { available_tools };

    let app_url = env::var("APP_URL").unwrap_or_else(|_| "http://localhost:4321".to_string());

    // Create router
    let app = Router::new()
        .route("/", get(home_handler))
        .route("/run-stream", post(run_stream_handler))
        .route("/tools", get(list_tools_handler))
        .layer(
            CorsLayer::new()
                .allow_origin([app_url.parse().unwrap()])
                .allow_methods(["GET", "POST", "OPTIONS"].map(|m| m.parse().unwrap()))
                .allow_headers(["content-type", "authorization"].map(|h| h.parse().unwrap()))
                .allow_credentials(true),
        )
        .with_state(state);

    // Start server
    let port = env::var("PORT").unwrap_or_else(|_| "4000".to_string());
    let listener = tokio::net::TcpListener::bind(format!("0.0.0.0:{port}"))
        .await
        .map_err(|err| Box::new(err) as BoxedError)?;

    println!("Server listening on http://localhost:{port}");

    axum::serve(listener, app)
        .await
        .map_err(|err| Box::new(err) as BoxedError)?;

    Ok(())
}
