use crate::{
    language_model::{LanguageModelMetadata, LanguageModelStream},
    openai::api as openai_api,
    stream_utils,
    usage_utils::calculate_cost,
    AssistantMessage, AudioFormat, AudioPart, AudioPartDelta, ContentDelta, ImagePart,
    LanguageModel, LanguageModelError, LanguageModelInput, LanguageModelResult, Message, Modality,
    ModelResponse, ModelUsage, Part, PartDelta, PartialModelResponse, ResponseFormatJson,
    ResponseFormatOption, TextPart, TextPartDelta, Tool, ToolCallPart, ToolCallPartDelta,
    ToolChoiceOption, ToolMessage, UserMessage,
};
use async_stream::try_stream;
use eventsource_stream::{self, Eventsource};
use futures::stream::StreamExt;
use reqwest::Client;

const PROVIDER: &str = "openai";

const OPENAI_AUDIO_SAMPLE_RATE: u32 = 24_000;
const OPENAI_AUDIO_CHANNELS: u32 = 1;

#[derive(Debug, Clone, Default)]
pub struct OpenAIModelOptions {
    pub base_url: Option<String>,
    pub api_key: String,
    pub model_id: String,
}

pub struct OpenAIModel {
    model_id: String,
    api_key: String,
    base_url: String,
    client: Client,
    metadata: Option<LanguageModelMetadata>,
}

impl OpenAIModel {
    #[must_use]
    pub fn new(options: OpenAIModelOptions) -> Self {
        let client = Client::new();

        let model_id = options.model_id;
        let api_key = options.api_key;

        Self {
            model_id,
            api_key,
            base_url: "https://api.openai.com/v1".to_string(),
            client,
            metadata: None,
        }
    }

    #[must_use]
    pub fn with_metadata(mut self, metadata: LanguageModelMetadata) -> Self {
        self.metadata = Some(metadata);
        self
    }
}

#[async_trait::async_trait]
impl LanguageModel for OpenAIModel {
    fn provider(&self) -> &'static str {
        PROVIDER
    }

    fn model_id(&self) -> String {
        self.model_id.clone()
    }

    async fn generate(&self, input: LanguageModelInput) -> LanguageModelResult<ModelResponse> {
        let params = into_openai_create_params(input, self.model_id.clone())?;

        let response = self
            .client
            .post(format!("{}/chat/completions", self.base_url))
            .header("Authorization", format!("Bearer {}", self.api_key))
            .json(&params)
            .send()
            .await?;

        if response.status().is_client_error() {
            return Err(LanguageModelError::StatusCode(
                response.status(),
                response.text().await.unwrap_or_default(),
            ));
        }

        let json = response.json::<openai_api::ChatCompletion>().await?;

        let choice = json.choices.first().ok_or_else(|| {
            LanguageModelError::Invariant(PROVIDER, "No choices in response".to_string())
        })?;

        let openai_api::ChatCompletionChoice { message, .. } = choice;

        if let Some(refusal) = &message.refusal {
            return Err(LanguageModelError::Refusal(refusal.to_string()));
        }

        let content = map_openai_message(message, &params)?;

        let usage: Option<ModelUsage> = json.usage.map(Into::into);

        let cost = if let (Some(usage), Some(pricing)) = (
            usage.as_ref(),
            self.metadata.as_ref().and_then(|m| m.pricing.as_ref()),
        ) {
            Some(calculate_cost(usage, pricing))
        } else {
            None
        };

        Ok(ModelResponse {
            content,
            usage,
            cost,
        })
    }

    async fn stream(&self, input: LanguageModelInput) -> LanguageModelResult<LanguageModelStream> {
        let mut params = into_openai_create_params(input, self.model_id.clone())?;
        params.stream = Some(true);

        let mut openai_stream = self
            .client
            .post(format!("{}/chat/completions", self.base_url))
            .header("Authorization", format!("Bearer {}", self.api_key))
            .json(&params)
            .send()
            .await?
            .bytes_stream()
            .eventsource();

        let stream = try_stream! {
            let mut refusal = String::new();

            let mut all_content_deltas: Vec<ContentDelta> = Vec::new();

            while let Some(event) = openai_stream.next().await {
                match event {
                    Ok(event) => {
                        if event.data.is_empty() {
                            continue; // Skip empty events
                        }
                        if event.data == "[DONE]" {
                            break; // End of stream
                        }
                        let chunk: openai_api::ChatCompletionChunk =
                        serde_json::from_str(&event.data)
                            .map_err(|e| LanguageModelError::Invariant(
                                PROVIDER,
                                format!("Failed to parse stream chunk: {e}")
                            ))?;

                        let choice = chunk.choices.first();
                        if let Some(choice) = choice {
                            if let Some(refusal_text) = &choice.delta.refusal {
                                refusal.push_str(refusal_text);
                            }


                            let incoming_content_deltas = map_openai_delta(
                                &choice.delta,
                                &all_content_deltas,
                                &params,
                            );
                            all_content_deltas.extend(incoming_content_deltas.clone());
                            for delta in incoming_content_deltas {
                                yield PartialModelResponse {
                                    delta: Some(delta.clone()),
                                    ..Default::default()
                                 }
                            }
                        }

                        if let Some(usage) = &chunk.usage {
                            yield PartialModelResponse {
                                usage: Some(usage.clone().into()),
                                ..Default::default()
                            };
                        }
                    }
                    Err(e) => {
                        match e {
                            eventsource_stream::EventStreamError::Utf8(_) => {
                                return Err(LanguageModelError::Invariant(
                                    PROVIDER,
                                    "Receive invalid UTF-8 sequence for stream data".to_string()
                                ))?;
                            }
                            eventsource_stream::EventStreamError::Parser(error) => {
                                return Err(LanguageModelError::Invariant(
                                    PROVIDER,
                                    format!("Receive invalid EventStream data: {error}")
                                ))?;
                            },
                            eventsource_stream::EventStreamError::Transport(e) => {
                                return Err(LanguageModelError::Transport(e))?;
                            }
                        }
                    }
                }
            }
        };

        Ok(LanguageModelStream::from_stream(stream))
    }
}

