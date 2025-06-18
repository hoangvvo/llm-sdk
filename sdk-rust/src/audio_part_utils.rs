use crate::{AudioFormat, LanguageModelError, LanguageModelResult};

pub fn map_audio_format_to_mime_type(format: &AudioFormat) -> String {
    match format {
        AudioFormat::Wav => "audio/wav",
        AudioFormat::Mp3 => "audio/mp3",
        AudioFormat::Linear16 => "audio/l16",
        AudioFormat::Flac => "audio/flac",
        AudioFormat::Mulaw | AudioFormat::Alaw => "audio/basic",
        AudioFormat::Aac => "audio/aac",
        AudioFormat::Opus => "audio/opus",
    }
    .to_string()
}

pub fn map_mime_type_to_audio_format(mime_type: &str) -> LanguageModelResult<AudioFormat> {
    let formatted_mime_type = mime_type
        .split(';')
        .next()
        .unwrap_or(mime_type)
        .trim()
        .to_lowercase();
    Ok(match formatted_mime_type.as_str() {
        "audio/wav" => AudioFormat::Wav,
        "audio/mp3" | "audio/mpeg" => AudioFormat::Mp3,
        "audio/l16" => AudioFormat::Linear16,
        "audio/flac" => AudioFormat::Flac,
        "audio/basic" => AudioFormat::Mulaw, // Default to Mulaw for "audio/basic"
        "audio/aac" => AudioFormat::Aac,
        "audio/opus" => AudioFormat::Opus,
        _ => Err(LanguageModelError::Invariant(
            "",
            format!("Unsupported audio mime type: {mime_type}"),
        ))?,
    })
}
