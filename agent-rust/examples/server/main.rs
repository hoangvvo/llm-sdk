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
use get_model::{get_model, get_model_list, ModelInfo};
use llm_agent::{AgentRequest, BoxedError};
use serde::{Deserialize, Serialize};
use std::{env, time::Duration};
use tower_http::cors::{Any, CorsLayer};

mod agent;
mod artifacts_tools;
mod context;
mod finance_tools;
mod get_model;
mod information_tools;
mod weather_tools;

#[derive(Clone, Deserialize)]
struct RunStreamBody {
    provider: String,
    model_id: String,
    input: AgentRequest<MyContext>,
    enabled_tools: Option<Vec<String>>,
    disabled_instructions: Option<bool>,
    temperature: Option<f64>,
    top_p: Option<f64>,
    top_k: Option<i32>,
    frequency_penalty: Option<f64>,
    presence_penalty: Option<f64>,
}

#[derive(Clone, Serialize)]
struct ToolInfo {
    name: String,
    description: String,
}

#[derive(Clone)]
struct AppState {
    model_list: Vec<ModelInfo>,
    available_tools: Vec<ToolInfo>,
}

async fn run_stream_handler(
    State(state): State<AppState>,
    headers: HeaderMap,
    Json(body): Json<RunStreamBody>,
) -> Result<Sse<impl Stream<Item = Result<Event, axum::Error>>>, (StatusCode, String)> {
    let api_key = headers
        .get("authorization")
        .and_then(|h| h.to_str().ok())
        .map(std::string::ToString::to_string);

    let model_info = state
        .model_list
        .iter()
        .find(|m| m.provider == body.provider && m.model_id == body.model_id)
        .ok_or_else(|| {
            (
                StatusCode::BAD_REQUEST,
                format!("Model not found: {} - {}", body.provider, body.model_id),
            )
        })?;

    let model = get_model(
        &body.provider,
        &body.model_id,
        model_info.metadata.clone(),
        api_key,
    )
    .map_err(|e| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;

    let options = AgentOptions {
        enabled_tools: body.enabled_tools,
        disabled_instructions: body.disabled_instructions.unwrap_or(false),
        temperature: body.temperature,
        top_p: body.top_p,
        top_k: body.top_k,
        frequency_penalty: body.frequency_penalty,
        presence_penalty: body.presence_penalty,
        audio: model_info.audio.clone(),
        reasoning: model_info.reasoning.clone(),
        modalities: model_info.modalities.clone(),
    };

    let agent = create_agent(model, model_info, &options);

    // Create a stream that handles the agent run
    let stream = stream! {
        match agent.run_stream(body.input).await {
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

async fn list_models_handler(State(state): State<AppState>) -> Json<Vec<ModelInfo>> {
    Json(state.model_list.clone())
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

    // Initialize state
    let model_list = get_model_list().unwrap_or_else(|e| {
        eprintln!("Warning: Could not load models list: {e}");
        vec![]
    });

    let available_tools = agent::get_available_tools()
        .iter()
        .map(|tool| ToolInfo {
            name: tool.name(),
            description: tool.description(),
        })
        .collect();

    let state = AppState {
        model_list,
        available_tools,
    };

    // Create router
    let app = Router::new()
        .route("/", get(home_handler))
        .route("/run-stream", post(run_stream_handler))
        .route("/models", get(list_models_handler))
        .route("/tools", get(list_tools_handler))
        .layer(
            CorsLayer::new()
                .allow_origin(["http://localhost:4321".parse().unwrap()])
                .allow_methods(Any)
                .allow_headers(Any)
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
