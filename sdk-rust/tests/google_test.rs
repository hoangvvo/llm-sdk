mod common;
use crate::common::{assert::PartAssertion, cases::RunTestCaseOptions};
use llm_sdk::{google::*, *};
use std::{env, error::Error, sync::LazyLock};
use tokio::test;

static GOOGLE_MODEL: LazyLock<GoogleModel> = LazyLock::new(|| {
    dotenvy::dotenv().ok();

    GoogleModel::new(
        "gemini-2.5-flash".to_string(),
        GoogleModelOptions {
            api_key: env::var("GOOGLE_API_KEY")
                .expect("GOOGLE_API_KEY must be set")
                .to_string(),
            ..Default::default()
        },
    )
});

static GOOGLE_AUDIO_MODEL: LazyLock<GoogleModel> = LazyLock::new(|| {
    dotenvy::dotenv().ok();

    GoogleModel::new(
        "gemini-2.5-flash-preview-tts".to_string(),
        GoogleModelOptions {
            api_key: env::var("GOOGLE_API_KEY")
                .expect("GOOGLE_API_KEY must be set")
                .to_string(),
            ..Default::default()
        },
    )
});

static GOOGLE_REASONING_MODEL: LazyLock<GoogleModel> = LazyLock::new(|| {
    dotenvy::dotenv().ok();

    GoogleModel::new(
        "gemini-2.0-flash-thinking-exp-01-21".to_string(),
        GoogleModelOptions {
            api_key: env::var("GOOGLE_API_KEY")
                .expect("GOOGLE_API_KEY must be set")
                .to_string(),
            ..Default::default()
        },
    )
});

test_set!(GOOGLE_MODEL, generate_text);

test_set!(GOOGLE_MODEL, stream_text);

test_set!(GOOGLE_MODEL, generate_with_system_prompt);

test_set!(GOOGLE_MODEL, generate_tool_call);

test_set!(GOOGLE_MODEL, stream_tool_call);

test_set!(GOOGLE_MODEL, generate_text_from_tool_result);

test_set!(GOOGLE_MODEL, stream_text_from_tool_result);

test_set!(GOOGLE_MODEL, generate_parallel_tool_calls);

test_set!(GOOGLE_MODEL, stream_parallel_tool_calls);

test_set!(GOOGLE_MODEL, stream_parallel_tool_calls_same_name);

test_set!(GOOGLE_MODEL, structured_response_format);

test_set!(GOOGLE_MODEL, source_part_input);

test_set!(
    GOOGLE_AUDIO_MODEL,
    generate_audio,
    Some(RunTestCaseOptions {
        additional_input: Some(|input| {
            input.modalities = Some(vec![Modality::Audio]);
            input.audio = Some(AudioOptions {
                voice: Some("Zephyr".to_string()),
                ..Default::default()
            });
        }),
        custom_output_content: Some(|content| {
            content
                .iter_mut()
                .map(|part| {
                    if let PartAssertion::Audio(part) = part {
                        part.audio_id = false;
                        part.transcript = None;
                        PartAssertion::Audio(part.clone())
                    } else {
                        part.clone()
                    }
                })
                .collect()
        })
    })
);

test_set!(
    GOOGLE_AUDIO_MODEL,
    stream_audio,
    Some(RunTestCaseOptions {
        additional_input: Some(|input| {
            input.modalities = Some(vec![Modality::Audio]);
            input.audio = Some(AudioOptions {
                voice: Some("Zephyr".to_string()),
                ..Default::default()
            });
        }),
        custom_output_content: Some(|content| {
            content
                .iter_mut()
                .map(|part| {
                    if let PartAssertion::Audio(part) = part {
                        part.audio_id = false;
                        part.transcript = None;
                        PartAssertion::Audio(part.clone())
                    } else {
                        part.clone()
                    }
                })
                .collect()
        })
    })
);

test_set!(GOOGLE_REASONING_MODEL, generate_reasoning);

test_set!(GOOGLE_REASONING_MODEL, stream_reasoning);
