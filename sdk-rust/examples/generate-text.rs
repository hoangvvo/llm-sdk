use dotenvy::dotenv;
use llm_sdk::{LanguageModelInput, Message, Part};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("openai", "gpt-4o");

    let response = model
        .generate(LanguageModelInput {
            messages: vec![
                Message::user(vec![Part::text("Tell me a story.")]),
                Message::assistant(vec![Part::text(
                    "Sure! What kind of story would you like to hear?",
                )]),
                Message::user(vec![Part::text("a fairy tale")]),
            ],
            ..Default::default()
        })
        .await
        .unwrap();

    println!("{response:#?}");
}
