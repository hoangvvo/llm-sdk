use llm_sdk::{
    ContentDelta, ModelResponse, ModelUsage, Part, PartDelta, PartialModelResponse, ReasoningPart,
    ReasoningPartDelta, StreamAccumulator, TextPartDelta, ToolCallPartDelta,
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
        cost: Some(0.30000000000000004),
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
