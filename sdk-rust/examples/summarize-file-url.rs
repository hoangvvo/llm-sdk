use dotenvy::dotenv;
use llm_sdk::{LanguageModelInput, Message, Part};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let provider = std::env::var("PROVIDER").unwrap_or_else(|_| "openai".to_string());
    let model_id = std::env::var("MODEL").unwrap_or_else(|_| "gpt-5.6-sol".to_string());
    let model = common::get_model(&provider, &model_id);

    let response = model
        .generate(LanguageModelInput::new([Message::user([
            Part::text("Summarize the attached PDF."),
            // The provider fetches this URL; no download or base64 encoding is needed here.
            Part::file_from_url(
                "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf",
                "application/pdf",
            ),
        ])]))
        .await
        .expect("Generation failed");

    println!("{response:#?}");
}
