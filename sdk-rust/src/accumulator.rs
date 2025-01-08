use crate::{
    audio_utils, AudioFormat, AudioPart, AudioPartDelta, ContentDelta, DeltaPart,
    LanguageModelError, LanguageModelResult, ModelResponse, ModelUsage, Part, PartialModelResponse,
    TextPart, TextPartDelta, ToolCallPart, ToolCallPartDelta,
};
use serde_json::Value;
use std::collections::BTreeMap;

/// Internal representation of accumulated text data
#[derive(Debug, Clone)]
struct AccumulatedTextData {
    text: String,
    id: Option<String>,
}

/// Internal representation of accumulated tool call data
#[derive(Debug, Clone)]
struct AccumulatedToolCallData {
    tool_name: String,
    tool_call_id: Option<String>,
    args: String,
    id: Option<String>,
}

/// Internal representation of accumulated audio data
#[derive(Debug, Clone)]
struct AccumulatedAudioData {
    audio_data_chunks: Vec<String>,
    format: Option<AudioFormat>,
    sample_rate: Option<u32>,
    channels: Option<u32>,
    transcript: String,
    id: Option<String>,
}

/// Represents accumulated data for different part types
#[derive(Debug, Clone)]
enum AccumulatedData {
    Text(AccumulatedTextData),
    ToolCall(AccumulatedToolCallData),
    Audio(AccumulatedAudioData),
}

/// Initializes accumulated data from a delta
fn initialize_accumulated_data(delta: ContentDelta) -> AccumulatedData {
    match delta.part {
        DeltaPart::Text(text_delta) => AccumulatedData::Text(AccumulatedTextData {
            text: text_delta.text,
            id: text_delta.id,
        }),
        DeltaPart::ToolCall(tool_delta) => AccumulatedData::ToolCall(AccumulatedToolCallData {
            tool_name: tool_delta.tool_name.unwrap_or_default(),
            tool_call_id: tool_delta.tool_call_id,
            args: tool_delta.args.unwrap_or_default(),
            id: tool_delta.id,
        }),
        DeltaPart::Audio(audio_delta) => AccumulatedData::Audio(AccumulatedAudioData {
            audio_data_chunks: audio_delta
                .audio_data
                .map(|data| vec![data])
                .unwrap_or_default(),
            format: audio_delta.format,
            sample_rate: audio_delta.sample_rate,
            channels: audio_delta.channels,
            transcript: audio_delta.transcript.unwrap_or_default(),
            id: audio_delta.id,
        }),
    }
}

/// Merges text delta with existing text data
fn merge_text_delta(existing: &mut AccumulatedTextData, delta: TextPartDelta) {
    existing.text.push_str(&delta.text);
    if delta.id.is_some() {
        existing.id = delta.id;
    }
}

/// Merges tool call delta with existing tool call data
fn merge_tool_call_delta(existing: &mut AccumulatedToolCallData, delta: ToolCallPartDelta) {
    if let Some(tool_name) = delta.tool_name {
        existing.tool_name.push_str(&tool_name);
    }
    if delta.tool_call_id.is_some() {
        existing.tool_call_id = delta.tool_call_id;
    }
    if let Some(args) = delta.args {
        existing.args.push_str(&args);
    }
    if delta.id.is_some() {
        existing.id = delta.id;
    }
}

/// Merges audio delta with existing audio data
fn merge_audio_delta(existing: &mut AccumulatedAudioData, delta: AudioPartDelta) {
    if let Some(audio_data) = delta.audio_data {
        existing.audio_data_chunks.push(audio_data);
    }
    if delta.format.is_some() {
        existing.format = delta.format;
    }
    if delta.sample_rate.is_some() {
        existing.sample_rate = delta.sample_rate;
    }
    if delta.channels.is_some() {
        existing.channels = delta.channels;
    }
    if let Some(transcript) = delta.transcript {
        existing.transcript.push_str(&transcript);
    }
    if delta.id.is_some() {
        existing.id = delta.id;
    }
}

/// Merges an incoming delta with existing accumulated data
fn merge_delta(existing: &mut AccumulatedData, delta: ContentDelta) -> Result<(), String> {
    match (existing, delta.part) {
        (AccumulatedData::Text(ref mut existing_text), DeltaPart::Text(text_delta)) => {
            merge_text_delta(existing_text, text_delta);
        }
        (AccumulatedData::ToolCall(ref mut existing_tool), DeltaPart::ToolCall(tool_delta)) => {
            merge_tool_call_delta(existing_tool, tool_delta);
        }
        (AccumulatedData::Audio(ref mut existing_audio), DeltaPart::Audio(audio_delta)) => {
            merge_audio_delta(existing_audio, audio_delta);
        }
        _ => Err(format!(
            "Type mismatch at index {}: existing type doesn't match incoming type",
            delta.index
        ))?,
    }

    Ok(())
}

/// Creates a text part from accumulated text data
fn create_text_part(data: AccumulatedTextData) -> Part {
    Part::Text(TextPart {
        text: data.text,
        id: data.id,
    })
}

/// Parses tool call arguments from JSON string
fn parse_tool_call_args(args: &str) -> LanguageModelResult<Value> {
    if args.trim().is_empty() {
        return Ok(Value::Object(serde_json::Map::new()));
    }

    serde_json::from_str(args).map_err(|e| {
        LanguageModelError::Invariant("", format!("Invalid tool call arguments: {args}: {e}"))
    })
}

