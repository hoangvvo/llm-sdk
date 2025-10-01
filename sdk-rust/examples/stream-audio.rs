// Requires ffplay (https://ffmpeg.org/) on PATH.
use base64::{engine::general_purpose::STANDARD as BASE64_STANDARD, Engine};
use dotenvy::dotenv;
use futures::StreamExt;
use llm_sdk::{AudioFormat, AudioOptions, LanguageModelInput, Message, Modality, Part, PartDelta};
use serde_json::Value;
use std::{
    io::Write,
    process::{Child, ChildStdin, Command, Stdio},
};

mod common;

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("openai-chat-completion", "gpt-4o-audio-preview");

    let mut stream = model
        .stream(LanguageModelInput {
            modalities: Some(vec![Modality::Text, Modality::Audio]),
            messages: vec![Message::user(vec![Part::text(
                "Is a golden retriever a good family dog?",
            )])],
            audio: Some(AudioOptions {
                format: Some(AudioFormat::Linear16),
                voice: Some("alloy".into()),
                ..Default::default()
            }),
            ..Default::default()
        })
        .await
        .expect("failed to start stream");

    let mut sample_rate: Option<u32> = None;
    let mut channels: Option<u32> = None;
    let mut ffplay: Option<(Child, ChildStdin)> = None;

    while let Some(item) = stream.next().await {
        let chunk = item.expect("stream error");
        log_partial(&chunk);

        if let Some(delta) = chunk.delta {
            if let PartDelta::Audio(audio) = delta.part {
                if let Some(format) = audio.format {
                    if format != AudioFormat::Linear16 {
                        panic!("unsupported audio format: {format:?}");
                    }
                }
                if let Some(b64) = audio.data {
                    let bytes = BASE64_STANDARD
                        .decode(b64.as_bytes())
                        .expect("invalid base64 audio");

                    if sample_rate.is_none() {
                        sample_rate = Some(audio.sample_rate.unwrap_or(24_000));
                    }
                    if channels.is_none() {
                        channels = Some(audio.channels.unwrap_or(1));
                    }

                    if ffplay.is_none() {
                        let rate = sample_rate.unwrap();
                        let ch = channels.unwrap();
                        ffplay = Some(start_ffplay(rate, ch));
                        println!(
                            "Streaming audio with ffplay ({} Hz, {} channel{}).",
                            rate,
                            ch,
                            if ch == 1 { "" } else { "s" },
                        );
                    }

                    if let Some((_, ref mut stdin)) = ffplay {
                        stdin
                            .write_all(&bytes)
                            .expect("failed to write audio to ffplay");
                    }
                }
            }
        }
    }

    if let Some((child, stdin)) = ffplay {
        finish_ffplay(child, stdin);
    }
}

fn start_ffplay(sample_rate: u32, channels: u32) -> (Child, ChildStdin) {
    let mut child = Command::new("ffplay")
        .args([
            "-loglevel",
            "error",
            "-autoexit",
            "-nodisp",
            "-f",
            "s16le",
            "-ar",
            &sample_rate.to_string(),
            "-i",
            "pipe:0",
            "-af",
            &format!(
                "aformat=channel_layouts={}",
                if channels <= 1 { "mono" } else { "stereo" }
            ),
        ])
        .stdin(Stdio::piped())
        .stdout(Stdio::null())
        .stderr(Stdio::inherit())
        .spawn()
        .expect("failed to start ffplay");

    let stdin = child.stdin.take().expect("ffplay stdin unavailable");
    (child, stdin)
}

fn finish_ffplay(mut child: Child, mut stdin: ChildStdin) {
    stdin.flush().expect("failed to flush ffplay stdin");
    drop(stdin);

    let status = child.wait().expect("failed to wait for ffplay");
    if !status.success() {
        panic!("ffplay exited with error");
    }
}

fn log_partial(partial: &llm_sdk::PartialModelResponse) {
    match serde_json::to_value(partial) {
        Ok(mut value) => {
            redact_data(&mut value);
            println!("{value:#?}");
        }
        Err(_) => println!("{partial:#?}"),
    }
}

fn redact_data(value: &mut Value) {
    match value {
        Value::Object(map) => {
            if let Some(Value::String(data)) = map.get_mut("data") {
                if let Ok(bytes) = BASE64_STANDARD.decode(data.as_bytes()) {
                    *data = format!("[{} bytes]", bytes.len());
                } else {
                    *data = "[invalid data]".to_string();
                }
            }
            for entry in map.values_mut() {
                redact_data(entry);
            }
        }
        Value::Array(array) => {
            for item in array.iter_mut() {
                redact_data(item);
            }
        }
        _ => {}
    }
}
