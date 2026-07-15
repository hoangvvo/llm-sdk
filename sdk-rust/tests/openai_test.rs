mod common;
use crate::common::cases::RunTestCaseOptions;
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

fn openai_model() -> OpenAIModel {
    OpenAIModel::new(
        "gpt-5.6-sol".to_string(),
        OpenAIModelOptions {
            api_key: openai_api_key().clone(),
            ..Default::default()
        },
    )
}

fn openai_reasoning_model() -> OpenAIModel {
    OpenAIModel::new(
        "o1".to_string(),
        OpenAIModelOptions {
            api_key: openai_api_key().clone(),
            ..Default::default()
        },
    )
}

test_group!(openai_model(), text_generation);
test_group!(openai_model(), conversation);
test_group!(openai_model(), tool_use);
test_group!(openai_model(), structured_output);
test_group!(openai_model(), generation_options);
test_group!(openai_model(), source_input);
test_group!(openai_model(), multimodal_tool_result);
test_group!(openai_model(), web_search);
test_group!(openai_model(), image_generation);
test_group!(openai_model(), image_input);
test_group!(
    openai_reasoning_model(),
    reasoning,
    Some(RunTestCaseOptions {
        profile: Some("openai_opaque_reasoning"),
    })
);
test_group!(openai_model(), reasoning_tool_use);
