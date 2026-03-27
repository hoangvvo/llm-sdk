use dotenvy::dotenv;
use llm_sdk::{LanguageModelInput, Message, Part, ReasoningOptions};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("openai", "gpt-5.4");

    let response = model
        .generate(LanguageModelInput {
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

    let (reasoning_parts, other_parts): (Vec<Part>, Vec<Part>) = response
        .content
        .into_iter()
        .partition(|part| matches!(part, Part::Reasoning { .. }));
    println!("Reasoning:");
    for part in reasoning_parts {
        println!("{part:#?}");
    }
    println!("\nAnswer:");
    for part in other_parts {
        println!("{part:#?}");
    }
}
