use crate::{
    language_model::{LanguageModelMetadata, LanguageModelStream},
    openai::api as openai_api,
    stream_utils,
    usage_utils::calculate_cost,
    AssistantMessage, AudioFormat, AudioPart, AudioPartDelta, ContentDelta, ContentDeltaPart,
    ImagePart, LanguageModel, LanguageModelError, LanguageModelInput, LanguageModelResult, Message,
    Modality, ModelResponse, ModelUsage, Part, PartialModelResponse, ResponseFormatJson,
    ResponseFormatOption, TextPart, TextPartDelta, Tool, ToolCallPart, ToolCallPartDelta,
    ToolChoiceOption, ToolMessage, UserMessage,
};
use async_stream::try_stream;
use eventsource_stream::{self, Eventsource};
use futures::stream::StreamExt;
use reqwest::Client;

const OPENAI_AUDIO_SAMPLE_RATE: u32 = 24_000;
const OPENAI_AUDIO_CHANNELS: u32 = 1;

pub struct OpenAIModel {
    model_id: String,
    api_key: String,
    base_url: String,
    client: Client,
    metadata: Option<LanguageModelMetadata>,
}

impl OpenAIModel {
    #[must_use]
    pub fn new(model_id: String, api_key: String) -> Self {
        let client = Client::new();
        Self {
            model_id,
            api_key,
            base_url: "https://api.openai.com/v1".to_string(),
            client,
            metadata: None,
        }
    }

    #[must_use]
    pub fn with_base_url(mut self, base_url: String) -> Self {
        self.base_url = base_url;
        self
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
        "openai"
    }

    fn model_id(&self) -> String {
        self.model_id.clone()
    }

    async fn generate(&self, input: LanguageModelInput) -> LanguageModelResult<ModelResponse> {
        let params = into_openai_params(&input, self.model_id.clone())?;

        let response = self
            .client
            .post(format!("{}/chat/completions", self.base_url))
            .header("Authorization", format!("Bearer {}", self.api_key))
            .json(&params)
            .send()
            .await?;

        if response.status().is_client_error() {
            return Err(LanguageModelError::StatusCode(response.status()));
        }

        let json = response.json::<openai_api::ChatCompletion>().await?;

        let choice = json
            .choices
            .first()
            .ok_or_else(|| LanguageModelError::Invariant("No choices in response".to_string()))?;

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
        let mut params = into_openai_params(&input, self.model_id.clone())?;
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
                                format!("Failed to parse chunk: {e}")
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
                                yield PartialModelResponse { delta: delta.clone() }
                            }
                        }
                    }
                    Err(e) => {
                        match e {
                            eventsource_stream::EventStreamError::Utf8(_) => {
                                return Err(LanguageModelError::Invariant(
                                    "Receive invalid UTF-8 sequence".to_string()
                                ))?;
                            }
                            eventsource_stream::EventStreamError::Parser(error) => {
                                return Err(LanguageModelError::Invariant(
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

fn into_openai_params(
    input: &LanguageModelInput,
    model_id: String,
) -> LanguageModelResult<openai_api::ChatCompletionCreateParams> {
    Ok(openai_api::ChatCompletionCreateParams {
        model: model_id,
        messages: into_openai_messages(input)?,
        max_completion_tokens: input.max_tokens,
        temperature: input.temperature,
        top_p: input.top_p,
        presence_penalty: input.presence_penalty,
        frequency_penalty: input.frequency_penalty,
        seed: input.seed,
        tools: input
            .tools
            .as_ref()
            .map(|tools| tools.iter().map(Into::into).collect()),
        tool_choice: input.tool_choice.as_ref().map(Into::into),
        response_format: input.response_format.as_ref().map(Into::into),
        modalities: input
            .modalities
            .as_ref()
            .map(|modalities| modalities.iter().map(Into::into).collect()),
        audio: input
            .extra
            .as_ref()
            .and_then(|extra| extra.get("audio"))
            .and_then(|value| {
                serde_json::from_value::<openai_api::ChatCompletionAudioParam>(value.clone()).ok()
            }),
        ..Default::default()
    })
}

fn into_openai_messages(
    input: &LanguageModelInput,
) -> LanguageModelResult<Vec<openai_api::ChatCompletionMessageParam>> {
    let mut openai_messages = vec![];

    if let Some(system_prompt) = &input.system_prompt {
        openai_messages.push(openai_api::ChatCompletionMessageParam::System(
            openai_api::ChatCompletionSystemMessageParam {
                content: vec![openai_api::ChatCompletionContentPartText {
                    text: system_prompt.clone(),
                }],
            },
        ));
    }

    for message in &input.messages {
        match message {
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
                        _ => Err(LanguageModelError::Unsupported(format!(
                            "Unsupported part in assistant message {part:?}"
                        )))?,
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
                        _ => Err(LanguageModelError::InvalidInput(format!(
                            "ToolMessage content must only contain ToolResult parts"
                        )))?,
                    };

                    openai_messages.push(openai_api::ChatCompletionMessageParam::Tool(
                        openai_api::ChatCompletionToolMessageParam {
                            tool_call_id: tool_part.tool_call_id.to_string(),
                            content: tool_part
                                .content
                                .iter()
                                .map(|p| match p {
                                    Part::Text(part) => Ok(part.into()),
                                    _ => Err(LanguageModelError::Unsupported(format!(
                                        "Unsupported part in tool message {p:?}"
                                    ))),
                                })
                                .collect::<LanguageModelResult<_>>()?,
                        },
                    ));
                }
            }

            Message::User(UserMessage { content }) => {
                let openai_message_param = openai_api::ChatCompletionUserMessageParam {
                    content: content
                        .iter()
                        .map(|part| match part {
                            Part::Text(part) => {
                                Ok(openai_api::ChatCompletionContentPart::Text(part.into()))
                            }
                            Part::Image(part) => {
                                Ok(openai_api::ChatCompletionContentPart::Image(part.into()))
                            }
                            Part::Audio(part) => Ok(
                                openai_api::ChatCompletionContentPart::InputAudio(part.try_into()?),
                            ),
                            _ => Err(LanguageModelError::Unsupported(format!(
                                "Unsupported part in user message {part:?}"
                            ))),
                        })
                        .collect::<LanguageModelResult<_>>()?,
                    ..Default::default()
                };
                openai_messages.push(openai_api::ChatCompletionMessageParam::User(
                    openai_message_param,
                ));
            }
        }
    }

    Ok(openai_messages)
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
                    None => Err(LanguageModelError::InvalidInput(
                        "Audio part must have a format".to_string(),
                    )),
                    Some(AudioFormat::Wav) => Ok(openai_api::AudioInputFormat::Wav),
                    Some(AudioFormat::Mp3) => Ok(openai_api::AudioInputFormat::Mp3),
                    _ => Err(LanguageModelError::Unsupported(format!(
                        "Unsupported audio format: {:?}",
                        part.format
                    ))),
                }?,
            },
        })
    }
}

