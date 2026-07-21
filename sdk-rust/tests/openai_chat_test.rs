use crate::{common::cases::RunTestCaseOptions, test_group, test_set};
use llm_sdk::openai::*;
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
        "gpt-5.6-terra".to_string(),
        OpenAIChatModelOptions {
            api_key: openai_api_key().clone(),
            ..Default::default()
        },
    )
}

fn openai_audio_model() -> OpenAIChatModel {
    OpenAIChatModel::new(
        "gpt-audio-1.5".to_string(),
        OpenAIChatModelOptions {
            api_key: openai_api_key().clone(),
            ..Default::default()
        },
    )
}

fn no_reasoning_options() -> RunTestCaseOptions {
    RunTestCaseOptions {
        profile: Some("reasoning_disabled"),
    }
}

test_group!(
    openai_model(),
    text_generation,
    Some(no_reasoning_options())
);
test_group!(openai_model(), conversation, Some(no_reasoning_options()));
test_group!(openai_model(), tool_use, Some(no_reasoning_options()));
test_group!(
    openai_model(),
    structured_output,
    Some(no_reasoning_options())
);
test_group!(
    openai_model(),
    generation_options,
    Some(no_reasoning_options())
);
test_group!(openai_model(), source_input, Some(no_reasoning_options()));
test_group!(openai_model(), image_input);

test_set!(
    openai_audio_model(),
    generate_audio,
    Some(RunTestCaseOptions {
        profile: Some("openai_audio_mp3"),
    })
);

test_set!(
    openai_audio_model(),
    stream_audio,
    Some(RunTestCaseOptions {
        profile: Some("openai_audio_linear16"),
    })
);
