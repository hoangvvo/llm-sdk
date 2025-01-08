use base64::{engine::general_purpose::STANDARD as BASE64_STANDARD, Engine};
use dotenvy::dotenv;
use futures::StreamExt;
use llm_sdk::{DeltaPart, LanguageModelInput, Message, Modality, Part, UserMessage};
use rodio::{buffer::SamplesBuffer, OutputStream, Sink};
use serde_json::json;

mod common;

fn bytes_to_i16_le_samples(bytes: &[u8]) -> Vec<i16> {
    let mut out = Vec::with_capacity(bytes.len() / 2);
    for chunk in bytes.chunks_exact(2) {
        out.push(i16::from_le_bytes([chunk[0], chunk[1]]));
    }
    out
}

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("openai", "gpt-4o-audio-preview");

    let mut stream = model
        .stream(LanguageModelInput {
            extra: Some(json!({
                "audio": { "voice": "alloy", "format": "pcm16" }
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
        .expect("failed to start stream");

    // Lazy init: keep OutputStream alive and reuse its Sink
    let mut out_stream: Option<(OutputStream, rodio::OutputStreamHandle, Sink)> = None;

    while let Some(item) = stream.next().await {
        let chunk = item.expect("stream error");
        println!("{chunk:#?}");

        if let Some(delta) = chunk.delta {
            let part = delta.part; // not an Option
            if let DeltaPart::Audio(a) = part {
                // audio_data is a String, not Option<String>
                if let Some(b64_data) = a.audio_data {
                    let sample_rate: u32 = a.sample_rate.unwrap_or(24_000);
                    let channels: u32 = a.channels.unwrap_or(1);

                    // Ensure audio output is initialized
                    if out_stream.is_none() {
                        let (s, h) =
                            OutputStream::try_default().expect("no default audio output device");
                        let k = Sink::try_new(&h).expect("failed creating sink");
                        out_stream = Some((s, h, k));
                    }

                    // Get a mutable handle to the sink
                    let sink = &mut out_stream.as_mut().unwrap().2;

                    // Decode base64 -> bytes -> i16 samples
                    let bytes = BASE64_STANDARD
                        .decode(b64_data.as_bytes())
                        .expect("invalid base64 audio");
                    let samples = bytes_to_i16_le_samples(&bytes);

                    // Append chunk to the sink (seamless streaming)
                    let source =
                        SamplesBuffer::new(u16::try_from(channels).unwrap(), sample_rate, samples);
                    sink.append(source);
                }
            }
        }
    }

    // Let the queued audio finish
    if let Some((_, _, sink)) = out_stream {
        sink.sleep_until_end();
        println!("Playback finished");
    }
}
