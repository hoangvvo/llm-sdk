use crate::{ContentDelta, DeltaPart};

/// Because of the difference in mapping, especially in `OpenAI` cases,
/// where text and audio part does not have indexes
/// or in Google cases, where no parts have indexes,
/// we need to guess an index for the incoming delta
/// which is required in our unified interface.
pub fn guess_delta_index(
    part: &DeltaPart,
    all_content_deltas: &[ContentDelta],
    existing_matching_delta: Option<&ContentDelta>,
) -> usize {
    // First check if we already have a matching delta provided
    if let Some(matching_delta) = existing_matching_delta {
        return matching_delta.index;
    }

    // Attempt to find the LAST matching delta in all_content_deltas
    let matching_delta = all_content_deltas.iter().rev().find(|content_delta| {
        match (&content_delta.part, part) {
            // For text and audio parts, they are the matching delta
            // if their types are the same
            (DeltaPart::Text(_), DeltaPart::Text(_))
            | (DeltaPart::Audio(_), DeltaPart::Audio(_)) => true,

            // For tool calls, we can't reliably match them
            // because there can be multiple tool calls with the same tool name
            // Different types don't match
            _ => false,
        }
    });

    if let Some(matching_delta) = matching_delta {
        return matching_delta.index;
    }

    // If no matching delta found, return max index + 1
    let max_index = all_content_deltas
        .iter()
        .map(|content_delta| content_delta.index)
        .max()
        .unwrap_or(0);

    // Since we're using usize, we start from 0 instead of -1
    if max_index == 0 && all_content_deltas.is_empty() {
        0
    } else {
        max_index + 1
    }
}
