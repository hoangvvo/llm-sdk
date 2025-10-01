mod common;
use crate::common::cases::RunTestCaseOptions;
use llm_sdk::{anthropic::*, *};
use std::{env, error::Error, sync::OnceLock};
use tokio::test;

fn anthropic_api_key() -> &'static String {
    static KEY: OnceLock<String> = OnceLock::new();

    KEY.get_or_init(|| {
        dotenvy::dotenv().ok();
        env::var("ANTHROPIC_API_KEY").expect("ANTHROPIC_API_KEY must be set")
    })
}

fn anthropic_model() -> AnthropicModel {
    AnthropicModel::new(
        "claude-sonnet-4-20250514".to_string(),
        AnthropicModelOptions {
            api_key: anthropic_api_key().clone(),
            ..Default::default()
        },
    )
}

test_set!(anthropic_model(), generate_text);

test_set!(anthropic_model(), stream_text);

test_set!(anthropic_model(), generate_with_system_prompt);

test_set!(anthropic_model(), generate_tool_call);

test_set!(anthropic_model(), stream_tool_call);

test_set!(anthropic_model(), generate_text_from_tool_result);

test_set!(anthropic_model(), stream_text_from_tool_result);

test_set!(anthropic_model(), generate_parallel_tool_calls);

test_set!(anthropic_model(), stream_parallel_tool_calls);

test_set!(anthropic_model(), stream_parallel_tool_calls_same_name);

test_set!(anthropic_model(), structured_response_format);

test_set!(anthropic_model(), source_part_input);

test_set!(
    ignore = "model does not support image generation",
    anthropic_model(),
    generate_image
);

test_set!(
    ignore = "model does not support image generation",
    anthropic_model(),
    stream_image
);

test_set!(anthropic_model(), generate_image_input);

test_set!(anthropic_model(), stream_image_input);

test_set!(
    ignore = "model does not support audio generation",
    anthropic_model(),
    generate_audio
);

test_set!(
    ignore = "model does not support audio generation",
    anthropic_model(),
    stream_audio
);

test_set!(
    anthropic_model(),
    generate_reasoning,
    Some(RunTestCaseOptions {
        additional_input: Some(|input| {
            input.reasoning = Some(ReasoningOptions {
                enabled: true,
                budget_tokens: Some(3000),
            });
        }),
        ..Default::default()
    })
);

test_set!(
    anthropic_model(),
    stream_reasoning,
    Some(RunTestCaseOptions {
        additional_input: Some(|input| {
            input.reasoning = Some(ReasoningOptions {
                enabled: true,
                budget_tokens: Some(3000),
            });
        }),
        ..Default::default()
    })
);
