use llm_agent::BoxedError;
#[cfg(feature = "anthropic")]
use llm_sdk::anthropic::{AnthropicModel, AnthropicModelOptions};
#[cfg(feature = "google")]
use llm_sdk::google::{GoogleModel, GoogleModelOptions};
#[cfg(feature = "openai")]
use llm_sdk::openai::{OpenAIChatModel, OpenAIChatModelOptions, OpenAIModel, OpenAIModelOptions};
use llm_sdk::{LanguageModel, LanguageModelMetadata};
use std::{
    env,
    sync::{Arc, Once},
};

pub fn get_model(
    provider: &str,
    model_id: &str,
    metadata: LanguageModelMetadata,
    api_key: Option<String>,
) -> Result<Arc<dyn LanguageModel + Send + Sync>, BoxedError> {
    install_tls_provider();

    match provider {
        #[cfg(feature = "openai")]
        "openai" => {
            let api_key = api_key
                .or_else(|| env::var("OPENAI_API_KEY").ok())
                .ok_or_else(|| "OPENAI_API_KEY is not set".to_string())?;

            Ok(Arc::new(
                OpenAIModel::new(
                    model_id,
                    OpenAIModelOptions {
                        api_key,
                        ..Default::default()
                    },
                )
                .with_metadata(metadata),
            ))
        }
        #[cfg(feature = "openai")]
        "openai-chat-completion" => {
            let api_key = api_key
                .or_else(|| env::var("OPENAI_API_KEY").ok())
                .ok_or_else(|| "OPENAI_API_KEY is not set".to_string())?;

            Ok(Arc::new(
                OpenAIChatModel::new(
                    model_id,
                    OpenAIChatModelOptions {
                        api_key,
                        ..Default::default()
                    },
                )
                .with_metadata(metadata),
            ))
        }
        #[cfg(feature = "anthropic")]
        "anthropic" => {
            let api_key = api_key
                .or_else(|| env::var("ANTHROPIC_API_KEY").ok())
                .ok_or_else(|| "ANTHROPIC_API_KEY is not set".to_string())?;

            Ok(Arc::new(
                AnthropicModel::new(
                    model_id,
                    AnthropicModelOptions {
                        api_key,
                        ..Default::default()
                    },
                )
                .with_metadata(metadata),
            ))
        }
        #[cfg(feature = "google")]
        "google" => {
            let api_key = api_key
                .or_else(|| env::var("GOOGLE_API_KEY").ok())
                .ok_or_else(|| "GOOGLE_API_KEY is not set".to_string())?;

            Ok(Arc::new(
                GoogleModel::new(
                    model_id,
                    GoogleModelOptions {
                        api_key,
                        ..Default::default()
                    },
                )
                .with_metadata(metadata),
            ))
        }
        _ => Err(format!("Unsupported provider: {provider}").into()),
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