fn into_openai_create_params(
    input: LanguageModelInput,
    model_id: String,
) -> LanguageModelResult<openai_api::ChatCompletionCreateParams> {
    let LanguageModelInput {
        messages,
        system_prompt,
        max_tokens,
        temperature,
        top_p,
        presence_penalty,
        frequency_penalty,
        seed,
        response_format,
        tools,
        tool_choice,
        extra,
        ..
    } = input;

    Ok(openai_api::ChatCompletionCreateParams {
        model: model_id,
        messages: into_openai_messages(&messages, system_prompt)?,
        max_completion_tokens: max_tokens,
        temperature,
        top_p,
        presence_penalty,
        frequency_penalty,
        seed,
        tools: tools
            .as_ref()
            .map(|tools| tools.iter().map(Into::into).collect()),
        tool_choice: tool_choice.as_ref().map(Into::into),
        response_format: response_format.as_ref().map(Into::into),
        modalities: input
            .modalities
            .as_ref()
            .map(|modalities| modalities.iter().map(Into::into).collect()),
        audio: extra
            .as_ref()
            .and_then(|extra| extra.get("audio"))
            .and_then(|value| {
                serde_json::from_value::<openai_api::ChatCompletionAudioParam>(value.clone()).ok()
            }),
        extra,
        ..Default::default()
    })
}

// MARK: To Provider Messages

