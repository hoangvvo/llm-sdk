use crate::{LanguageModelError, LanguageModelResult};
use base64::Engine as _;

pub fn base64_to_i16sample(b64: &str) -> Result<Vec<i16>, String> {
    let bytes = base64::engine::general_purpose::STANDARD
        .decode(b64)
        .map_err(|e| format!("Failed to decode base64: {e}"))?;

    // Convert bytes to i16 samples (little-endian)
    if bytes.len() % 2 != 0 {
        return Err("Base64 data length is not a multiple of 2".to_string());
    }

    let mut samples = Vec::with_capacity(bytes.len() / 2);
    for chunk in bytes.chunks(2) {
        let sample = i16::from_le_bytes([chunk[0], chunk[1]]);
        samples.push(sample);
    }

    Ok(samples)
}

pub fn i16sample_to_base64(samples: &[i16]) -> String {
    // Convert i16 samples to bytes (little-endian)
    let mut result_bytes = Vec::with_capacity(samples.len() * 2);
    for &sample in samples {
        result_bytes.extend_from_slice(&sample.to_le_bytes());
    }

    // Encode to base64
    base64::engine::general_purpose::STANDARD.encode(result_bytes)
}

/// Concatenates audio data chunks into a single base64 string
pub fn concatenate_b64_audio_chunks(chunks: &[String]) -> LanguageModelResult<String> {
    if chunks.is_empty() {
        return Ok(String::new());
    }

    // Decode all chunks and collect samples
    let mut all_samples: Vec<i16> = Vec::new();

    for chunk in chunks {
        let samples = base64_to_i16sample(chunk).map_err(|e| {
            LanguageModelError::Invariant("", format!("Failed to decode audio chunk: {e}"))
        })?;
        all_samples.extend(samples);
    }

    let b64 = i16sample_to_base64(&all_samples);

    Ok(b64)
}
