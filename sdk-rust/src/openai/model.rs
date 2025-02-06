use crate::{
    language_model::{LanguageModelMetadata, LanguageModelStream},
    openai::api as openai_api,
    source_part_utils, stream_utils,
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

    fn metadata(&self) -> Option<&LanguageModelMetadata> {
        self.metadata.as_ref()
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

        let choice = json.choices.into_iter().next().ok_or_else(|| {
            LanguageModelError::Invariant(PROVIDER, "No choices in response".to_string())
        })?;

        let openai_api::ChatCompletionChoice { message, .. } = choice;

        if let Some(refusal) = message.refusal {
            return Err(LanguageModelError::Refusal(refusal));
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

                        let choice = chunk.choices.into_iter().next();
                        if let Some(choice) = choice {
                            if let Some(refusal_text) = &choice.delta.refusal {
                                refusal.push_str(refusal_text);
                            }


                            let incoming_content_deltas = map_openai_delta(
                                choice.delta,
                                &all_content_deltas,
                                &params,
                            );
                            all_content_deltas.extend(incoming_content_deltas.clone());
                            for delta in incoming_content_deltas {
                                yield PartialModelResponse {
                                    delta: Some(delta),
                                    ..Default::default()
                                 }
                            }
                        }

                        if let Some(usage) = chunk.usage {
                            yield PartialModelResponse {
                                usage: Some(usage.into()),
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
        messages: into_openai_messages(messages, system_prompt)?,
        max_completion_tokens: max_tokens,
        temperature,
        top_p,
        presence_penalty,
        frequency_penalty,
        seed,
        tools: tools.map(|tools| tools.into_iter().map(Into::into).collect()),
        tool_choice: tool_choice.map(Into::into),
        response_format: response_format.map(Into::into),
        modalities: input
            .modalities
            .map(|modalities| modalities.into_iter().map(Into::into).collect()),
        audio: extra
            .as_ref()
            .and_then(|extra| extra.get("audio").cloned())
            .and_then(|value| {
                serde_json::from_value::<openai_api::ChatCompletionAudioParam>(value).ok()
            }),
        extra,
        ..Default::default()
    })
}

// MARK: To Provider Messages

fn into_openai_messages(
    messages: Vec<Message>,
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
                let message_parts =
                    source_part_utils::get_compatible_parts_without_source_parts(content);
                let openai_message_param = openai_api::ChatCompletionUserMessageParam {
                    content: message_parts
                        .into_iter()
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

                let message_parts =
                    source_part_utils::get_compatible_parts_without_source_parts(content);

                for part in message_parts {
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

                    let tool_result_part_content =
                        source_part_utils::get_compatible_parts_without_source_parts(
                            tool_part.content,
                        );

                    openai_messages.push(openai_api::ChatCompletionMessageParam::Tool(
                        openai_api::ChatCompletionToolMessageParam {
                            tool_call_id: tool_part.tool_call_id,
                            content: tool_result_part_content
                                .into_iter()
                                .map(|p| match p {
                                    Part::Text(part) => {
                                        Ok(openai_api::ChatCompletionToolMessageParamToolContentPart::Text(part.into()))
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

impl TryFrom<Part> for openai_api::ChatCompletionContentPart {
    type Error = LanguageModelError;

    fn try_from(part: Part) -> Result<Self, Self::Error> {
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

impl From<TextPart> for openai_api::ChatCompletionContentPartText {
    fn from(part: TextPart) -> Self {
        Self { text: part.text }
    }
}

impl From<ImagePart> for openai_api::ChatCompletionContentPartImage {
    fn from(part: ImagePart) -> Self {
        Self {
            image_url: openai_api::ChatCompletionContentPartImageImageURL {
                url: format!("data:{};base64,{}", part.mime_type, part.image_data),
                detail: None,
            },
        }
    }
}

impl TryFrom<AudioPart> for openai_api::ChatCompletionContentPartInputAudio {
    type Error = LanguageModelError;

    fn try_from(part: AudioPart) -> Result<Self, Self::Error> {
        Ok(Self {
            input_audio: openai_api::ChatCompletionContentPartInputAudioInputAudio {
                data: part.audio_data,
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

impl TryFrom<AudioPart> for openai_api::ChatCompletionAssistantMessageParamAudio {
    type Error = LanguageModelError;

    fn try_from(part: AudioPart) -> Result<Self, Self::Error> {
        let id = part.audio_id.ok_or_else(|| {
            LanguageModelError::Unsupported(
                PROVIDER,
                "Cannot convert audio part to OpenAI assistant message without an ID".to_string(),
            )
        })?;

        Ok(Self { id })
    }
}

impl TryFrom<ToolCallPart> for openai_api::ChatCompletionMessageFunctionToolCall {
    type Error = LanguageModelError;

    fn try_from(part: ToolCallPart) -> Result<Self, Self::Error> {
        Ok(Self {
            id: part.tool_call_id,
            function: openai_api::ChatCompletionMessageFunctionToolCallFunction {
                name: part.tool_name,
                arguments: part.args.to_string(),
            },
        })
    }
}

// MARK: To Provider Tools

impl From<Tool> for openai_api::ChatCompletionTool {
    fn from(value: Tool) -> Self {
        Self::Function(openai_api::ChatCompletionFunctionTool {
            function: openai_api::FunctionDefinition {
                name: value.name,
                description: Some(value.description),
                parameters: Some(value.parameters),
                strict: Some(true),
            },
        })
    }
}

impl From<ToolChoiceOption> for openai_api::ChatCompletionToolChoiceOption {
    fn from(tool_choice: ToolChoiceOption) -> Self {
        match tool_choice {
            ToolChoiceOption::None => Self::None,
            ToolChoiceOption::Auto => Self::Auto,
            ToolChoiceOption::Required => Self::Required,
            ToolChoiceOption::Tool(tool) => {
                Self::Named(openai_api::ChatCompletionNamedToolChoice {
                    function: openai_api::ChatCompletionNamedToolChoiceFunction {
                        name: tool.tool_name,
                    },
                    type_: "function".to_string(),
                })
            }
        }
    }
}

// MARK: To Provider Response Format

impl From<ResponseFormatOption> for openai_api::ResponseFormat {
    fn from(response_format: ResponseFormatOption) -> Self {
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
                            name,
                            description,
                            schema: Some(schema),
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

impl From<Modality> for openai_api::Modality {
    fn from(modality: Modality) -> Self {
        match modality {
            Modality::Text => Self::Text,
            Modality::Audio => Self::Audio,
        }
    }
}

// MARK: To SDK Message

fn map_openai_message(
    message: openai_api::CompletionsCompletionsAPIChatCompletionMessage,
    create_params: &openai_api::ChatCompletionCreateParams,
) -> LanguageModelResult<Vec<Part>> {
    let mut parts = vec![];

    if let Some(content) = message.content {
        parts.push(Part::Text(TextPart { text: content }));
    }

    if let Some(tool_calls) = message.tool_calls {
        for tool_call in tool_calls {
            match tool_call {
                openai_api::ChatCompletionMessageToolCall::Function(function_tool_call) => {
                    parts.push(Part::ToolCall(function_tool_call.try_into()?));
                }
            }
        }
    }

    if let Some(audio) = message.audio {
        let mut audio_part = AudioPart {
            audio_id: Some(audio.id),
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
            audio_data: audio.data,
            transcript: Some(audio.transcript),
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

impl TryFrom<openai_api::ChatCompletionMessageFunctionToolCall> for ToolCallPart {
    type Error = LanguageModelError;

    fn try_from(
        value: openai_api::ChatCompletionMessageFunctionToolCall,
    ) -> Result<Self, Self::Error> {
        let args_value: serde_json::Value = serde_json::from_str(&value.function.arguments)
            .map_err(|e| {
                LanguageModelError::InvalidInput(format!(
                    "failed to parse tool arguments JSON: {e}"
                ))
            })?;

        Ok(Self {
            tool_call_id: value.id,
            tool_name: value.function.name,
            args: args_value,
        })
    }
}

// MARK: To SDK Delta

fn map_openai_delta(
    delta: openai_api::ChatCompletionChunkChoiceDelta,
    existing_content_deltas: &[ContentDelta],
    create_params: &openai_api::ChatCompletionCreateParams,
) -> Vec<ContentDelta> {
    let mut content_deltas = vec![];

    if let Some(content) = delta.content {
        if !content.is_empty() {
            let text_part = TextPartDelta { text: content };
            let part = PartDelta::Text(text_part);
            let index = stream_utils::guess_delta_index(
                &part,
                &[existing_content_deltas, content_deltas.as_slice()].concat(),
                None,
            );
            content_deltas.push(ContentDelta { index, part });
        }
    }

    if let Some(audio) = delta.audio {
        let mut audio_part = AudioPartDelta::default();
        if let Some(id) = audio.id {
            audio_part.audio_id = Some(id);
        }
        if let Some(data) = audio.data {
            audio_part.audio_data = Some(data);
            audio_part.format = create_params
                .audio
                .as_ref()
                .map(|audio_param| &audio_param.format)
                .map(AudioFormat::from);
            audio_part.sample_rate = Some(OPENAI_AUDIO_SAMPLE_RATE);
            audio_part.channels = Some(OPENAI_AUDIO_CHANNELS);
        }
        if let Some(transcript) = audio.transcript {
            audio_part.transcript = Some(transcript);
        }
        let part = PartDelta::Audio(audio_part);
        let index = stream_utils::guess_delta_index(
            &part,
            &[existing_content_deltas, content_deltas.as_slice()].concat(),
            None,
        );
        content_deltas.push(ContentDelta { index, part });
    }

    if let Some(tool_calls) = delta.tool_calls {
        for tool_call in tool_calls {
            let mut tool_call_part = ToolCallPartDelta::default();
            if let Some(id) = tool_call.id {
                tool_call_part.tool_call_id = Some(id);
            }
            if let Some(function) = tool_call.function {
                if let Some(name) = function.name {
                    tool_call_part.tool_name = Some(name);
                }
                if let Some(args) = function.arguments {
                    tool_call_part.args = Some(args);
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
