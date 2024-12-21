use llm_sdk::openai::{OpenAIModel, OpenAIModelOptions};
use std::env;

mod language_model_tests;

fn get_model() -> Box<dyn llm_sdk::LanguageModel> {
    Box::new(OpenAIModel::new(OpenAIModelOptions {
        api_key: env::var("OPENAI_API_KEY").unwrap(),
        base_url: None,
        model_id: "gpt-4o".to_string(),
        structured_outputs: false,
    }))
}

#[tokio::test]
async fn test_generate_text() {
    language_model_tests::test_generate_text(get_model()).await;
}

#[tokio::test]
async fn test_generate_with_systemp_prompt() {
    language_model_tests::test_generate_with_system_prompt(get_model()).await;
}

#[tokio::test]
async fn test_generate_tool_call() {
    language_model_tests::test_generate_tool_call(get_model()).await;
}

#[tokio::test]
async fn test_generate_text_from_tool_result() {
    language_model_tests::test_generate_text_from_tool_result(get_model()).await;
}

#[tokio::test]
async fn test_generate_tool_call_for_complex_schema() {
    language_model_tests::test_generate_tool_call_for_complex_schema(get_model()).await;
}

#[tokio::test]
async fn test_generate_response_format_json() {
    language_model_tests::test_generate_response_format_json(Box::new(OpenAIModel::new(
        OpenAIModelOptions {
            api_key: env::var("OPENAI_API_KEY").unwrap(),
            base_url: None,
            model_id: "gpt-4o".to_string(),
            structured_outputs: true,
        },
    )))
    .await;
}