fn into_openai_messages(
    messages: &[Message],
    system_prompt: Option<String>,
) -> LanguageModelResult<Vec<openai_api::ChatCompletionMessageParam>> {
    let mut openai_messages = vec![];

    if let Some(system_prompt) = system_prompt {
        openai_messages.push(openai_api::ChatCompletionMessageParam::System(
            openai_api::ChatCompletionSystemMessageParam {
                content: vec![openai_api::SystemContentPart::Text(
                    openai_api::ChatCompletionContentPartText {
                        text: system_prompt,
                    },
                )],
            },
        ));
    }

    for message in messages {
        match message {
            Message::User(UserMessage { content }) => {
                let openai_message_param = openai_api::ChatCompletionUserMessageParam {
                    content: content
                        .iter()
                        .map(TryInto::try_into)
                        .collect::<LanguageModelResult<_>>()?,
                    ..Default::default()
                };
                openai_messages.push(openai_api::ChatCompletionMessageParam::User(
                    openai_message_param,
                ));
            }

            Message::Assistant(AssistantMessage { content }) => {
                let mut openai_message_param =
                    openai_api::ChatCompletionAssistantMessageParam::default();

                for part in content {
                    match part {
                        Part::Text(part) => {
                            openai_message_param
                                .content
                                .get_or_insert_default()
                                .push(openai_api::AssistantContentPart::Text(part.into()));
                        }
                        Part::ToolCall(part) => {
                            openai_message_param
                                .tool_calls
                                .get_or_insert_default()
                                .push(openai_api::ChatCompletionMessageToolCall::Function(
                                    part.try_into()?,
                                ));
                        }
                        Part::Audio(part) => {
                            openai_message_param.audio = Some(part.try_into()?);
                        }
                        _ => Err(LanguageModelError::Unsupported(
                            PROVIDER,
                            format!(
                                "Cannot convert part to OpenAI assistant message for type {part:?}"
                            ),
                        ))?,
                    }
                }

                openai_messages.push(openai_api::ChatCompletionMessageParam::Assistant(
                    openai_message_param,
                ));
            }

            Message::Tool(ToolMessage { content }) => {
                for part in content {
                    let tool_part = match part {
                        Part::ToolResult(part) => part,
                        _ => Err(LanguageModelError::InvalidInput(
                            "ToolMessage content must only contain ToolResult parts".to_string(),
                        ))?,
                    };

                    openai_messages.push(openai_api::ChatCompletionMessageParam::Tool(
                        openai_api::ChatCompletionToolMessageParam {
                            tool_call_id: tool_part.tool_call_id.to_string(),
                            content: tool_part
                                .content
                                .iter()
                                .map(|p| match p {
                                    Part::Text(part) => {
                                        Ok(openai_api::ToolContentPart::Text(part.into()))
                                    }
                                    _ => Err(LanguageModelError::Unsupported(
                                        PROVIDER,
                                        format!(
                                        "Cannot convert part to OpenAI tool message for type {p:?}"
                                    ),
                                    )),
                                })
                                .collect::<LanguageModelResult<_>>()?,
                        },
                    ));
                }
            }
        }
    }

    Ok(openai_messages)
}

impl TryFrom<&Part> for openai_api::ChatCompletionContentPart {
    type Error = LanguageModelError;

    fn try_from(part: &Part) -> Result<Self, Self::Error> {
        match part {
            Part::Text(part) => Ok(Self::Text(part.into())),
            Part::Image(part) => Ok(Self::Image(part.into())),
            Part::Audio(part) => Ok(Self::InputAudio(part.try_into()?)),
            _ => Err(LanguageModelError::Unsupported(
                PROVIDER,
                format!("Cannot convert part to OpenAI content part for type {part:?}"),
            )),
        }
    }
}

impl From<&TextPart> for openai_api::ChatCompletionContentPartText {
    fn from(part: &TextPart) -> Self {
        Self {
            text: part.text.to_string(),
        }
    }
}

impl From<&ImagePart> for openai_api::ChatCompletionContentPartImage {
    fn from(part: &ImagePart) -> Self {
        Self {
            image_url: openai_api::ChatCompletionContentPartImageImageURL {
                url: format!("data:{};base64,{}", part.mime_type, part.image_data),
                detail: None,
            },
        }
    }
}

impl TryFrom<&AudioPart> for openai_api::ChatCompletionContentPartInputAudio {
    type Error = LanguageModelError;

    fn try_from(part: &AudioPart) -> Result<Self, Self::Error> {
        Ok(Self {
            input_audio: openai_api::ChatCompletionContentPartInputAudioInputAudio {
                data: part.audio_data.clone(),
                format: match part.format {
                    AudioFormat::Wav => Ok(openai_api::AudioInputFormat::Wav),
                    AudioFormat::Mp3 => Ok(openai_api::AudioInputFormat::Mp3),
                    _ => Err(LanguageModelError::Unsupported(
                        PROVIDER,
                        format!(
                        "Cannot convert audio format to OpenAI InputAudio format for format {:?}",
                        part.format
                    ),
                    )),
                }?,
            },
        })
    }
}

impl TryFrom<&AudioPart> for openai_api::ChatCompletionAssistantMessageParamAudio {
    type Error = LanguageModelError;

