use dotenvy::dotenv;
use llm_sdk::{LanguageModelInput, Message, Part, UserMessage};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("openai", "gpt-4o");

    let response = model
        .generate(LanguageModelInput {
            messages: vec![
                Message::User(UserMessage {
                    content: vec![Part::Text("Tell me a story.".into())],
                }),
                // Or use the convenient function to create a user message
                Message::assistant(vec![Part::text(
                    "Sure! What kind of story would you like to hear?",
                )]),
                Message::user(vec![Part::Text("a fairy tale".into())]),
            ],
            ..Default::default()
        })
        .await
        .unwrap();

    println!("{response:#?}");
}
