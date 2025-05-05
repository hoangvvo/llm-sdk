use anyhow::{anyhow, Result};
use dotenvy::dotenv;
use llm_sdk::{
    google::{GoogleModel, GoogleModelOptions},
    openai::{OpenAIChatModel, OpenAIChatModelOptions, OpenAIModel, OpenAIModelOptions},
    AudioOptions, LanguageModel, LanguageModelMetadata, Modality, ReasoningOptions,
};
use serde::{Deserialize, Serialize};
use std::{env, path::PathBuf, sync::Arc};

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct ModelInfo {
    pub provider: String,
    pub model_id: String,
    pub metadata: LanguageModelMetadata,
    pub audio: Option<AudioOptions>,
    pub reasoning: Option<ReasoningOptions>,
    pub modalities: Option<Vec<Modality>>,
}

pub fn get_model(
    provider: &str,
    model_id: &str,
    metadata: LanguageModelMetadata,
    api_key: Option<String>,
) -> Result<Arc<dyn LanguageModel + Send + Sync>> {
    dotenv().ok();

    match provider {
        "openai" => {
            let api_key = api_key
                .or_else(|| env::var("OPENAI_API_KEY").ok())
                .ok_or_else(|| anyhow!("OPENAI_API_KEY is not set"))?;

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
                .ok_or_else(|| anyhow!("OPENAI_API_KEY is not set"))?;
            let model = OpenAIChatModel::new(
                model_id,
                OpenAIChatModelOptions {
                    api_key,
                    ..Default::default()
                },
            )
            .with_metadata(metadata);
            Ok(Arc::new(model))
        }
        "google" => {
            let api_key = api_key
                .or_else(|| env::var("GOOGLE_API_KEY").ok())
                .ok_or_else(|| anyhow!("GOOGLE_API_KEY is not set"))?;

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
        _ => Err(anyhow!("Unsupported provider: {}", provider)),
    }
}

pub fn get_model_list() -> Result<Vec<ModelInfo>> {
    let mut path = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    path.push("../website/models.json");
    let data = std::fs::read_to_string(path)?;
    let models: Vec<ModelInfo> = serde_json::from_str(&data)?;
    Ok(models)
}
