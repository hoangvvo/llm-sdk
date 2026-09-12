use base64::{engine::general_purpose::STANDARD as BASE64_STANDARD, Engine};
use dotenvy::dotenv;
use llm_sdk::{FilePart, LanguageModelInput, Message, Part};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let provider = std::env::var("PROVIDER").unwrap_or_else(|_| "openai".to_string());
    let model_id = std::env::var("MODEL").unwrap_or_else(|_| "gpt-5.6-sol".to_string());
    let model = common::get_model(&provider, &model_id);

    let file_url = "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf";
    let file_bytes = reqwest::get(file_url)
        .await
        .expect("Failed to fetch file")
        .error_for_status()
        .expect("Failed to fetch file")
        .bytes()
        .await
        .expect("Failed to read file");

    let response = model
        .generate(LanguageModelInput::new([Message::user([
            Part::text("Summarize the attached PDF."),
            FilePart::from_data(BASE64_STANDARD.encode(&file_bytes), "application/pdf")
                .with_filename("dummy.pdf")
                .into(),
        ])]))
        .await
        .expect("Generation failed");

    println!("{response:#?}");
}
