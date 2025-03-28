mod common;
use crate::common::cases::RunTestCaseOptions;
use llm_sdk::{openai::*, *};
use std::{env, error::Error, sync::LazyLock};
use tokio::test;

static OPENAI_MODEL: LazyLock<OpenAIChatModel> = LazyLock::new(|| {
    dotenvy::dotenv().ok();

    OpenAIChatModel::new(
        "gpt-4o".to_string(),
        OpenAIChatModelOptions {
            api_key: env::var("OPENAI_API_KEY")
                .expect("OPENAI_API_KEY must be set")
                .to_string(),
            ..Default::default()
        },
    )
});

static OPENAI_AUDIO_MODEL: LazyLock<OpenAIChatModel> = LazyLock::new(|| {
    dotenvy::dotenv().ok();

    OpenAIChatModel::new(
        "gpt-4o-audio-preview".to_string(),
        OpenAIChatModelOptions {
            api_key: env::var("OPENAI_API_KEY")
                .expect("OPENAI_API_KEY must be set")
                .to_string(),
            ..Default::default()
        },
    )
});

test_set!(OPENAI_MODEL, generate_text);

test_set!(OPENAI_MODEL, stream_text);

test_set!(OPENAI_MODEL, generate_with_system_prompt);

test_set!(OPENAI_MODEL, generate_tool_call);

test_set!(OPENAI_MODEL, stream_tool_call);

test_set!(OPENAI_MODEL, generate_text_from_tool_result);

test_set!(OPENAI_MODEL, stream_text_from_tool_result);

test_set!(OPENAI_MODEL, generate_parallel_tool_calls);

test_set!(OPENAI_MODEL, stream_parallel_tool_calls);

test_set!(OPENAI_MODEL, stream_parallel_tool_calls_same_name);

test_set!(OPENAI_MODEL, structured_response_format);

test_set!(OPENAI_MODEL, source_part_input);

test_set!(
    OPENAI_AUDIO_MODEL,
    generate_audio,
    Some(RunTestCaseOptions {
        additional_input: Some(|input| {
            input.audio = Some(AudioOptions {
                format: Some(AudioFormat::Mp3),
                voice: Some("alloy".to_string()),
                ..Default::default()
            });
        }),
    })
);

test_set!(
    OPENAI_AUDIO_MODEL,
    stream_audio,
    Some(RunTestCaseOptions {
        additional_input: Some(|input| {
            input.audio = Some(AudioOptions {
                format: Some(AudioFormat::Linear16),
                voice: Some("alloy".to_string()),
                ..Default::default()
            });
        }),
    })
);

test_set!(
    ignore = "reasoning is not supported in chat completion api",
    OPENAI_MODEL,
    generate_reasoning
);

test_set!(
    ignore = "reasoning is not supported in chat completion api",
    OPENAI_MODEL,
    stream_reasoning
);
