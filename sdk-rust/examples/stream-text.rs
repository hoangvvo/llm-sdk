use dotenvy::dotenv;
use futures::stream::StreamExt;
use llm_sdk::{LanguageModelInput, Message, Part, StreamAccumulator};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("openai", "gpt-4o");

    let mut stream = model
        .stream(LanguageModelInput {
            messages: vec![
                Message::user(vec![Part::text("Tell me a story.")]),
                Message::assistant(vec![Part::text(
                    "Sure! What kind of story would you like to hear?",
                )]),
                Message::user(vec![Part::text("A fairy tale.")]),
            ],
            ..Default::default()
        })
        .await
        .unwrap();

    let mut accumulator = StreamAccumulator::new();

    while let Some(partial_response) = stream.next().await {
        let partial_response = partial_response.unwrap();
        accumulator.add_partial(partial_response.clone()).unwrap();
        println!("{partial_response:#?}");
    }

    let final_response = accumulator.compute_response();
    println!("Final response: {final_response:#?}");
}
