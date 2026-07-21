use dotenvy::dotenv;
use llm_sdk::{LanguageModelInput, Message, Part, WebSearchTool};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let provider = std::env::var("PROVIDER").unwrap_or_else(|_| "openai".to_string());
    let model_id = std::env::var("MODEL").unwrap_or_else(|_| "gpt-5.6-sol".to_string());
    let model = common::get_model(&provider, &model_id);

    let response = model
        .generate(
            LanguageModelInput::new([Message::user([Part::text(
                "Use web search to find the official IANA page about reserved domains. Reply with \
                 one sentence containing the word IANA and cite the source.",
            )])])
            .with_tools([WebSearchTool::new()]),
        )
        .await
        .unwrap();

    for part in response.content {
        match part {
            Part::ToolCall(call) => println!("web search call: {:#?}", call.call),
            Part::ToolResult(result) => println!("web search result: {:#?}", result.result),
            Part::Text(text) => println!("{} {:#?}", text.text, text.citations),
            _ => {}
        }
    }
}
