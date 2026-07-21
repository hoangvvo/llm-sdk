use llm_sdk::{
    AudioFormat, AudioPartDelta, ContentDelta, ImagePartDelta, ModelResponse, ModelUsage, Part,
    PartDelta, PartialModelResponse, ReasoningPart, ReasoningPartDelta, StreamAccumulator,
    TextPartDelta, ToolCallPartDelta,
};
use serde_json::json;

fn partial(index: usize, part: PartDelta) -> PartialModelResponse {
    PartialModelResponse {
        delta: Some(ContentDelta { index, part }),
        ..Default::default()
    }
}

#[test]
fn reconstructs_interleaved_multipart_stream_and_metadata() {
    let mut accumulator = StreamAccumulator::new();
    let partials = vec![
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 2,
                part: PartDelta::Reasoning(ReasoningPartDelta::default().with_text("think ")),
            }),
            usage: Some(ModelUsage {
                input_tokens: 2,
                output_tokens: 1,
                ..Default::default()
            }),
            cost: Some(0.1),
        },
        partial(0, PartDelta::Text(TextPartDelta::new("Hel"))),
        partial(
            1,
            PartDelta::ToolCall(
                ToolCallPartDelta::default()
                    .with_tool_call_id("call_1")
                    .with_tool_name("weather")
                    .with_args(r#"{"city":"#),
            ),
        ),
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("lo")),
            }),
            usage: Some(ModelUsage {
                input_tokens: 3,
                output_tokens: 4,
                ..Default::default()
            }),
            cost: Some(0.2),
        },
        partial(
            1,
            PartDelta::ToolCall(ToolCallPartDelta::default().with_args(r#""Paris"}"#)),
        ),
        partial(
            2,
            PartDelta::Reasoning(
                ReasoningPartDelta::default()
                    .with_text("done")
                    .with_signature("sig"),
            ),
        ),
    ];

    for response in partials {
        accumulator
            .add_partial(response)
            .expect("partial should accumulate");
    }
    assert_eq!(accumulator.size(), 3);
    assert!(!accumulator.is_empty());

    let response = accumulator
        .compute_response()
        .expect("response should compute");
    let expected = ModelResponse {
        content: vec![
            Part::text("Hello"),
            Part::tool_call("call_1", "weather", json!({"city": "Paris"})),
            Part::Reasoning(ReasoningPart::new("think done").with_signature("sig")),
        ],
        usage: Some(ModelUsage {
            input_tokens: 5,
            output_tokens: 5,
            ..Default::default()
        }),
        cost: Some(0.300_000_000_000_000_04),
    };
    assert_eq!(response, expected);
}

#[test]
fn rejects_mismatched_and_malformed_deltas() {
    let mut mismatch = StreamAccumulator::new();
    mismatch
        .add_partial(partial(0, PartDelta::Text(TextPartDelta::new("hello"))))
        .expect("initial partial should accumulate");
    let error = mismatch
        .add_partial(partial(
            0,
            PartDelta::Reasoning(ReasoningPartDelta::default().with_text("wrong")),
        ))
        .expect_err("part type mismatch should fail");
    assert!(error.contains("Type mismatch at index 0"));

    let mut malformed = StreamAccumulator::new();
    malformed
        .add_partial(partial(
            0,
            PartDelta::ToolCall(
                ToolCallPartDelta::default()
                    .with_tool_call_id("call_1")
                    .with_tool_name("weather")
                    .with_args("{bad json"),
            ),
        ))
        .expect("partial should accumulate before final validation");
    assert!(malformed.compute_response().is_err());
}

#[test]
fn clear_removes_content_and_metadata() {
    let mut accumulator = StreamAccumulator::new();
    accumulator
        .add_partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("old")),
            }),
            usage: Some(ModelUsage {
                input_tokens: 2,
                output_tokens: 1,
                ..Default::default()
            }),
            cost: Some(0.4),
        })
        .expect("partial should accumulate");

    accumulator.clear();
    assert!(accumulator.is_empty());
    let response = accumulator
        .compute_response()
        .expect("cleared response should compute");
    assert_eq!(response, ModelResponse::default());
}

#[test]
fn snapshots_independently_materializable_parts() {
    let mut accumulator = StreamAccumulator::new();
    let partials = vec![
        PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta::new("partial")),
            }),
            usage: Some(ModelUsage {
                input_tokens: 2,
                output_tokens: 3,
                ..Default::default()
            }),
            cost: Some(0.25),
        },
        partial(
            1,
            PartDelta::ToolCall(
                ToolCallPartDelta::default()
                    .with_tool_call_id("call_1")
                    .with_tool_name("weather")
                    .with_args(r#"{"city":"Paris"}"#),
            ),
        ),
        partial(
            2,
            PartDelta::ToolCall(ToolCallPartDelta::default().with_args("{incomplete")),
        ),
        partial(
            3,
            PartDelta::Image(
                ImagePartDelta::default()
                    .with_data("aGVsbG8=")
                    .with_mime_type("image/png"),
            ),
        ),
        partial(
            4,
            PartDelta::Audio(
                AudioPartDelta::default()
                    .with_data("AAABAA==")
                    .with_format(AudioFormat::Linear16),
            ),
        ),
    ];
    for response in partials {
        accumulator
            .add_partial(response)
            .expect("partial should accumulate");
    }

    assert_eq!(
        accumulator.snapshot(),
        ModelResponse {
            content: vec![
                Part::text("partial"),
                Part::tool_call("call_1", "weather", json!({"city": "Paris"})),
                Part::image("aGVsbG8=", "image/png"),
                Part::audio("AAABAA==", AudioFormat::Linear16),
            ],
            usage: Some(ModelUsage {
                input_tokens: 2,
                output_tokens: 3,
                ..Default::default()
            }),
            cost: Some(0.25),
        }
    );
    assert!(accumulator.compute_response().is_err());
}

#[test]
fn accumulates_web_search_call_and_result() {
    let mut accumulator = StreamAccumulator::new();
    accumulator
        .add_partial(partial(
            0,
            PartDelta::ToolCall(llm_sdk::ToolCallPartDelta {
                tool_call_id: Some("ws_1".to_string()),
                call: llm_sdk::ToolCallDelta::WebSearch(llm_sdk::WebSearchToolCallDelta {
                    action: Some(llm_sdk::WebSearchAction::Search {
                        queries: vec!["sdk docs".to_string()],
                    }),
                    status: Some(llm_sdk::WebSearchToolCallStatus::Completed),
                }),
                signature: None,
                id: None,
            }),
        ))
        .unwrap();
    accumulator
        .add_partial(partial(
            1,
            PartDelta::ToolResult(llm_sdk::ToolResultPartDelta {
                tool_call_id: "ws_1".to_string(),
                result: llm_sdk::ToolResult::WebSearch(llm_sdk::WebSearchToolResult {
                    sources: vec![llm_sdk::WebSearchSource {
                        url: "https://example.com".to_string(),
                        title: None,
                        page_age: None,
                        signature: None,
                    }],
                    error_code: None,
                }),
                status: llm_sdk::ToolResultStatus::Completed,
            }),
        ))
        .unwrap();
    let response = accumulator.compute_response().unwrap();
    assert!(
        matches!(response.content[0], Part::ToolCall(ref call) if matches!(call.call, llm_sdk::ToolCall::WebSearch(_)))
    );
    assert!(
        matches!(response.content[1], Part::ToolResult(ref result) if matches!(result.result, llm_sdk::ToolResult::WebSearch(_)))
    );
}
