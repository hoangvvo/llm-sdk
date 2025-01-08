use dotenvy::dotenv;
use futures::stream::StreamExt;
use llm_sdk::{
    AssistantMessage, LanguageModelInput, Message, Part, StreamAccumulator, UserMessage,
};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("openai", "gpt-4o");

    let mut stream = model
        .stream(LanguageModelInput {
            messages: vec![
                Message::User(UserMessage {
                    content: vec![Part::Text("Tell me a story.".into())],
                }),
                Message::Assistant(AssistantMessage {
                    content: vec![Part::Text(
                        "What kind of story would you like to hear?".into(),
                    )],
                }),
                Message::User(UserMessage {
                    content: vec![Part::Text("A fairy tale.".into())],
                }),
            ],
            ..Default::default()
        })
        .await
        .unwrap();

    let mut accumulator = StreamAccumulator::new();

    while let Some(partial_response) = stream.next().await {
        let partial_response = partial_response.unwrap();
        accumulator.add_partial(&partial_response).unwrap();
        println!("{partial_response:#?}");
    }

    let final_response = accumulator.compute_response();
    println!("Final response: {final_response:#?}");
}
