mod common;
use llm_sdk::anthropic::*;
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
        "claude-sonnet-5".to_string(),
        AnthropicModelOptions {
            api_key: anthropic_api_key().clone(),
            ..Default::default()
        },
    )
}

test_group!(anthropic_model(), text_generation);
test_group!(anthropic_model(), conversation);
test_group!(anthropic_model(), tool_use);
test_group!(anthropic_model(), structured_output);
test_group!(anthropic_model(), generation_options);
test_group!(anthropic_model(), source_input);
test_group!(anthropic_model(), multimodal_tool_result);
test_group!(
    anthropic_model(),
    web_search,
    Some(crate::common::cases::RunTestCaseOptions {
        profile: Some("anthropic_web_search"),
    })
);
test_group!(anthropic_model(), image_input);
test_group!(
    anthropic_model(),
    reasoning,
    Some(crate::common::cases::RunTestCaseOptions {
        profile: Some("anthropic_adaptive_reasoning"),
    })
);
test_group!(anthropic_model(), reasoning_tool_use);
test_group!(anthropic_model(), anthropic_refusal);
