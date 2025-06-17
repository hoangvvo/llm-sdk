use crate::errors::BoxedError;
use base64::{engine::general_purpose, Engine as _};
use llm_sdk::{self, audio_part_utils, Part};
use rmcp::model::{Content, RawContent};

/// Convert MCP content blocks into llm-sdk parts understood by the agent
/// toolkit.
pub(super) fn convert_mcp_content(contents: &[Content]) -> Result<Vec<Part>, BoxedError> {
    let mut parts = Vec::with_capacity(contents.len());

    for content in contents {
        match &content.raw {
            RawContent::Text(text) => {
                parts.push(Part::text(text.text.clone()));
            }
            RawContent::Image(image) => {
                let encoded = general_purpose::STANDARD.encode(&image.data);
                parts.push(Part::image(encoded, image.mime_type.clone()));
            }
            RawContent::Audio(audio) => {
                let format = audio_part_utils::map_mime_type_to_audio_format(&audio.mime_type)
                    .map_err(|err| Box::new(err) as BoxedError)?;
                let encoded = general_purpose::STANDARD.encode(&audio.data);
                parts.push(Part::audio(encoded, format));
            }
            // Resource links and embedded resources are skipped to keep parity with other SDKs.
            RawContent::Resource(_) | RawContent::ResourceLink(_) => {}
        }
    }

    Ok(parts)
}
