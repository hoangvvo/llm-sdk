use dotenvy::dotenv;
use llm_agent::BoxedError;
use llm_sdk::{
    google::{GoogleModel, GoogleModelOptions},
    openai::{OpenAIChatModel, OpenAIChatModelOptions, OpenAIModel, OpenAIModelOptions},
    AudioOptions, LanguageModel, LanguageModelMetadata, Modality, ReasoningOptions,
};
use serde::{Deserialize, Serialize};
use std::{
    env,
    io::{Error as IoError, ErrorKind},
    path::PathBuf,
    sync::Arc,
};

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
) -> Result<Arc<dyn LanguageModel + Send + Sync>, BoxedError> {
    dotenv().ok();

    match provider {
        "openai" => {
            let api_key = api_key
                .or_else(|| env::var("OPENAI_API_KEY").ok())
                .ok_or_else(|| missing_env("OPENAI_API_KEY"))?;

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
                .ok_or_else(|| missing_env("OPENAI_API_KEY"))?;
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
                .ok_or_else(|| missing_env("GOOGLE_API_KEY"))?;

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
        _ => Err(Box::new(IoError::new(
            ErrorKind::InvalidInput,
            format!("Unsupported provider: {provider}"),
        ))),
    }
}

pub fn get_model_list() -> Result<Vec<ModelInfo>, BoxedError> {
    let mut path = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    path.push("../website/models.json");
    let data = std::fs::read_to_string(path).map_err(|err| Box::new(err) as BoxedError)?;
    let models: Vec<ModelInfo> =
        serde_json::from_str(&data).map_err(|err| Box::new(err) as BoxedError)?;
    Ok(models)
}

fn missing_env(var: &str) -> BoxedError {
    format!("{var} is not set").into()
}
