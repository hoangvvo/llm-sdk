use dotenvy::dotenv;
use futures::stream::StreamExt;
use llm_sdk::{LanguageModelInput, Message, Part, StreamAccumulator, UserMessage};
use serde_json::json;

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let mut stream = common::get_model("anthropic", "claude-opus-4-20250514")
        .stream(LanguageModelInput {
            messages: vec![
                Message::User(UserMessage {
                    content: vec![
                        // Provide sources as part of the user message
                        Part::source(
                            "https://health-site.example/articles/coffee-benefits",
                            "Coffee Health Benefits: What the Research Shows",
                            vec![Part::text(
                                "Coffee contains over 1,000 bioactive compounds, with caffeine being the most studied. A typical 8-ounce cup contains 80-100mg of caffeine. Research shows moderate coffee consumption (3-4 cups daily) is associated with reduced risk of type 2 diabetes, Parkinson's disease, and liver disease. The antioxidants in coffee, particularly chlorogenic acid, may contribute to these protective effects beyond just the caffeine content.",
                            )],
                        ),
                        Part::text(
                            "Based on what you know about coffee's health benefits and caffeine content, what would be the optimal daily coffee consumption for someone who wants the health benefits but is sensitive to caffeine? Consider timing and metabolism.",
                        ),
                    ],
                }),
                Message::assistant(vec![
                    // The model requests a tool call to get more data, which includes sources
                    Part::tool_call(
                        "caffeine_lookup_456",
                        "lookup",
                        json!({
                            "query": "caffeine sensitivity optimal timing metabolism coffee health benefits"
                        }),
                    ),
                ]),
                Message::tool(vec![
                    Part::tool_result(
                        "caffeine_lookup_456",
                        "lookup",
                        vec![
                            // Provide other sources as part of the tool result
                            Part::source(
                                "https://medical-journal.example/2024/caffeine-metabolism-study",
                                "Optimizing Coffee Intake for Caffeine-Sensitive Individuals",
                                vec![Part::text(
                                    "For caffeine-sensitive individuals, the half-life of caffeine extends to 8-12 hours compared to the average 5-6 hours. These individuals experience effects at doses as low as 50mg. Research shows consuming 1-2 cups (100-200mg caffeine) before noon provides 75% of coffee's antioxidant benefits while minimizing side effects like insomnia and anxiety. Splitting intake into smaller doses (half-cups) throughout the morning can further reduce sensitivity reactions while maintaining beneficial compound levels.",
                                )],
                            ),
                        ],
                    ),
                ]),
            ],
            ..Default::default()
        })
        .await
        .unwrap();

    let mut accumulator = StreamAccumulator::new();

    while let Some(partial) = stream.next().await {
        let partial = partial.unwrap();
        println!("{partial:#?}");
        accumulator.add_partial(partial).unwrap();
    }

    let final_response = accumulator.compute_response();
    println!("{final_response:#?}");
}