impl TryFrom<&AudioPart> for openai_api::ChatCompletionAssistantMessageParamAudio {
    type Error = LanguageModelError;

    fn try_from(part: &AudioPart) -> Result<Self, Self::Error> {
        let id = part.id.as_ref().ok_or_else(|| {
            LanguageModelError::InvalidInput("Audio part must have an ID".to_string())
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
                arguments: serde_json::from_value(part.args.clone())
                    .map_err(|e| LanguageModelError::InvalidInput(e.to_string()))?,
            },
        })
    }
}

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

impl From<&Modality> for openai_api::Modality {
    fn from(modality: &Modality) -> Self {
        match modality {
            Modality::Text => Self::Text,
            Modality::Audio => Self::Audio,
        }
    }
}

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
            format: create_params
                .audio
                .as_ref()
                .map(|audio_param| &audio_param.format)
                .map(AudioFormat::from),
            audio_data: audio.data.to_string(),
            ..Default::default()
        };
        if matches!(audio_part.format, Some(AudioFormat::Linear16)) {
            audio_part.sample_rate = Some(OPENAI_AUDIO_SAMPLE_RATE);
            audio_part.channels = Some(OPENAI_AUDIO_CHANNELS);
        }
        parts.push(Part::Audio(audio_part));
    }

    Ok(parts)
}

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
        Ok(Self {
            tool_call_id: value.id.to_string(),
            tool_name: value.function.name.to_string(),
            args: serde_json::to_value(&value.function.arguments)
                .map_err(|e| LanguageModelError::InvalidInput(e.to_string()))?,
            id: None,
        })
    }
}

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
        let part = ContentDeltaPart::Text(text_part);
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
        let part = ContentDeltaPart::Audio(audio_part);
        let index = stream_utils::guess_delta_index(
            &part,
            &[existing_content_deltas, content_deltas.as_slice()].concat(),
            None,
        );
        content_deltas.push(ContentDelta { index, part });
    }

    if let Some(tool_calls) = &delta.tool_calls {
        let all_existing_tool_calls = existing_content_deltas
            .iter()
            .filter(|delta| matches!(delta.part, ContentDeltaPart::ToolCall(_)))
            .collect::<Vec<_>>();
        for tool_call in tool_calls {
            let existing_delta = all_existing_tool_calls.get(tool_call.index);

            let mut tool_call_part = ToolCallPartDelta::default();
            if let Some(id) = &tool_call.id {
                tool_call_part.id = Some(id.to_string());
            }
            if let Some(function) = &tool_call.function {
                if let Some(name) = &function.name {
                    tool_call_part.tool_name = Some(name.to_string());
                }
                if let Some(args) = &function.arguments {
                    tool_call_part.args = Some(args.to_string());
                }
            }

            let part = ContentDeltaPart::ToolCall(tool_call_part);
            let index = stream_utils::guess_delta_index(
                &part,
                &[existing_content_deltas, content_deltas.as_slice()].concat(),
                existing_delta.map(|v| &**v),
            );

            content_deltas.push(ContentDelta { index, part });
        }
    }

    content_deltas
}
