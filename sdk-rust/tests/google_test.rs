mod common;
use crate::common::{assert::PartAssertion, cases::RunTestCaseOptions};
use llm_sdk::{google::*, *};
use std::{env, error::Error, sync::OnceLock};
use tokio::test;

fn google_api_key() -> &'static String {
    static KEY: OnceLock<String> = OnceLock::new();

    KEY.get_or_init(|| {
        dotenvy::dotenv().ok();
        env::var("GOOGLE_API_KEY").expect("GOOGLE_API_KEY must be set")
    })
}

fn google_model() -> GoogleModel {
    GoogleModel::new(
        "gemini-2.5-flash".to_string(),
        GoogleModelOptions {
            api_key: google_api_key().clone(),
            ..Default::default()
        },
    )
}

fn google_audio_model() -> GoogleModel {
    GoogleModel::new(
        "gemini-2.5-flash-preview-tts".to_string(),
        GoogleModelOptions {
            api_key: google_api_key().clone(),
            ..Default::default()
        },
    )
}

fn google_reasoning_model() -> GoogleModel {
    GoogleModel::new(
        "gemini-2.0-flash-thinking-exp-01-21".to_string(),
        GoogleModelOptions {
            api_key: google_api_key().clone(),
            ..Default::default()
        },
    )
}

test_set!(google_model(), generate_text);

test_set!(google_model(), stream_text);

test_set!(google_model(), generate_with_system_prompt);

test_set!(google_model(), generate_tool_call);

test_set!(google_model(), stream_tool_call);

test_set!(google_model(), generate_text_from_tool_result);

test_set!(google_model(), stream_text_from_tool_result);

test_set!(google_model(), generate_parallel_tool_calls);

test_set!(google_model(), stream_parallel_tool_calls);

test_set!(google_model(), stream_parallel_tool_calls_same_name);

test_set!(google_model(), structured_response_format);

test_set!(google_model(), source_part_input);

test_set!(
    google_audio_model(),
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
                        part.id = false;
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
    google_audio_model(),
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
                        part.id = false;
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

test_set!(google_reasoning_model(), generate_reasoning);

test_set!(google_reasoning_model(), stream_reasoning);
