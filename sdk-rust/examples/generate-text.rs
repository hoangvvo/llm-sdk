use dotenvy::dotenv;
use llm_sdk::{LanguageModelInput, Message, Part};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let provider = std::env::var("PROVIDER").unwrap_or_else(|_| "openai".to_string());
    let model_id = std::env::var("MODEL").unwrap_or_else(|_| "gpt-5.6-terra".to_string());
    let model = common::get_model(&provider, &model_id);

    let response = model
        .generate(LanguageModelInput::new([
            Message::user([Part::text("Tell me a story.")]),
            Message::assistant([Part::text(
                "Sure! What kind of story would you like to hear?",
            )]),
            Message::user([Part::text("a fairy tale")]),
        ]))
        .await
        .unwrap();

    println!("{response:#?}");
}
