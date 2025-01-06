use base64::{engine::general_purpose::STANDARD as BASE64_STANDARD, Engine};
use dotenvy::dotenv;
use llm_sdk::{LanguageModelInput, Message, Modality, Part, UserMessage};
use std::io::Cursor;

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("openai", "gpt-4o-audio-preview");

    let response = model
        .generate(LanguageModelInput {
            extra: Some(serde_json::json!({
                "audio": {
                    "voice": "alloy",
                    "format": "mp3"
                }
            })),
            modalities: Some(vec![Modality::Text, Modality::Audio]),
            messages: vec![Message::User(UserMessage {
                content: vec![Part::Text(
                    "Is a golden retriever a good family dog?".into(),
                )],
            })],
            ..Default::default()
        })
        .await
        .expect("model.generate failed");

    println!("{response:#?}");

    if let Some(audio_part) = response.content.iter().find_map(|p| match p {
        Part::Audio(a) => Some(a),
        _ => None,
    }) {
        let audio_bytes = BASE64_STANDARD
            .decode(&audio_part.audio_data)
            .expect("invalid base64 audio data");

        let cursor = Cursor::new(audio_bytes);
        let (_stream, stream_handle) = rodio::OutputStream::try_default().unwrap();
        let sink = rodio::Sink::try_new(&stream_handle).unwrap();

        let source = rodio::Decoder::new(cursor).unwrap();
        sink.append(source);

        sink.sleep_until_end();
    }
}
