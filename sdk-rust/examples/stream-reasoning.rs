use dotenvy::dotenv;
use futures::StreamExt;
use llm_sdk::{LanguageModelInput, Message, Part, PartDelta, ReasoningOptions};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("openai", "o1");

    let mut stream = model
        .stream(LanguageModelInput {
            messages: vec![
                Message::user(
                  vec![Part::text(r"A car starts from rest and accelerates at a constant rate of 4 m/s^2 for 10 seconds.
1. What is the final velocity of the car after 10 seconds?
2. How far does the car travel in those 10 seconds?")]
                )
            ],
                reasoning: Some(ReasoningOptions {
                enabled: true,
                ..Default::default()
            }),
            ..Default::default()
        })
        .await
        .unwrap();

    while let Some(partial) = stream.next().await {
        let partial = partial.expect("stream error");
        if let Some(delta) = partial.delta {
            if let PartDelta::Reasoning { .. } = delta.part {
                println!("Reasoning:");
                println!("{delta:#?}");
            } else {
                println!("Answer:");
                println!("{delta:#?}");
            }
        }
    }
}
