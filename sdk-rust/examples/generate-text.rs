use dotenvy::dotenv;
use llm_sdk::{LanguageModelInput, Message, Part};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("openai", "gpt-4o");

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