    fn try_from(part: &AudioPart) -> Result<Self, Self::Error> {
        let id = part.id.as_ref().ok_or_else(|| {
            LanguageModelError::Unsupported(
                PROVIDER,
                "Cannot convert audio part to OpenAI assistant message without an ID".to_string(),
            )
        })?;

        Ok(Self { id: id.to_string() })
    }
}

impl TryFrom<&ToolCallPart> for openai_api::ChatCompletionMessageFunctionToolCall {
    type Error = LanguageModelError;

    fn try_from(part: &ToolCallPart) -> Result<Self, Self::Error> {
        Ok(Self {
            id: part.tool_call_id.to_string(),
            function: openai_api::ChatCompletionMessageFunctionToolCallFunction {
                name: part.tool_name.to_string(),
                arguments: part.args.to_string(),
            },
        })
    }
}

// MARK: To Provider Tools

impl From<&Tool> for openai_api::ChatCompletionTool {
    fn from(value: &Tool) -> Self {
        Self::Function(openai_api::ChatCompletionFunctionTool {
            function: openai_api::FunctionDefinition {
                name: value.name.clone(),
                description: Some(value.description.clone()),
                parameters: Some(value.parameters.clone()),
                strict: Some(true),
            },
        })
    }
}

impl From<&ToolChoiceOption> for openai_api::ChatCompletionToolChoiceOption {
    fn from(tool_choice: &ToolChoiceOption) -> Self {
        match tool_choice {
            ToolChoiceOption::None => Self::None,
            ToolChoiceOption::Auto => Self::Auto,
            ToolChoiceOption::Required => Self::Required,
            ToolChoiceOption::Tool(tool) => {
                Self::Named(openai_api::ChatCompletionNamedToolChoice {
                    function: openai_api::ChatCompletionNamedToolChoiceFunction {
                        name: tool.tool_name.clone(),
                    },
                    type_: "function".to_string(),
                })
            }
        }
    }
}

// MARK: To Provider Response Format

impl From<&ResponseFormatOption> for openai_api::ResponseFormat {
    fn from(response_format: &ResponseFormatOption) -> Self {
        match response_format {
            ResponseFormatOption::Text => Self::Text,
            ResponseFormatOption::Json(ResponseFormatJson {
                name,
                description,
                schema,
            }) => {
                if let Some(schema) = schema {
                    Self::JsonSchema(openai_api::ResponseFormatJSONSchema {
                        json_schema: openai_api::ResponseFormatJSONSchemaJSONSchema {
                            name: name.clone(),
                            description: description.clone(),
                            schema: Some(schema.clone()),
                            strict: Some(true),
                        },
                    })
                } else {
                    Self::JsonObject
                }
            }
        }
    }
}

// MARK: To Provider Modality

impl From<&Modality> for openai_api::Modality {
    fn from(modality: &Modality) -> Self {
        match modality {
            Modality::Text => Self::Text,
            Modality::Audio => Self::Audio,
        }
    }
}

// MARK: To SDK Message

fn map_openai_message(
    message: &openai_api::CompletionsCompletionsAPIChatCompletionMessage,
    create_params: &openai_api::ChatCompletionCreateParams,
) -> LanguageModelResult<Vec<Part>> {
    let mut parts = vec![];

    if let Some(content) = &message.content {
        parts.push(Part::Text(TextPart {
            text: content.to_string(),
            id: None,
        }));
    }

    if let Some(tool_calls) = &message.tool_calls {
        for tool_call in tool_calls {
            match tool_call {
                openai_api::ChatCompletionMessageToolCall::Function(function_tool_call) => {
                    parts.push(Part::ToolCall(function_tool_call.try_into()?));
                }
            }
        }
    }

    if let Some(audio) = &message.audio {
        let mut audio_part = AudioPart {
            id: Some(audio.id.to_string()),
            format: AudioFormat::from(
                &create_params
                    .audio
                    .as_ref()
                    .ok_or_else(|| {
                        LanguageModelError::Invariant(
                            PROVIDER,
                            "Audio returned from OpenAI API but no audio parameter was provided"
                                .to_string(),
                        )
                    })?
                    .format,
            ),
            audio_data: audio.data.to_string(),
            transcript: Some(audio.transcript.clone()),
            ..Default::default()
        };
        if matches!(audio_part.format, AudioFormat::Linear16) {
            audio_part.sample_rate = Some(OPENAI_AUDIO_SAMPLE_RATE);
            audio_part.channels = Some(OPENAI_AUDIO_CHANNELS);
        }
        parts.push(Part::Audio(audio_part));
    }

    Ok(parts)
}

