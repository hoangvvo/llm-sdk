use crate::{
    AudioPartDelta, ContentDelta, ImagePartDelta, LanguageModelError, LanguageModelResult, Part,
    PartDelta, ReasoningPartDelta, TextPartDelta, ToolCallPartDelta,
};

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
    part: &PartDelta,
    all_content_deltas: &[ContentDelta],
    tool_call_index: Option<usize>,
) -> usize {
    // contentDeltas may have the structure of
    // [part0 partial, part0 partial, part1 partial].
    // For the purpose of this matching, we want only
    // [part0, part1]
    let unique_content_deltas = all_content_deltas
        .iter()
        .enumerate()
        .filter(|(index, content_delta)| {
            all_content_deltas
                .iter()
                .position(|find_part| find_part.index == content_delta.index)
                == Some(*index)
        })
        .map(|(_, content_delta)| content_delta)
        .collect::<Vec<_>>();

    if let (Some(tool_call_index), PartDelta::ToolCall(_)) = (tool_call_index, part) {
        // Providers like OpenAI track tool calls in a separate field, so we
        // need to reconcile that. To understand how this matching works:
        // [Provider]
        // toolCalls: [index 0] [index 1]
        // [LLM-SDK state]
        // parts: [index 0 text] [index 1 tool] [index 2 text] [index 3 tool]
        // In this case, we need to map the tool index 0 -> 1 and 1 -> 3
        let tool_part_deltas: Vec<_> = unique_content_deltas
            .iter()
            .filter(|content_delta| matches!(content_delta.part, PartDelta::ToolCall(_)))
            .collect();

        let existing_tool_call_delta = tool_part_deltas.get(tool_call_index).copied();

        if let Some(existing_tool_call_delta) = existing_tool_call_delta {
            return existing_tool_call_delta.index;
        }
        // If no matching tool call delta found, return the length of
        // unique_content_deltas This is because we want to append a new tool
        // call delta
        return unique_content_deltas.len();
    }

    // Attempt to find the LAST matching delta in unique_content_deltas
    let matching_delta = unique_content_deltas.iter().rev().find(|content_delta| {
        match (&content_delta.part, part) {
            // For text and audio parts, they are the matching delta
            // if their types are the same. This is because providers that do not
            // provide indexes like only have 1 part for each type (e.g openai has only 1
            // message.content or 1 message.audio)
            (PartDelta::Text(_), PartDelta::Text(_))
            | (PartDelta::Audio(_), PartDelta::Audio(_)) => true,

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
    let max_index = unique_content_deltas
        .iter()
        .map(|content_delta| content_delta.index)
        .max()
        .unwrap_or(0);

    // Since we're using usize, we start from 0 instead of -1
    if max_index == 0 && unique_content_deltas.is_empty() {
        0
    } else {
        max_index + 1
    }
}

pub fn loosely_convert_part_to_part_delta(part: Part) -> LanguageModelResult<PartDelta> {
    Ok(match part {
        Part::Text(text_part) => PartDelta::Text(TextPartDelta {
            text: text_part.text,
        }),
        Part::Image(image_part) => PartDelta::Image(ImagePartDelta {
            image_data: Some(image_part.image_data),
            mime_type: Some(image_part.mime_type),
            width: image_part.width,
            height: image_part.height,
            id: image_part.id,
        }),
        Part::Audio(audio_part) => PartDelta::Audio(AudioPartDelta {
            audio_data: Some(audio_part.audio_data),
            format: Some(audio_part.format),
            sample_rate: audio_part.sample_rate,
            channels: audio_part.channels,
            id: audio_part.id,
            transcript: audio_part.transcript,
        }),
        Part::ToolCall(tool_call_part) => PartDelta::ToolCall(ToolCallPartDelta {
            tool_name: Some(tool_call_part.tool_name),
            args: Some(tool_call_part.args.to_string()),
            tool_call_id: Some(tool_call_part.tool_call_id),
            id: tool_call_part.id,
        }),
        Part::Reasoning(reasoning_part) => PartDelta::Reasoning(ReasoningPartDelta {
            text: Some(reasoning_part.text),
            signature: reasoning_part.signature,
            id: reasoning_part.id,
        }),
        _ => Err(LanguageModelError::Invariant(
            "",
            "Cannot convert part to part delta".to_string(),
        ))?,
    })
}
