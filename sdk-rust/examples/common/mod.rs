use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    LanguageModel,
};

pub fn get_model(provider: &str, model_id: &str) -> Box<dyn LanguageModel> {
    match provider {
        "openai" => Box::new(OpenAIModel::new(OpenAIModelOptions {
            model_id: model_id.to_string(),
            api_key: std::env::var("OPENAI_API_KEY")
                .expect("OPENAI_API_KEY environment variable must be set"),
            ..Default::default()
        })),
        _ => panic!("Unsupported provider: {provider}"),
    }
}