impl From<&openai_api::AudioOutputFormat> for AudioFormat {
    fn from(format: &openai_api::AudioOutputFormat) -> Self {
        match format {
            openai_api::AudioOutputFormat::Wav => Self::Wav,
            openai_api::AudioOutputFormat::Mp3 => Self::Mp3,
            openai_api::AudioOutputFormat::Aac => Self::Aac,
            openai_api::AudioOutputFormat::Flac => Self::Flac,
            openai_api::AudioOutputFormat::Opus => Self::Opus,
            openai_api::AudioOutputFormat::Pcm16 => Self::Linear16,
        }
    }
}

impl TryFrom<&openai_api::ChatCompletionMessageFunctionToolCall> for ToolCallPart {
    type Error = LanguageModelError;

    fn try_from(
        value: &openai_api::ChatCompletionMessageFunctionToolCall,
    ) -> Result<Self, Self::Error> {
        let args_value: serde_json::Value = serde_json::from_str(&value.function.arguments)
            .map_err(|e| {
                LanguageModelError::InvalidInput(format!(
                    "failed to parse tool arguments JSON: {e}"
                ))
            })?;

        Ok(Self {
            tool_call_id: value.id.to_string(),
            tool_name: value.function.name.to_string(),
            args: args_value,
            id: None,
        })
    }
}

// MARK: To SDK Delta

fn map_openai_delta(
    delta: &openai_api::ChatCompletionChunkChoiceDelta,
    existing_content_deltas: &[ContentDelta],
    create_params: &openai_api::ChatCompletionCreateParams,
) -> Vec<ContentDelta> {
    let mut content_deltas = vec![];

    if let Some(content) = &delta.content {
        let text_part = TextPartDelta {
            text: content.clone(),
            id: None,
        };
        let part = PartDelta::Text(text_part);
        let index = stream_utils::guess_delta_index(
            &part,
            &[existing_content_deltas, content_deltas.as_slice()].concat(),
            None,
        );
        content_deltas.push(ContentDelta { index, part });
    }

    if let Some(audio) = &delta.audio {
        let mut audio_part = AudioPartDelta::default();
        if let Some(id) = &audio.id {
            audio_part.id = Some(id.to_string());
        }
        if let Some(data) = &audio.data {
            audio_part.audio_data = Some(data.to_string());
            audio_part.format = create_params
                .audio
                .as_ref()
                .map(|audio_param| &audio_param.format)
                .map(AudioFormat::from);
            audio_part.sample_rate = Some(OPENAI_AUDIO_SAMPLE_RATE);
            audio_part.channels = Some(OPENAI_AUDIO_CHANNELS);
        }
        if let Some(transcript) = &audio.transcript {
            audio_part.transcript = Some(transcript.to_string());
        }
        let part = PartDelta::Audio(audio_part);
        let index = stream_utils::guess_delta_index(
            &part,
            &[existing_content_deltas, content_deltas.as_slice()].concat(),
            None,
        );
        content_deltas.push(ContentDelta { index, part });
    }

    if let Some(tool_calls) = &delta.tool_calls {
        for tool_call in tool_calls {
            let mut tool_call_part = ToolCallPartDelta::default();
            if let Some(id) = &tool_call.id {
                tool_call_part.tool_call_id = Some(id.to_string());
            }
            if let Some(function) = &tool_call.function {
                if let Some(name) = &function.name {
                    tool_call_part.tool_name = Some(name.to_string());
                }
                if let Some(args) = &function.arguments {
                    tool_call_part.args = Some(args.to_string());
                }
            }

            let part = PartDelta::ToolCall(tool_call_part);
            let index = stream_utils::guess_delta_index(
                &part,
                &[existing_content_deltas, content_deltas.as_slice()].concat(),
                Some(tool_call.index),
            );

            content_deltas.push(ContentDelta { index, part });
        }
    }

    content_deltas
}

