use axum::{
    body::{Body, Bytes},
    extract::State,
    http::{header::AUTHORIZATION, HeaderMap, StatusCode},
    response::{IntoResponse, Response},
    routing::post,
    Json, Router,
};
use futures::{stream, StreamExt};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    LanguageModel, LanguageModelError, LanguageModelInput, Message, Part, PartDelta,
    ToolResultPart, ToolResultStatus,
};
use serde_json::{json, Value};
use std::{convert::Infallible, sync::Arc};
use tokio::{net::TcpListener, sync::Mutex, task::JoinHandle};

#[derive(Clone, Default)]
struct RequestCapture {
    body: Arc<Mutex<Option<Value>>>,
    authorization: Arc<Mutex<Option<String>>>,
}

struct StubServer {
    base_url: String,
    task: JoinHandle<()>,
}

impl Drop for StubServer {
    fn drop(&mut self) {
        self.task.abort();
    }
}

async fn start_server(router: Router) -> StubServer {
    let listener = TcpListener::bind("127.0.0.1:0")
        .await
        .expect("bind stub server");
    let address = listener.local_addr().expect("stub server address");
    let task = tokio::spawn(async move {
        axum::serve(listener, router)
            .await
            .expect("serve recorded responses");
    });
    StubServer {
        base_url: format!("http://{address}/v1"),
        task,
    }
}

fn recorded_input() -> LanguageModelInput {
    LanguageModelInput {
        system_prompt: Some("Be exact".to_string()),
        messages: vec![Message::user(vec![Part::text("Hello")])],
        max_tokens: Some(17),
        temperature: Some(0.2),
        top_p: Some(0.8),
        ..Default::default()
    }
}

fn recorded_model(base_url: &str) -> OpenAIModel {
    OpenAIModel::new(
        "recorded-model",
        OpenAIModelOptions {
            api_key: "test-token".to_string(),
            base_url: Some(base_url.to_string()),
            ..Default::default()
        },
    )
}

fn completed_response(output: &Value, usage: &Value) -> Value {
    json!({
        "created_at": 0,
        "error": null,
        "id": "resp_1",
        "incomplete_details": null,
        "instructions": null,
        "object": "response",
        "output": output,
        "parallel_tool_calls": false,
        "status": "completed",
        "usage": usage
    })
}

async fn generate_handler(
    State(capture): State<RequestCapture>,
    headers: HeaderMap,
    body: Bytes,
) -> Json<Value> {
    *capture.body.lock().await = Some(serde_json::from_slice(&body).expect("request JSON"));
    *capture.authorization.lock().await = headers
        .get(AUTHORIZATION)
        .and_then(|value| value.to_str().ok())
        .map(str::to_string);

    Json(completed_response(
        &json!([{
            "type": "message",
            "id": "msg_1",
            "role": "assistant",
            "status": "completed",
            "content": [{
                "type": "output_text",
                "text": "Recorded response",
                "annotations": [],
                "logprobs": []
            }]
        }]),
        &json!({
            "input_tokens": 4,
            "output_tokens": 2,
            "total_tokens": 6,
            "input_tokens_details": {"cached_tokens": 1},
            "output_tokens_details": {"reasoning_tokens": 0}
        }),
    ))
}

async fn fragmented_stream_handler(body: Bytes) -> Response {
    let request: Value = serde_json::from_slice(&body).expect("request JSON");
    assert_eq!(request["stream"], true);
    let first = json!({
        "type": "response.output_item.added",
        "output_index": 0,
        "sequence_number": 0,
        "item": {
            "type": "function_call",
            "id": "fc_1",
            "call_id": "call_1",
            "name": "lookup",
            "arguments": "",
            "status": "in_progress"
        }
    })
    .to_string();
    let completed = json!({
        "type": "response.completed",
        "sequence_number": 3,
        "response": completed_response(
            &json!([]),
            &json!({
                "input_tokens": 7,
                "output_tokens": 3,
                "total_tokens": 10,
                "input_tokens_details": {"cached_tokens": 0},
                "output_tokens_details": {"reasoning_tokens": 0}
            })
        )
    });
    let chunks = vec![
        Bytes::from(format!("data: {}", &first[..31])),
        Bytes::from(format!("{}\n\n", &first[31..])),
        Bytes::from(format!(
            "data: {}\n\n",
            json!({
                "type": "response.function_call_arguments.delta",
                "item_id": "fc_1",
                "output_index": 0,
                "sequence_number": 1,
                "delta": "{\"city\":"
            })
        )),
        Bytes::from(format!(
            "data: {}\n\n",
            json!({
                "type": "response.function_call_arguments.delta",
                "item_id": "fc_1",
                "output_index": 0,
                "sequence_number": 2,
                "delta": "\"Hanoi\"}"
            })
        )),
        Bytes::from(format!("data: {completed}\n\n")),
        Bytes::from_static(b"data: [DONE]\n\n"),
    ];
    Response::builder()
        .header("content-type", "text/event-stream")
        .body(Body::from_stream(stream::iter(
            chunks.into_iter().map(Ok::<Bytes, Infallible>),
        )))
        .expect("stream response")
}

