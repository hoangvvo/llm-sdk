mod common;
use crate::common::cases::RunTestCaseOptions;
use llm_sdk::{openai::*, *};
use std::{env, error::Error, sync::OnceLock};
use tokio::test;

fn openai_api_key() -> &'static String {
    static KEY: OnceLock<String> = OnceLock::new();

    KEY.get_or_init(|| {
        dotenvy::dotenv().ok();
        env::var("OPENAI_API_KEY").expect("OPENAI_API_KEY must be set")
    })
}

fn openai_model() -> OpenAIChatModel {
    OpenAIChatModel::new(
        "gpt-4o".to_string(),
        OpenAIChatModelOptions {
            api_key: openai_api_key().clone(),
            ..Default::default()
        },
    )
}

fn openai_audio_model() -> OpenAIChatModel {
    OpenAIChatModel::new(
        "gpt-4o-audio-preview".to_string(),
        OpenAIChatModelOptions {
            api_key: openai_api_key().clone(),
            ..Default::default()
        },
    )
}

test_set!(openai_model(), generate_text);

test_set!(openai_model(), stream_text);

test_set!(openai_model(), generate_with_system_prompt);

test_set!(openai_model(), generate_tool_call);

test_set!(openai_model(), stream_tool_call);

test_set!(openai_model(), generate_text_from_tool_result);

test_set!(openai_model(), stream_text_from_tool_result);

test_set!(openai_model(), generate_parallel_tool_calls);

test_set!(openai_model(), stream_parallel_tool_calls);

test_set!(openai_model(), stream_parallel_tool_calls_same_name);

test_set!(openai_model(), structured_response_format);

test_set!(openai_model(), source_part_input);

test_set!(
    ignore = "chat completion api does not support image generation",
    openai_model(),
    generate_image
);

test_set!(
    ignore = "chat completion api does not support image generation",
    openai_model(),
    stream_image
);

test_set!(
    openai_audio_model(),
    generate_audio,
    Some(RunTestCaseOptions {
        additional_input: Some(|input| {
            input.audio = Some(AudioOptions {
                format: Some(AudioFormat::Mp3),
                voice: Some("alloy".to_string()),
                ..Default::default()
            });
        }),
        ..Default::default()
    })
);

test_set!(
    openai_audio_model(),
    stream_audio,
    Some(RunTestCaseOptions {
        additional_input: Some(|input| {
            input.audio = Some(AudioOptions {
                format: Some(AudioFormat::Linear16),
                voice: Some("alloy".to_string()),
                ..Default::default()
            });
        }),
        ..Default::default()
    })
);

test_set!(
    ignore = "reasoning is not supported in chat completion api",
    openai_model(),
    generate_reasoning
);

test_set!(
    ignore = "reasoning is not supported in chat completion api",
    openai_model(),
    stream_reasoning
);