// MARK: To SDK Usage

impl From<openai_api::CompletionsAPICompletionUsage> for ModelUsage {
    fn from(value: openai_api::CompletionsAPICompletionUsage) -> Self {
        // TODO: map details token
        Self {
            input_tokens: value.prompt_tokens,
            output_tokens: value.completion_tokens,
            ..Default::default()
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{generate_common_tests, test_utils::prelude::*, StreamAccumulator};
    use std::{env, sync::LazyLock};
    pub use tokio::test;

    static OPENAI_MODEL: LazyLock<OpenAIModel> = LazyLock::new(|| {
        dotenvy::dotenv().ok();

        OpenAIModel::new(OpenAIModelOptions {
            model_id: "gpt-4o".to_string(),
            api_key: env::var("OPENAI_API_KEY")
                .expect("OPENAI_API_KEY must be set")
                .to_string(),
            ..Default::default()
        })
    });

    static OPENAI_AUDIO_MODEL: LazyLock<OpenAIModel> = LazyLock::new(|| {
        dotenvy::dotenv().ok();

        OpenAIModel::new(OpenAIModelOptions {
            model_id: "gpt-4o-audio-preview".to_string(),
            api_key: env::var("OPENAI_API_KEY")
                .expect("OPENAI_API_KEY must be set")
                .to_string(),
            ..Default::default()
        })
    });

    generate_common_tests! {
        model_name: OPENAI_MODEL,
    }

    #[test]
    async fn test_generate_audio() -> Result<(), Box<dyn Error>> {
        let response = OPENAI_AUDIO_MODEL
            .generate(LanguageModelInput {
                modalities: Some(vec![Modality::Text, Modality::Audio]),
                extra: Some(serde_json::json!({
                    "audio": {
                        "voice": "alloy",
                        "format": "pcm16"
                    }
                })),
                messages: vec![Message::User(UserMessage {
                    content: vec![Part::Text(TextPart {
                        text: "Hello".to_string(),
                        id: None,
                    })],
                })],
                ..Default::default()
            })
            .await?;

        let audio_part = response
            .content
            .iter()
            .find_map(|part| match part {
                Part::Audio(audio) => Some(audio),
                _ => None,
            })
            .ok_or_else(|| "Audio part must be present".to_string())?
            .clone();

        assert!(
            !audio_part.audio_data.is_empty(),
            "Audio data must be present"
        );
        assert!(
            audio_part.transcript.is_some_and(|t| !t.is_empty()),
            "Transcript must be present"
        );
        assert!(
            audio_part.id.is_some_and(|id| !id.is_empty()),
            "Audio part ID must be present"
        );

        Ok(())
    }

    #[test]
    async fn test_stream_audio() -> Result<(), Box<dyn Error>> {
        let mut stream = OPENAI_AUDIO_MODEL
            .stream(LanguageModelInput {
                modalities: Some(vec![Modality::Text, Modality::Audio]),
                extra: Some(serde_json::json!({
                    "audio": {
                        "voice": "alloy",
                        "format": "pcm16"
                    }
                })),
                messages: vec![Message::User(UserMessage {
                    content: vec![Part::Text(TextPart {
                        text: "Hello".to_string(),
                        id: None,
                    })],
                })],
                ..Default::default()
            })
            .await?;

        let mut accumulator = StreamAccumulator::new();

        while let Some(partial_response) = stream.next().await {
            let partial_response = partial_response.unwrap();
            accumulator.add_partial(&partial_response).unwrap();
            println!("{partial_response:#?}");
        }

        let response = accumulator.compute_response()?;

        let audio_part = response
            .content
            .iter()
            .find_map(|part| match part {
                Part::Audio(audio) => Some(audio),
                _ => None,
            })
            .ok_or_else(|| "Audio part must be present".to_string())?
            .clone();

        assert!(
            !audio_part.audio_data.is_empty(),
            "Audio data must be present"
        );
        assert!(
            audio_part.transcript.is_some_and(|t| !t.is_empty()),
            "Transcript must be present"
        );
        assert!(
            audio_part.id.is_some_and(|id| !id.is_empty()),
            "Audio part ID must be present"
        );

        Ok(())
    }
}
