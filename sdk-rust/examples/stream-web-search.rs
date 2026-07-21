use dotenvy::dotenv;
use futures::stream::StreamExt;
use llm_sdk::{LanguageModelInput, Message, Part, StreamAccumulator, WebSearchTool};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let provider = std::env::var("PROVIDER").unwrap_or_else(|_| "openai".to_string());
    let model_id = std::env::var("MODEL").unwrap_or_else(|_| "gpt-5.6-sol".to_string());
    let model = common::get_model(&provider, &model_id);

    let mut stream = model
        .stream(
            LanguageModelInput::new([Message::user([Part::text(
                "Use web search to find the official IANA page about reserved domains. Reply with \
                 one sentence containing the word IANA and cite the source.",
            )])])
            .with_tools([WebSearchTool::new()]),
        )
        .await
        .unwrap();

    let mut accumulator = StreamAccumulator::new();

    while let Some(partial) = stream.next().await {
        let partial = partial.unwrap();
        println!("{partial:#?}");
        accumulator.add_partial(partial).unwrap();
    }

    let response = accumulator.compute_response().unwrap();
    for part in response.content {
        match part {
            Part::ToolCall(call) => println!("web search call: {:#?}", call.call),
            Part::ToolResult(result) => println!("web search result: {:#?}", result.result),
            Part::Text(text) => println!("{} {:#?}", text.text, text.citations),
            _ => {}
        }
    }
}
