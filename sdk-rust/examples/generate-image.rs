use base64::{engine::general_purpose::STANDARD as BASE64_STANDARD, Engine};
use dotenvy::dotenv;
use llm_sdk::{LanguageModelInput, Message, Modality, Part};
use std::{fs, process::Command, time::Duration};
use tokio::time::sleep;

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("google", "gemini-2.0-flash-exp-image-generation");

    println!("Requesting image generation...");
    let response = model
        .generate(LanguageModelInput {
            modalities: Some(vec![Modality::Text, Modality::Image]),
            messages: vec![Message::user(vec![Part::text(
                "A bright, sunlit green hill with a single large, leafy tree, fluffy clouds \
                 drifting across a deep blue sky, painted in the warm, detailed, hand-painted \
                 style of a Studio Ghibli landscape—soft colors, gentle light, and a sense of \
                 quiet wonder.",
            )])],
            ..Default::default()
        })
        .await
        .expect("model.generate failed");
    // Generation response is intentionally not printed to keep output concise

    if let Some(image_part) = response.content.iter().find_map(|p| match p {
        Part::Image(i) => Some(i),
        _ => None,
    }) {
        let ext = image_part.mime_type.split('/').nth(1).unwrap_or("png");
        let file_name = format!("image.{}", ext);

        let image_bytes = BASE64_STANDARD
            .decode(&image_part.image_data)
            .expect("invalid base64 image data");

        fs::write(&file_name, image_bytes).expect("failed to write image file");
        println!("Saved image to {}", file_name);

        println!("Rendering image to terminal...");
        // viuer prints the image directly in supported terminals
        let config = viuer::Config::default();
        if let Err(e) = viuer::print_from_file(&file_name, &config) {
            eprintln!("Failed to render image: {e}");
        }

        println!("---");
        // Try to open with the default image viewer
        let _ = if cfg!(target_os = "macos") {
            Command::new("open").arg(&file_name).status()
        } else if cfg!(target_os = "linux") {
            Command::new("xdg-open").arg(&file_name).status()
        } else if cfg!(target_os = "windows") {
            Command::new("cmd")
                .args(["/C", "start", "", &file_name])
                .status()
        } else {
            Err(std::io::Error::new(
                std::io::ErrorKind::Other,
                "unsupported OS for auto-open",
            ))
        };

        sleep(Duration::from_secs(5)).await;
        let _ = fs::remove_file(&file_name);
        println!("Done.");
    } else {
        eprintln!("Image part not found in response");
    }
}
