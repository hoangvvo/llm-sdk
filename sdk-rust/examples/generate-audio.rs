// Requires ffplay (https://ffmpeg.org/) on PATH.
use base64::{engine::general_purpose::STANDARD as BASE64_STANDARD, Engine};
use dotenvy::dotenv;
use llm_sdk::{AudioFormat, AudioOptions, LanguageModelInput, Message, Modality, Part};
use std::{
    io::Write,
    process::{Command, Stdio},
};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("openai-chat-completion", "gpt-4o-audio-preview");

    let response = model
        .generate(LanguageModelInput {
            modalities: Some(vec![Modality::Text, Modality::Audio]),
            messages: vec![Message::user(vec![Part::text(
                "Is a golden retriever a good family dog?",
            )])],
            audio: Some(AudioOptions {
                format: Some(AudioFormat::Mp3),
                voice: Some("alloy".into()),
                ..Default::default()
            }),
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
            .decode(&audio_part.data)
            .expect("invalid base64 audio data");

        play(&audio_bytes).expect("ffplay playback failed");
    } else {
        println!("Audio part not found in response");
    }
}

fn play(audio: &[u8]) -> std::io::Result<()> {
    let mut child = Command::new("ffplay")
        .args(["-autoexit", "-nodisp", "-loglevel", "error", "-"])
        .stdin(Stdio::piped())
        .stdout(Stdio::null())
        .stderr(Stdio::inherit())
        .spawn()?;

    {
        let stdin = child.stdin.as_mut().expect("ffplay stdin unavailable");
        stdin.write_all(audio)?;
    }

    let status = child.wait()?;
    if status.success() {
        Ok(())
    } else {
        Err(std::io::Error::other("ffplay exited with error"))
    }
}
