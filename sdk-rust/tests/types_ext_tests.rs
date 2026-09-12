use llm_sdk::{AudioFormat, AudioPart, FilePart, ImagePart, Part};
use serde_json::json;

#[test]
fn media_part_constructors_preserve_sources_and_metadata() {
    let cases: [(Part, serde_json::Value); 6] = [
        (
            ImagePart::from_data("AAEC", "image/png")
                .with_width(640)
                .with_height(480)
                .with_id("image_1")
                .into(),
            json!({"type": "image", "data": "AAEC", "mime_type": "image/png",
                "width": 640, "height": 480, "id": "image_1"}),
        ),
        (
            ImagePart::from_url("https://example.com/image.png", "image/png")
                .with_width(640)
                .with_height(480)
                .with_id("image_1")
                .into(),
            json!({"type": "image", "url": "https://example.com/image.png", "mime_type": "image/png",
                "width": 640, "height": 480, "id": "image_1"}),
        ),
        (
            AudioPart::from_data("AAEC", AudioFormat::Mp3)
                .with_sample_rate(24000)
                .with_channels(1)
                .with_transcript("Hello")
                .with_id("audio_1")
                .into(),
            json!({"type": "audio", "data": "AAEC", "format": "mp3",
                "sample_rate": 24000, "channels": 1, "transcript": "Hello", "id": "audio_1"}),
        ),
        (
            AudioPart::from_url("https://example.com/audio.mp3", AudioFormat::Mp3)
                .with_sample_rate(24000)
                .with_channels(1)
                .with_transcript("Hello")
                .with_id("audio_1")
                .into(),
            json!({"type": "audio", "url": "https://example.com/audio.mp3", "format": "mp3",
                "sample_rate": 24000, "channels": 1, "transcript": "Hello", "id": "audio_1"}),
        ),
        (
            FilePart::from_data("AAEC", "application/pdf")
                .with_filename("document.pdf")
                .into(),
            json!({"type": "file", "data": "AAEC", "mime_type": "application/pdf",
                "filename": "document.pdf"}),
        ),
        (
            FilePart::from_url("https://example.com/document.pdf", "application/pdf")
                .with_filename("document.pdf")
                .into(),
            json!({"type": "file", "url": "https://example.com/document.pdf", "mime_type": "application/pdf",
                "filename": "document.pdf"}),
        ),
    ];

    for (part, expected) in cases {
        let value = serde_json::to_value(&part).unwrap();
        assert_eq!(value, expected);
        assert_eq!(serde_json::from_value::<Part>(value).unwrap(), part);
    }
}

#[test]
fn media_part_shortcuts_serialize_only_the_selected_source() {
    let cases = [
        (
            Part::image("AAEC", "image/png"),
            json!({"type": "image", "data": "AAEC", "mime_type": "image/png"}),
        ),
        (
            Part::image_from_url("https://example.com/image.png", "image/png"),
            json!({"type": "image", "url": "https://example.com/image.png", "mime_type": "image/png"}),
        ),
        (
            Part::audio("AAEC", AudioFormat::Mp3),
            json!({"type": "audio", "data": "AAEC", "format": "mp3"}),
        ),
        (
            Part::audio_from_url("https://example.com/audio.mp3", AudioFormat::Mp3),
            json!({"type": "audio", "url": "https://example.com/audio.mp3", "format": "mp3"}),
        ),
        (
            Part::file("AAEC", "application/pdf"),
            json!({"type": "file", "data": "AAEC", "mime_type": "application/pdf"}),
        ),
        (
            Part::file_from_url("https://example.com/document.pdf", "application/pdf"),
            json!({"type": "file", "url": "https://example.com/document.pdf", "mime_type": "application/pdf"}),
        ),
    ];

    for (part, expected) in cases {
        assert_eq!(serde_json::to_value(part).unwrap(), expected);
    }
}
