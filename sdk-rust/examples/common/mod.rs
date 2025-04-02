use llm_sdk::{
    google::{GoogleModel, GoogleModelOptions},
    openai::{OpenAIChatModel, OpenAIChatModelOptions, OpenAIModel, OpenAIModelOptions},
    LanguageModel,
};

pub fn get_model(provider: &str, model_id: &str) -> Box<dyn LanguageModel> {
    match provider {
        "openai" => Box::new(OpenAIModel::new(
            model_id.to_string(),
            OpenAIModelOptions {
                api_key: std::env::var("OPENAI_API_KEY")
                    .expect("OPENAI_API_KEY environment variable must be set"),
                ..Default::default()
            },
        )),
        "openai-chat-completion" => Box::new(OpenAIChatModel::new(
            model_id.to_string(),
            OpenAIChatModelOptions {
                api_key: std::env::var("OPENAI_API_KEY")
                    .expect("OPENAI_API_KEY environment variable must be set"),
                ..Default::default()
            },
        )),
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
