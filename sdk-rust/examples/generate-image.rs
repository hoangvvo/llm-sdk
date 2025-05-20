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

    let response = model
        .generate(LanguageModelInput {
            modalities: Some(vec![Modality::Text, Modality::Image]),
            messages: vec![Message::user(vec![Part::text(
                "Generate an image of a sunset over the ocean",
            )])],
            ..Default::default()
        })
        .await
        .expect("model.generate failed");

    println!("{response:#?}");

    if let Some(image_part) = response.content.iter().find_map(|p| match p {
        Part::Image(i) => Some(i),
        _ => None,
    }) {
        let ext = image_part.mime_type.split('/').nth(1).unwrap_or("png");
        let file_name = format!("sunset.{}", ext);

        let image_bytes = BASE64_STANDARD
            .decode(&image_part.image_data)
            .expect("invalid base64 image data");

        fs::write(&file_name, image_bytes).expect("failed to write image file");
        println!("Saved image to {}", file_name);

        // Try to open with the default image viewer
        let open_status = if cfg!(target_os = "macos") {
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

        if let Err(e) = open_status {
            eprintln!("Failed to open image: {e}");
        }

        // Cleanup after a short delay similar to JS example
        sleep(Duration::from_secs(5)).await;
        let _ = fs::remove_file(&file_name);
    } else {
        eprintln!("Image part not found in response");
    }
}
