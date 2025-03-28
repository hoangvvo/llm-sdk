use base64::{engine::general_purpose::STANDARD as BASE64_STANDARD, Engine};
use dotenvy::dotenv;
use llm_sdk::{AudioFormat, LanguageModelInput};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let audio_url = "https://archive.org/download/MLKDream/MLKDream.ogg";

    let audio_res = reqwest::get(audio_url)
        .await
        .expect("failed to fetch audio");
    let audio_bytes = audio_res.bytes().await.expect("failed to read bytes");

    let audio_b64 = BASE64_STANDARD.encode(&audio_bytes);

    let model = common::get_model("google", "gemini-2.0-flash");

    let response = model
        .generate(LanguageModelInput {
            messages: vec![llm_sdk::Message::user(vec![
                llm_sdk::Part::text("What is this speech about?"),
                llm_sdk::Part::audio(audio_b64, AudioFormat::Opus),
            ])],
            ..Default::default()
        })
        .await
        .expect("model.generate failed");

    println!("{response:#?}");
}