async fn failure_handler(body: Bytes) -> Response {
    let request: Value = serde_json::from_slice(&body).expect("request JSON");
    if request["stream"] != true {
        return (
            StatusCode::TOO_MANY_REQUESTS,
            Json(json!({"error": {"message": "rate limited"}})),
        )
            .into_response();
    }

    Response::builder()
        .header("content-type", "text/event-stream")
        .body(Body::from("data: {\"type\":\n\n"))
        .expect("malformed stream response")
}

#[tokio::test]
async fn sends_exact_generate_request_and_maps_recorded_response() {
    let capture = RequestCapture::default();
    let server = start_server(
        Router::new()
            .route("/v1/responses", post(generate_handler))
            .with_state(capture.clone()),
    )
    .await;

    let result = recorded_model(&server.base_url)
        .generate(recorded_input())
        .await
        .expect("generate recorded response");

    assert_eq!(
        capture.body.lock().await.as_ref(),
        Some(&json!({
            "model": "recorded-model",
            "input": [{
                "type": "message",
                "role": "user",
                "content": [{"type": "input_text", "text": "Hello"}]
            }],
            "instructions": "Be exact",
            "max_output_tokens": 17,
            "store": false,
            "stream": false,
            "temperature": 0.2,
            "top_p": 0.8
        }))
    );
    assert_eq!(
        capture.authorization.lock().await.as_deref(),
        Some("Bearer test-token")
    );
    assert_eq!(result.content, vec![Part::text("Recorded response")]);
    let usage = result.usage.expect("recorded usage");
    assert_eq!(usage.input_tokens, 4);
    assert_eq!(usage.output_tokens, 2);
}

#[tokio::test]
async fn sends_a_fallback_output_for_an_empty_cancelled_tool_result() {
    let capture = RequestCapture::default();
    let server = start_server(
        Router::new()
            .route("/v1/responses", post(generate_handler))
            .with_state(capture.clone()),
    )
    .await;
    let input = LanguageModelInput {
        messages: vec![
            Message::assistant(vec![Part::tool_call("call_1", "wait", json!({}))]),
            Message::tool(vec![Part::ToolResult(
                ToolResultPart::new("call_1", "wait", Vec::new())
                    .with_status(ToolResultStatus::Cancelled),
            )]),
        ],
        ..Default::default()
    };

    recorded_model(&server.base_url)
        .generate(input)
        .await
        .expect("generate recorded response");

    let body = capture.body.lock().await;
    let outputs: Vec<Value> = body
        .as_ref()
        .and_then(|body| body["input"].as_array())
        .expect("request input")
        .iter()
        .filter(|item| item["type"] == "function_call_output")
        .cloned()
        .collect();
    assert_eq!(
        outputs,
        vec![json!({
            "type": "function_call_output",
            "call_id": "call_1",
            "output": "cancelled"
        })]
    );
}

#[tokio::test]
async fn parses_fragmented_sse_split_tool_args_and_usage_only_chunks() {
    let server =
        start_server(Router::new().route("/v1/responses", post(fragmented_stream_handler))).await;
    let mut response_stream = recorded_model(&server.base_url)
        .stream(recorded_input())
        .await
        .expect("create recorded stream");
    let mut partials = Vec::new();
    while let Some(partial) = response_stream.next().await {
        partials.push(partial.expect("decode recorded partial"));
    }

    assert_eq!(partials.len(), 4);
    let Some(PartDelta::ToolCall(initial)) = partials[0].delta.as_ref().map(|delta| &delta.part)
    else {
        panic!("expected initial tool-call delta");
    };
    assert_eq!(initial.id.as_deref(), Some("fc_1"));
    assert_eq!(initial.tool_call_id.as_deref(), Some("call_1"));
    assert_eq!(initial.tool_name.as_deref(), Some("lookup"));
    assert_eq!(initial.args.as_deref(), Some(""));
    let Some(PartDelta::ToolCall(first_args)) = partials[1].delta.as_ref().map(|delta| &delta.part)
    else {
        panic!("expected first argument delta");
    };
    assert_eq!(first_args.args.as_deref(), Some("{\"city\":"));
    let Some(PartDelta::ToolCall(second_args)) =
        partials[2].delta.as_ref().map(|delta| &delta.part)
    else {
        panic!("expected second argument delta");
    };
    assert_eq!(second_args.args.as_deref(), Some("\"Hanoi\"}"));
    assert!(partials[3].delta.is_none());
    let usage = partials[3].usage.as_ref().expect("usage-only partial");
    assert_eq!(usage.input_tokens, 7);
    assert_eq!(usage.output_tokens, 3);
}

#[tokio::test]
async fn surfaces_recorded_http_and_malformed_stream_failures() {
    let server = start_server(Router::new().route("/v1/responses", post(failure_handler))).await;
    let model = recorded_model(&server.base_url);

    let error = model
        .generate(recorded_input())
        .await
        .expect_err("HTTP failure must surface");
    assert!(matches!(
        error,
        LanguageModelError::StatusCode(StatusCode::TOO_MANY_REQUESTS, _)
    ));

    let mut response_stream = model
        .stream(recorded_input())
        .await
        .expect("create malformed stream");
    let error = response_stream
        .next()
        .await
        .expect("malformed event result")
        .expect_err("malformed event must fail");
    assert!(matches!(error, LanguageModelError::Invariant("openai", _)));
}
