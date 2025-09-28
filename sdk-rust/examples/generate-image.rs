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
                 style of a Studio Ghibli landscape, soft colors, gentle light, and a sense of \
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
        let file_name = format!("image.{ext}");

        let image_bytes = BASE64_STANDARD
            .decode(&image_part.image_data)
            .expect("invalid base64 image data");

        fs::write(&file_name, image_bytes).expect("failed to write image file");
        println!("Saved image to {file_name}");

        _ = open_file(&file_name);

        sleep(Duration::from_secs(5)).await;
        let _ = fs::remove_file(&file_name);
        println!("Done.");
    } else {
        eprintln!("Image part not found in response");
    }
}

fn open_file(path: &str) -> std::io::Result<()> {
    #[cfg(target_os = "macos")]
    {
        Command::new("open").arg(path).status()?;
    }

    #[cfg(target_os = "linux")]
    {
        Command::new("xdg-open").arg(path).status()?;
    }

    #[cfg(target_os = "windows")]
    {
        Command::new("cmd")
            .args(["/C", "start", "", path])
            .status()?;
    }

    Ok(())
}
