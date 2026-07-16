mod common;
use crate::common::cases::RunTestCaseOptions;
use llm_sdk::google::*;
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
        "gemini-3.1-flash-lite".to_string(),
        GoogleModelOptions {
            api_key: google_api_key().clone(),
            ..Default::default()
        },
    )
}

fn google_audio_model() -> GoogleModel {
    GoogleModel::new(
        "gemini-3.1-flash-tts-preview".to_string(),
        GoogleModelOptions {
            api_key: google_api_key().clone(),
            ..Default::default()
        },
    )
}

fn google_image_model() -> GoogleModel {
    GoogleModel::new(
        "gemini-3.1-flash-image".to_string(),
        GoogleModelOptions {
            api_key: google_api_key().clone(),
            ..Default::default()
        },
    )
}

fn google_multimodal_tool_model() -> GoogleModel {
    GoogleModel::new(
        "gemini-3.1-pro-preview".to_string(),
        GoogleModelOptions {
            api_key: google_api_key().clone(),
            ..Default::default()
        },
    )
}

fn google_reasoning_model() -> GoogleModel {
    GoogleModel::new(
        "gemini-3.1-pro-preview".to_string(),
        GoogleModelOptions {
            api_key: google_api_key().clone(),
            ..Default::default()
        },
    )
}

test_group!(google_model(), text_generation);
test_group!(google_model(), conversation);
test_group!(google_model(), tool_use);
test_group!(google_model(), structured_output);
test_group!(google_model(), generation_options);
test_group!(google_model(), source_input);
test_group!(google_multimodal_tool_model(), multimodal_tool_result);
test_group!(
    google_model(),
    web_search,
    Some(RunTestCaseOptions {
        profile: Some("google_web_search"),
    })
);
test_group!(google_image_model(), image_generation);
test_group!(google_image_model(), image_input);
test_group!(
    google_audio_model(),
    audio_generation,
    Some(RunTestCaseOptions {
        profile: Some("google_audio"),
    })
);
test_group!(google_reasoning_model(), reasoning);
test_group!(google_reasoning_model(), reasoning_tool_use);
