use base64::{engine::general_purpose::STANDARD as BASE64_STANDARD, Engine};
use dotenvy::dotenv;
use llm_sdk::{LanguageModelInput, Message, Part};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let image_url = "https://images.unsplash.com/photo-1464809142576-df63ca4ed7f0";
    let image_res = reqwest::get(image_url)
        .await
        .expect("failed to fetch image");
    let mime_type = image_res
        .headers()
        .get(reqwest::header::CONTENT_TYPE)
        .and_then(|ct| ct.to_str().ok())
        .unwrap_or("image/jpeg")
        .to_string();
    let image_bytes = image_res.bytes().await.expect("failed to read bytes");

    let image_b64 = BASE64_STANDARD.encode(&image_bytes);

    let model = common::get_model("openai", "gpt-4o");

    let response = model
        .generate(LanguageModelInput {
            messages: vec![Message::user(vec![
                Part::text("Describe this image"),
                Part::image(image_b64, mime_type),
            ])],
            ..Default::default()
        })
        .await
        .expect("model.generate failed");

    println!("{response:#?}");
}
