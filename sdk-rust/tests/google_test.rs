use crate::{common, common::cases::RunTestCaseOptions, test_group};
use llm_sdk::{google::*, LanguageModelMetadata, LanguageModelPricing};
use std::{env, error::Error, sync::OnceLock};
use tokio::test;

fn google_api_key() -> &'static String {
    static KEY: OnceLock<String> = OnceLock::new();

    common::install_tls_provider();
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
test_group!(
    google_model(),
    web_search_tool_mix,
    Some(RunTestCaseOptions {
        profile: Some("google_web_search_tool_mix"),
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

#[test]
async fn transport() -> Result<(), Box<dyn Error>> {
    common::install_tls_provider();

    common::transports::run_transport_test_group("google_transport", |base_url| {
        GoogleModel::new(
            "test-model",
            GoogleModelOptions {
                api_key: "test-token".to_string(),
                base_url: Some(format!("{base_url}/v1beta")),
                ..Default::default()
            },
        )
        .with_metadata(LanguageModelMetadata {
            pricing: Some(LanguageModelPricing {
                input_cost_per_text_token: Some(2.0),
                input_cost_per_cached_token: Some(1.0),
                input_cost_per_cache_write_token: None,
                input_cost_per_cached_text_token: None,
                output_cost_per_text_token: Some(4.0),
                input_cost_per_audio_token: Some(3.0),
                input_cost_per_cached_audio_token: None,
                output_cost_per_audio_token: Some(5.0),
                input_cost_per_image_token: None,
                input_cost_per_cached_image_token: None,
                output_cost_per_image_token: None,
            }),
            capabilities: None,
        })
    })
    .await
}
