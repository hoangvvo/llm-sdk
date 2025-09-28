use llm_agent::BoxedError;
use llm_sdk::{
    anthropic::{AnthropicModel, AnthropicModelOptions},
    google::{GoogleModel, GoogleModelOptions},
    openai::{OpenAIChatModel, OpenAIChatModelOptions, OpenAIModel, OpenAIModelOptions},
    LanguageModel, LanguageModelMetadata,
};
use std::{env, sync::Arc};

pub fn get_model(
    provider: &str,
    model_id: &str,
    metadata: LanguageModelMetadata,
    api_key: Option<String>,
) -> Result<Arc<dyn LanguageModel + Send + Sync>, BoxedError> {
    match provider {
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
