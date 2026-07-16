use crate::errors::BoxedError;
use llm_sdk::{self, audio_part_utils, Part};
use rmcp::model::{Content, RawContent};

/// Convert MCP content blocks into llm-sdk parts understood by the agent
/// toolkit.
pub(super) fn convert_mcp_content(contents: Vec<Content>) -> Result<Vec<Part>, BoxedError> {
    let mut parts = Vec::with_capacity(contents.len());

    for content in contents {
        match content.raw {
            RawContent::Text(text) => {
                parts.push(Part::text(text.text));
            }
            RawContent::Image(image) => {
                parts.push(Part::image(image.data, image.mime_type));
            }
            RawContent::Audio(audio) => {
                let format = audio_part_utils::map_mime_type_to_audio_format(&audio.mime_type)
                    .map_err(|err| Box::new(err) as BoxedError)?;
                parts.push(Part::audio(audio.data, format));
            }
            // Resource links and embedded resources are skipped to keep parity with other SDKs.
            RawContent::Resource(_) | RawContent::ResourceLink(_) => {}
        }
    }

    Ok(parts)
}