/// Creates a tool call part from accumulated tool call data
fn create_tool_call_part(data: AccumulatedToolCallData, index: usize) -> LanguageModelResult<Part> {
    let tool_call_id = data.tool_call_id.ok_or_else(|| {
        LanguageModelError::Invariant(
            "",
            format!("Missing required field tool_call_id at index {index}"),
        )
    })?;

    if data.tool_name.is_empty() {
        return Err(LanguageModelError::Invariant(
            "",
            format!("Missing required field tool_name at index {index}"),
        ));
    }

    Ok(Part::ToolCall(ToolCallPart {
        tool_call_id,
        tool_name: data.tool_name,
        args: parse_tool_call_args(&data.args)?,
        id: data.id,
    }))
}

/// Concatenates audio data chunks into a single base64 string
fn concatenate_audio_chunks(chunks: &[String]) -> LanguageModelResult<String> {
    if chunks.is_empty() {
        return Ok(String::new());
    }

    // Decode all chunks and collect samples
    let mut all_samples: Vec<i16> = Vec::new();

    for chunk in chunks {
        let samples = audio_utils::base64_to_i16sample(chunk).map_err(|e| {
            LanguageModelError::Invariant("", format!("Failed to decode audio chunk: {e}"))
        })?;
        all_samples.extend(samples);
    }

    let b64 = audio_utils::i16sample_to_base64(&all_samples);

    Ok(b64)
}

/// Creates an audio part from accumulated audio data
fn create_audio_part(data: AccumulatedAudioData) -> LanguageModelResult<Part> {
    let format = data.format.ok_or_else(|| {
        LanguageModelError::Invariant(
            "",
            "Missing required field format for audio part".to_string(),
        )
    })?;

    if !matches!(format, AudioFormat::Linear16) {
        return Err(LanguageModelError::NotImplemented(
            "",
            format!(
                "Only linear16 format is supported for audio concatenation. Received: {format:?}"
            ),
        ));
    }

    let concatenated_audio = concatenate_audio_chunks(&data.audio_data_chunks)?;

    Ok(Part::Audio(AudioPart {
        audio_data: concatenated_audio,
        format,
        sample_rate: data.sample_rate,
        channels: data.channels,
        transcript: if data.transcript.is_empty() {
            None
        } else {
            Some(data.transcript)
        },
        id: data.id,
    }))
}

/// Creates a final Part from accumulated data
fn create_part(data: AccumulatedData, index: usize) -> LanguageModelResult<Part> {
    match data {
        AccumulatedData::Text(text_data) => Ok(create_text_part(text_data)),
        AccumulatedData::ToolCall(tool_data) => create_tool_call_part(tool_data, index),
        AccumulatedData::Audio(audio_data) => create_audio_part(audio_data),
    }
}

/// Manages the accumulation and merging of content deltas for streaming
/// responses
pub struct StreamAccumulator {
    /// Map of index to accumulated data, using `BTreeMap` for automatic sorting
    accumulated_parts: BTreeMap<usize, AccumulatedData>,
    /// Accumulated usage statistics
    accumulated_usage: Option<ModelUsage>,
}

impl StreamAccumulator {
    /// Creates a new `StreamAccumulator`
    #[must_use]
    pub fn new() -> Self {
        Self {
            accumulated_parts: BTreeMap::new(),
            accumulated_usage: None,
        }
    }

    /// Adds a chunk of content deltas to the accumulator
    ///
    /// # Errors
    /// Returns an error if delta types mismatch for the same index
    pub fn add_partial(&mut self, partial: &PartialModelResponse) -> Result<(), String> {
        if let Some(delta) = &partial.delta {
            self.process_delta(delta.clone())?;
        }
        if let Some(usage) = &partial.usage {
            self.process_usage(usage.clone())?;
        }
        Ok(())
    }

    /// Computes the final response from accumulated deltas
    ///
    /// # Errors
    /// Returns an error if required fields are missing or format is unsupported
    pub fn compute_response(self) -> LanguageModelResult<ModelResponse> {
        let content = self
            .accumulated_parts
            .into_iter()
            .map(|(index, data)| create_part(data, index))
            .collect::<Result<Vec<_>, _>>()?;

        Ok(ModelResponse {
            content,
            cost: None,
            usage: self.accumulated_usage,
        })
    }

    /// Clears all accumulated data
    pub fn clear(&mut self) {
        self.accumulated_parts.clear();
    }

    /// Gets the number of accumulated parts
    #[must_use]
    pub fn size(&self) -> usize {
        self.accumulated_parts.len()
    }

    /// Checks if the accumulator has any data
    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.accumulated_parts.is_empty()
    }

    /// Processes a single delta, either merging with existing or creating new
    fn process_delta(&mut self, delta: ContentDelta) -> Result<(), String> {
        let index = delta.index;

        if let Some(existing) = self.accumulated_parts.get_mut(&index) {
            merge_delta(existing, delta)
        } else {
            let accumulated = initialize_accumulated_data(delta);
            self.accumulated_parts.insert(index, accumulated);
            Ok(())
        }
    }

    fn process_usage(&mut self, usage: ModelUsage) -> Result<(), String> {
        let accumulated_usage = self
            .accumulated_usage
            .get_or_insert_with(ModelUsage::default);

        accumulated_usage.input_tokens += usage.input_tokens;
        accumulated_usage.output_tokens += usage.output_tokens;

        Ok(())
    }
}

impl Default for StreamAccumulator {
    fn default() -> Self {
        Self::new()
    }
}
