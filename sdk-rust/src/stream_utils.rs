use crate::{ContentDelta, DeltaPart};

/// Because of the difference in mapping, especially in `OpenAI` cases,
/// where text and audio part does not have indexes
/// or in Google cases, where no parts have indexes,
/// we need to guess an index for the incoming delta
/// which is required in our unified interface.
///
/// toolCallIndex does not always correspond to the index of the tool call in
/// the deltas because some providers keep tool call separate from other parts
/// (e.g openai). We can match this against the existing tool call deltas
pub fn guess_delta_index(
    part: &DeltaPart,
    all_content_deltas: &[ContentDelta],
    tool_call_index: Option<usize>,
) -> usize {
    //     if (part.type === "tool-call" && typeof toolCallIndex === "number") {
    //     const toolPartDeltas = allContentDeltas.filter(
    //       (contentDelta) => contentDelta.part.type === "tool-call",
    //     );
    //     const existingToolCallDelta = toolPartDeltas[toolCallIndex];
    //     if (existingToolCallDelta) {
    //       return existingToolCallDelta.index;
    //     }
    //   }
    if let (Some(tool_call_index), DeltaPart::ToolCall(_)) = (tool_call_index, part) {
        let mut existing_tool_call_deltas = all_content_deltas
            .iter()
            .filter(|content_delta| matches!(content_delta.part, DeltaPart::ToolCall(_)));
        if let Some(existing_tool_call_delta) = existing_tool_call_deltas.nth(tool_call_index) {
            return existing_tool_call_delta.index;
        }
    }

    // Attempt to find the LAST matching delta in all_content_deltas
    let matching_delta = all_content_deltas.iter().rev().find(|content_delta| {
        match (&content_delta.part, part) {
            // For text and audio parts, they are the matching delta
            // if their types are the same. This is because providers that do not
            // provide indexes like only have 1 part for each type (e.g openai has only 1
            // message.content or 1 message.audio)
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
