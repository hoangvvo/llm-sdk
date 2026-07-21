#[cfg(feature = "anthropic")]
use llm_sdk::anthropic::{AnthropicModel, AnthropicModelOptions};
#[cfg(feature = "google")]
use llm_sdk::google::{GoogleModel, GoogleModelOptions};
#[cfg(feature = "openai")]
use llm_sdk::openai::{OpenAIChatModel, OpenAIChatModelOptions, OpenAIModel, OpenAIModelOptions};
use llm_sdk::LanguageModel;
use std::sync::Once;

pub fn get_model(provider: &str, model_id: &str) -> Box<dyn LanguageModel> {
    install_tls_provider();

    match provider {
        #[cfg(feature = "openai")]
        "openai" => Box::new(OpenAIModel::new(
            model_id.to_string(),
            OpenAIModelOptions {
                api_key: std::env::var("OPENAI_API_KEY")
                    .expect("OPENAI_API_KEY environment variable must be set"),
                ..Default::default()
            },
        )),
        #[cfg(feature = "openai")]
        "openai-chat-completion" => Box::new(OpenAIChatModel::new(
            model_id.to_string(),
            OpenAIChatModelOptions {
                api_key: std::env::var("OPENAI_API_KEY")
                    .expect("OPENAI_API_KEY environment variable must be set"),
                ..Default::default()
            },
        )),
        #[cfg(feature = "anthropic")]
        "anthropic" => Box::new(AnthropicModel::new(
            model_id.to_string(),
            AnthropicModelOptions {
                api_key: std::env::var("ANTHROPIC_API_KEY")
                    .expect("ANTHROPIC_API_KEY environment variable must be set"),
                ..Default::default()
            },
        )),
        #[cfg(feature = "google")]
        "google" => Box::new(GoogleModel::new(
            model_id.to_string(),
            GoogleModelOptions {
                api_key: std::env::var("GOOGLE_API_KEY")
                    .expect("GOOGLE_API_KEY environment variable must be set"),
                ..Default::default()
            },
        )),
        _ => panic!("Unsupported provider: {provider}"),
    }
}

fn install_tls_provider() {
    static INIT: Once = Once::new();

    INIT.call_once(|| {
        if rustls::crypto::CryptoProvider::get_default().is_none() {
            rustls::crypto::ring::default_provider()
                .install_default()
                .expect("the application must select its Rustls provider once");
        }
    });
}
