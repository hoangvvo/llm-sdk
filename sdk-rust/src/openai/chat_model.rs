use super::chat_api::{
    self, AssistantAudioData, AssistantAudioDataInner, AssistantMessageContent,
    AssistantMessageContentInner, ChatCompletionAudioParams, ChatCompletionMessageToolCall,
    ChatCompletionMessageToolCallUnion, ChatCompletionNamedToolChoice,
    ChatCompletionRequestAssistantMessage, ChatCompletionRequestMessage,
    ChatCompletionRequestMessageContentPartAudio, ChatCompletionRequestMessageContentPartImage,
    ChatCompletionRequestMessageContentPartText, ChatCompletionRequestSystemMessage,
    ChatCompletionRequestToolMessage, ChatCompletionRequestToolMessageContentPart,
    ChatCompletionRequestUserMessage, ChatCompletionRequestUserMessageContentPart,
    ChatCompletionStreamOptions, ChatCompletionStreamOptionsInner,
    ChatCompletionStreamResponseDelta, ChatCompletionTool, ChatCompletionToolChoiceOption,
    ChatCompletionToolUnion, CompletionUsage, CreateChatCompletionRequest,
    CreateChatCompletionResponse, CreateChatCompletionStreamResponse,
    CreateModelResponseProperties, FunctionObject, JsonSchemaConfig, Metadata, ModelIdsShared,
    ModelResponseProperties, NamedToolFunction, ReasoningEffort, ReasoningEffortEnum,
    ResponseFormat, ResponseFormatJsonObject, ResponseFormatJsonSchema,
    ResponseFormatJsonSchemaSchema, ResponseFormatText, ResponseModalities, ResponseModalityEnum,
    ToolCallFunction, ToolChoiceString, ToolMessageContent, VoiceIdsShared,
};
use crate::{
    client_utils, source_part_utils, stream_utils, AssistantMessage, AudioFormat, AudioOptions,
    ContentDelta, LanguageModel, LanguageModelError, LanguageModelInput, LanguageModelMetadata,
    LanguageModelResult, LanguageModelStream, Message, ModelResponse, ModelUsage, Part, PartDelta,
    PartialModelResponse, ResponseFormatJson, ResponseFormatOption, Tool, ToolCallPart,
    ToolChoiceOption, ToolChoiceTool, ToolMessage, UserMessage,
};
use async_stream::try_stream;
use futures::{future::BoxFuture, StreamExt};
use reqwest::{
    header::{self, HeaderMap, HeaderName, HeaderValue},
    Client,
};
use serde_json::Value;
use std::{collections::HashMap, sync::Arc};

const PROVIDER: &str = "openai";
const OPENAI_AUDIO_SAMPLE_RATE: u32 = 24_000;
const OPENAI_AUDIO_CHANNELS: u32 = 1;

pub struct OpenAIChatModel {
    model_id: String,
    api_key: String,
    base_url: String,
    client: Client,
    metadata: Option<Arc<LanguageModelMetadata>>,
    headers: HashMap<String, String>,
}

#[derive(Clone, Default)]
pub struct OpenAIChatModelOptions {
    pub base_url: Option<String>,
    pub api_key: String,
    pub headers: Option<HashMap<String, String>>,
    pub client: Option<Client>,
}

impl OpenAIChatModel {
    #[must_use]
    pub fn new(model_id: impl Into<String>, options: OpenAIChatModelOptions) -> Self {
        let OpenAIChatModelOptions {
            base_url,
            api_key,
            headers,
            client,
        } = options;

        let base_url = base_url
            .unwrap_or_else(|| "https://api.openai.com/v1".to_string())
            .trim_end_matches('/')
            .to_string();
        let client = client.unwrap_or_else(Client::new);
        let headers = headers.unwrap_or_default();

        Self {
            model_id: model_id.into(),
            api_key,
            base_url,
            client,
            metadata: None,
            headers,
        }
    }

    #[must_use]
    pub fn with_metadata(mut self, metadata: LanguageModelMetadata) -> Self {
        self.metadata = Some(Arc::new(metadata));
        self
    }

    fn request_headers(&self) -> LanguageModelResult<HeaderMap> {
        let mut headers = HeaderMap::new();

        let auth_header =
            HeaderValue::from_str(&format!("Bearer {}", self.api_key)).map_err(|error| {
                LanguageModelError::InvalidInput(format!(
                    "Invalid OpenAI API key header value: {error}"
                ))
            })?;
        headers.insert(header::AUTHORIZATION, auth_header);

        for (key, value) in &self.headers {
            let header_name = HeaderName::from_bytes(key.as_bytes()).map_err(|error| {
                LanguageModelError::InvalidInput(format!(
                    "Invalid OpenAI header name '{key}': {error}"
                ))
            })?;
            let header_value = HeaderValue::from_str(value).map_err(|error| {
                LanguageModelError::InvalidInput(format!(
                    "Invalid OpenAI header value for '{key}': {error}"
                ))
            })?;
            headers.insert(header_name, header_value);
        }

        Ok(headers)
    }
}

impl LanguageModel for OpenAIChatModel {
    fn provider(&self) -> &'static str {
        PROVIDER
    }

    fn model_id(&self) -> String {
        self.model_id.clone()
    }

    fn metadata(&self) -> Option<&LanguageModelMetadata> {
        self.metadata.as_deref()
    }

    fn generate(
        &self,
        input: LanguageModelInput,
    ) -> BoxFuture<'_, LanguageModelResult<ModelResponse>> {
        Box::pin(async move {
            crate::opentelemetry::trace_generate(
                self.provider(),
                &self.model_id(),
                input,
                |input| async move {
                    let (request, payload) =
                        convert_to_openai_create_params(&input, &self.model_id(), false)?;
                    let headers = self.request_headers()?;

                    let response: CreateChatCompletionResponse = client_utils::send_json(
                        &self.client,
                        &format!("{}/chat/completions", self.base_url),
                        &payload,
                        headers,
                    )
                    .await?;

                    let choice = response.choices.into_iter().next().ok_or_else(|| {
                        LanguageModelError::Invariant(
                            PROVIDER,
                            "No choices in response".to_string(),
                        )
                    })?;

                    if let Some(ref refusal) = choice.message.refusal {
                        if !refusal.is_empty() {
                            return Err(LanguageModelError::Refusal(refusal.clone()));
                        }
                    }

                    let content = map_openai_message(&choice.message, request.audio.as_ref())?;

                    let usage = response
                        .usage
                        .map(|usage| map_openai_usage(usage, &input))
                        .transpose()?;

                    let cost = if let (Some(usage), Some(pricing)) = (
                        usage.as_ref(),
                        self.metadata().and_then(|m| m.pricing.as_ref()),
                    ) {
                        Some(usage.calculate_cost(pricing))
                    } else {
                        None
                    };

                    Ok(ModelResponse {
                        content,
                        usage,
                        cost,
                    })
                },
            )
            .await
        })
    }

    fn stream(
        &self,
        input: LanguageModelInput,
    ) -> BoxFuture<'_, LanguageModelResult<LanguageModelStream>> {
        Box::pin(async move {
            crate::opentelemetry::trace_stream(
                self.provider(),
                &self.model_id(),
                input,
                |input| async move {
                    let metadata = self.metadata.clone();
                    let (request, payload) =
                        convert_to_openai_create_params(&input, &self.model_id(), true)?;
                    let headers = self.request_headers()?;

                    let mut stream =
                        client_utils::send_sse_stream::<Value, CreateChatCompletionStreamResponse>(
                            &self.client,
                            &format!("{}/chat/completions", self.base_url),
                            &payload,
                            headers,
                            PROVIDER,
                        )
                        .await?;

                    let mut refusal = String::new();
                    let mut content_deltas: Vec<ContentDelta> = Vec::new();
                    let audio_params = request.audio.clone();

                    let stream = try_stream! {
                        while let Some(chunk) = stream.next().await {
                            let chunk = chunk?;

                            if let Some(choice) = chunk.choices.unwrap_or_default().into_iter().next() {
                                if let Some(delta_refusal) = choice.delta.refusal.clone() {
                                    refusal.push_str(&delta_refusal);
                                }

                                let deltas = map_openai_delta(
                                    choice.delta,
                                    &content_deltas,
                                    audio_params.as_ref(),
                                )?;

                                for delta in deltas {
                                    content_deltas.push(delta.clone());
                                    yield PartialModelResponse {
                                        delta: Some(delta),
                                        ..Default::default()
                                    };
                                }
                            }

                            if let Some(usage) = chunk.usage {
                                let usage = map_openai_usage(usage, &input)?;
                                let cost = metadata
                                    .as_ref()
                                    .and_then(|m| m.pricing.as_ref())
                                    .map(|pricing| usage.calculate_cost(pricing));

                                yield PartialModelResponse {
                                    delta: None,
                                    usage: Some(usage),
                                    cost,
                                };
                            }
                        }

                        if !refusal.is_empty() {
                            Err(LanguageModelError::Refusal(refusal))?;
                        }
                    };

                    Ok(LanguageModelStream::from_stream(stream))
                },
            )
            .await
        })
    }
}

fn convert_to_openai_create_params(
    input: &LanguageModelInput,
    model_id: &str,
    stream: bool,
) -> LanguageModelResult<(CreateChatCompletionRequest, Value)> {
    let messages = convert_to_openai_messages(input.messages.clone(), input.system_prompt.clone())?;

    let modalities = input
        .modalities
        .as_ref()
        .map(|modalities| -> LanguageModelResult<ResponseModalities> {
            if modalities.is_empty() {
                Ok(ResponseModalities::Null)
            } else {
                let converted = modalities
                    .iter()
                    .map(convert_to_openai_modality)
                    .collect::<LanguageModelResult<Vec<_>>>()?;
                Ok(ResponseModalities::Array(converted))
            }
        })
        .transpose()?;

    let create_model_response_properties = CreateModelResponseProperties {
        model_response_properties: ModelResponseProperties {
            metadata: input
                .metadata
                .as_ref()
                .map(|metadata| Metadata::Map(metadata.clone()))
                .or(Some(Metadata::Null))
                .filter(|metadata| !matches!(metadata, Metadata::Null)),
            prompt_cache_key: None,
            safety_identifier: None,
            service_tier: None,
            temperature: input.temperature,
            top_logprobs: None,
            top_p: input.top_p,
            ..Default::default()
        },
        top_logprobs: None,
    };

    let audio = input
        .audio
        .as_ref()
        .map(convert_to_openai_audio)
        .transpose()?;

    let reasoning_effort = input
        .reasoning
        .as_ref()
        .and_then(|reasoning| reasoning.budget_tokens)
        .map(convert_to_openai_reasoning_effort)
        .transpose()?;

    let request = CreateChatCompletionRequest {
        create_model_response_properties,
        audio,
        frequency_penalty: input.frequency_penalty,
        logit_bias: None,
        logprobs: None,
        max_completion_tokens: input
            .max_tokens
            .map(|value| {
                i32::try_from(value).map_err(|_| {
                    LanguageModelError::InvalidInput(
                        "max_tokens exceeds supported range for OpenAI chat completions"
                            .to_string(),
                    )
                })
            })
            .transpose()?,
        messages,
        modalities,
        model: ModelIdsShared::String(model_id.to_string()),
        n: None,
        parallel_tool_calls: None,
        prediction: None,
        presence_penalty: input.presence_penalty,
        reasoning_effort,
        response_format: input
            .response_format
            .as_ref()
            .map(convert_to_openai_response_format)
            .transpose()?,
        seed: input.seed,
        stop: None,
        store: None,
        stream: Some(stream),
        stream_options: if stream {
            Some(ChatCompletionStreamOptions::Options(
                ChatCompletionStreamOptionsInner {
                    include_obfuscation: None,
                    include_usage: Some(true),
                },
            ))
        } else {
            None
        },
        tool_choice: input
            .tool_choice
            .as_ref()
            .map(convert_to_openai_tool_choice)
            .transpose()?,
        tools: input
            .tools
            .as_ref()
            .map(|tools| {
                tools
                    .iter()
                    .map(convert_to_openai_tool)
                    .collect::<LanguageModelResult<Vec<_>>>()
            })
            .transpose()?,
        top_logprobs: None,
        verbosity: None,
        web_search_options: None,
    };

    let payload = merge_extra(&request, &input.extra)?;

    Ok((request, payload))
}

fn convert_to_openai_messages(
    messages: Vec<Message>,
    system_prompt: Option<String>,
) -> LanguageModelResult<Vec<ChatCompletionRequestMessage>> {
    let mut openai_messages = Vec::new();

    if let Some(prompt) = system_prompt {
        openai_messages.push(ChatCompletionRequestMessage::System(
            ChatCompletionRequestSystemMessage {
                content: chat_api::SystemMessageContent::Text(prompt),
                name: None,
            },
        ));
    }

    for message in messages {
        match message {
            Message::User(user_message) => {
                openai_messages.push(ChatCompletionRequestMessage::User(convert_user_message(
                    user_message,
                )?));
            }
            Message::Assistant(assistant_message) => {
                openai_messages.push(ChatCompletionRequestMessage::Assistant(
                    convert_assistant_message(assistant_message)?,
                ));
            }
            Message::Tool(tool_message) => {
                let tool_messages = convert_tool_message(tool_message)?;
                openai_messages.extend(
                    tool_messages
                        .into_iter()
                        .map(ChatCompletionRequestMessage::Tool),
                );
            }
        }
    }

    Ok(openai_messages)
}

fn convert_user_message(
    user_message: UserMessage,
) -> LanguageModelResult<ChatCompletionRequestUserMessage> {
    let parts = source_part_utils::get_compatible_parts_without_source_parts(user_message.content);
    let mut content_parts = Vec::new();

    for part in parts {
        match part {
            Part::Text(text_part) => {
                content_parts.push(ChatCompletionRequestUserMessageContentPart::Text(
                    ChatCompletionRequestMessageContentPartText {
                        text: text_part.text,
                        type_field: "text".to_string(),
                    },
                ));
            }
            Part::Image(image_part) => {
                content_parts.push(ChatCompletionRequestUserMessageContentPart::Image(
                    ChatCompletionRequestMessageContentPartImage {
                        image_url: chat_api::ImageUrl {
                            detail: None,
                            url: format!(
                                "data:{};base64,{}",
                                image_part.mime_type, image_part.image_data
                            ),
                        },
                    },
                ));
            }
            Part::Audio(audio_part) => {
                let format = match audio_part.format {
                    AudioFormat::Mp3 => chat_api::InputAudioFormat::Mp3,
                    AudioFormat::Wav => chat_api::InputAudioFormat::Wav,
                    _ => {
                        return Err(LanguageModelError::Unsupported(
                            PROVIDER,
                            format!(
                                "Cannot convert audio format '{:?}' to OpenAI input audio format",
                                audio_part.format
                            ),
                        ))
                    }
                };
                content_parts.push(ChatCompletionRequestUserMessageContentPart::Audio(
                    ChatCompletionRequestMessageContentPartAudio {
                        input_audio: chat_api::InputAudio {
                            data: audio_part.audio_data,
                            format,
                        },
                    },
                ));
            }
            unsupported => {
                return Err(LanguageModelError::Unsupported(
                    PROVIDER,
                    format!(
                        "Cannot convert part to OpenAI user message for type {:?}",
                        unsupported
                    ),
                ));
            }
        }
    }

    if content_parts.is_empty() {
        return Err(LanguageModelError::InvalidInput(
            "User message content must not be empty".to_string(),
        ));
    }

    Ok(ChatCompletionRequestUserMessage {
        content: chat_api::UserMessageContent::Array(content_parts),
        name: None,
    })
}

fn convert_assistant_message(
    assistant_message: AssistantMessage,
) -> LanguageModelResult<ChatCompletionRequestAssistantMessage> {
    let parts =
        source_part_utils::get_compatible_parts_without_source_parts(assistant_message.content);

    let mut content_parts: Vec<chat_api::ChatCompletionRequestAssistantMessageContentPart> =
        Vec::new();
    let mut tool_calls: Vec<ChatCompletionMessageToolCallUnion> = Vec::new();
    let mut audio: Option<AssistantAudioData> = None;

    for part in parts {
        match part {
            Part::Text(text_part) => {
                content_parts.push(
                    chat_api::ChatCompletionRequestAssistantMessageContentPart::Text(
                        ChatCompletionRequestMessageContentPartText {
                            text: text_part.text,
                            type_field: "text".to_string(),
                        },
                    ),
                );
            }
            Part::ToolCall(tool_call_part) => {
                tool_calls.push(ChatCompletionMessageToolCallUnion::Function(
                    convert_to_openai_tool_call(&tool_call_part)?,
                ));
            }
            Part::Audio(audio_part) => {
                let id = audio_part.id.ok_or_else(|| {
                    LanguageModelError::Unsupported(
                        PROVIDER,
                        "Cannot convert audio part to OpenAI assistant message without an ID"
                            .to_string(),
                    )
                })?;
                audio = Some(AssistantAudioData::Audio(AssistantAudioDataInner { id }));
            }
            unsupported => {
                return Err(LanguageModelError::Unsupported(
                    PROVIDER,
                    format!(
                        "Cannot convert part to OpenAI assistant message for type {:?}",
                        unsupported
                    ),
                ));
            }
        }
    }

    let content = if content_parts.is_empty() {
        None
    } else {
        Some(AssistantMessageContent::Content(
            AssistantMessageContentInner::Array(content_parts),
        ))
    };

    Ok(ChatCompletionRequestAssistantMessage {
        audio,
        content,
        refusal: None,
        tool_calls: if tool_calls.is_empty() {
            None
        } else {
            Some(tool_calls)
        },
    })
}

fn convert_tool_message(
    tool_message: ToolMessage,
) -> LanguageModelResult<Vec<ChatCompletionRequestToolMessage>> {
    let mut result = Vec::new();

    for part in tool_message.content {
        match part {
            Part::ToolResult(tool_result_part) => {
                let mut content_parts = Vec::new();
                let converted_parts = source_part_utils::get_compatible_parts_without_source_parts(
                    tool_result_part.content,
                );
                for content_part in converted_parts {
                    match content_part {
                        Part::Text(text_part) => {
                            content_parts.push(ChatCompletionRequestToolMessageContentPart::Text(
                                ChatCompletionRequestMessageContentPartText {
                                    text: text_part.text,
                                    type_field: "text".to_string(),
                                },
                            ))
                        }
                        unsupported => {
                            return Err(LanguageModelError::Unsupported(
                                PROVIDER,
                                format!(
                                    "Tool messages must contain only text parts, found {:?}",
                                    unsupported
                                ),
                            ));
                        }
                    }
                }

                result.push(ChatCompletionRequestToolMessage {
                    content: ToolMessageContent::Array(content_parts),
                    tool_call_id: tool_result_part.tool_call_id,
                });
            }
            unsupported => {
                return Err(LanguageModelError::InvalidInput(format!(
                    "Tool messages must contain only tool result parts, found {:?}",
                    unsupported
                )));
            }
        }
    }

    Ok(result)
}

fn convert_to_openai_tool(tool: &Tool) -> LanguageModelResult<ChatCompletionToolUnion> {
    let function = FunctionObject {
        description: Some(tool.description.clone()),
        name: tool.name.clone(),
        parameters: Some(tool.parameters.clone()),
        strict: Some(true),
    };
    Ok(ChatCompletionToolUnion::Function(ChatCompletionTool {
        function,
        type_field: "function".to_string(),
    }))
}

fn convert_to_openai_tool_call(
    part: &ToolCallPart,
) -> LanguageModelResult<ChatCompletionMessageToolCall> {
    let arguments = serde_json::to_string(&part.args).map_err(|error| {
        LanguageModelError::InvalidInput(format!(
            "Failed to serialize tool call arguments: {error}"
        ))
    })?;

    Ok(ChatCompletionMessageToolCall {
        function: ToolCallFunction {
            arguments,
            name: part.tool_name.clone(),
        },
        id: part.id.clone().unwrap_or_else(|| part.tool_call_id.clone()),
        type_field: "function".to_string(),
    })
}

fn convert_to_openai_tool_choice(
    tool_choice: &ToolChoiceOption,
) -> LanguageModelResult<ChatCompletionToolChoiceOption> {
    Ok(match tool_choice {
        ToolChoiceOption::Auto => ChatCompletionToolChoiceOption::String(ToolChoiceString::Auto),
        ToolChoiceOption::None => ChatCompletionToolChoiceOption::String(ToolChoiceString::None),
        ToolChoiceOption::Required => {
            ChatCompletionToolChoiceOption::String(ToolChoiceString::Required)
        }
        ToolChoiceOption::Tool(ToolChoiceTool { tool_name }) => {
            ChatCompletionToolChoiceOption::NamedTool(ChatCompletionNamedToolChoice {
                function: NamedToolFunction {
                    name: tool_name.clone(),
                },
                type_field: "function".to_string(),
            })
        }
    })
}

fn convert_to_openai_response_format(
    response_format: &ResponseFormatOption,
) -> LanguageModelResult<ResponseFormat> {
    Ok(match response_format {
        ResponseFormatOption::Text => ResponseFormat::Text(ResponseFormatText {
            type_field: "text".to_string(),
        }),
        ResponseFormatOption::Json(ResponseFormatJson {
            name,
            description,
            schema,
        }) => {
            if let Some(schema) = schema {
                ResponseFormat::JsonSchema(ResponseFormatJsonSchema {
                    json_schema: JsonSchemaConfig {
                        description: description.clone(),
                        name: name.clone(),
                        schema: Some(ResponseFormatJsonSchemaSchema::from(schema.clone())),
                        strict: Some(true),
                    },
                    type_field: "json_schema".to_string(),
                })
            } else {
                ResponseFormat::JsonObject(ResponseFormatJsonObject {
                    type_field: "json_object".to_string(),
                })
            }
        }
    })
}

fn convert_to_openai_modality(
    modality: &crate::Modality,
) -> LanguageModelResult<ResponseModalityEnum> {
    Ok(match modality {
        crate::Modality::Text => ResponseModalityEnum::Text,
        crate::Modality::Audio => ResponseModalityEnum::Audio,
        other => {
            return Err(LanguageModelError::Unsupported(
                PROVIDER,
                format!(
                    "Cannot convert modality to OpenAI modality for modality {:?}",
                    other
                ),
            ))
        }
    })
}

fn convert_to_openai_audio(audio: &AudioOptions) -> LanguageModelResult<ChatCompletionAudioParams> {
    let voice = audio.voice.clone().ok_or_else(|| {
        LanguageModelError::InvalidInput("Audio voice is required for OpenAI audio".to_string())
    })?;

    let format = match audio.format.clone() {
        Some(AudioFormat::Wav) => chat_api::AudioFormat::Wav,
        Some(AudioFormat::Mp3) => chat_api::AudioFormat::Mp3,
        Some(AudioFormat::Flac) => chat_api::AudioFormat::Flac,
        Some(AudioFormat::Aac) => chat_api::AudioFormat::Aac,
        Some(AudioFormat::Opus) => chat_api::AudioFormat::Opus,
        Some(AudioFormat::Linear16) => chat_api::AudioFormat::Pcm16,
        None => {
            return Err(LanguageModelError::InvalidInput(
                "Audio format is required for OpenAI audio".to_string(),
            ))
        }
        Some(other) => {
            return Err(LanguageModelError::Unsupported(
                PROVIDER,
                format!(
                    "Cannot convert audio format '{:?}' to OpenAI audio format",
                    other
                ),
            ))
        }
    };

    Ok(ChatCompletionAudioParams {
        format,
        voice: VoiceIdsShared::String(voice),
    })
}

fn convert_to_openai_reasoning_effort(budget_tokens: u32) -> LanguageModelResult<ReasoningEffort> {
    let effort = match budget_tokens {
        crate::openai::types::OPENAI_REASONING_EFFORT_MINIMAL => ReasoningEffortEnum::Minimal,
        crate::openai::types::OPENAI_REASONING_EFFORT_LOW => ReasoningEffortEnum::Low,
        crate::openai::types::OPENAI_REASONING_EFFORT_MEDIUM => ReasoningEffortEnum::Medium,
        crate::openai::types::OPENAI_REASONING_EFFORT_HIGH => ReasoningEffortEnum::High,
        _ => {
            return Err(LanguageModelError::Unsupported(
                PROVIDER,
                "Budget tokens property is not supported for OpenAI reasoning. You may use OPENAI_REASONING_EFFORT_* constants to map it to OpenAI reasoning effort levels.".to_string(),
            ))
        }
    };

    Ok(ReasoningEffort::Enum(effort))
}

fn merge_extra(
    request: &CreateChatCompletionRequest,
    extra: &Option<Value>,
) -> LanguageModelResult<Value> {
    let mut payload = serde_json::to_value(request).map_err(|error| {
        LanguageModelError::InvalidInput(format!("Failed to serialize OpenAI request: {error}"))
    })?;

    if let Some(extra) = extra {
        if let Value::Object(extra_map) = extra {
            let map = payload.as_object_mut().ok_or_else(|| {
                LanguageModelError::InvalidInput(
                    "Serialized OpenAI request is not an object".to_string(),
                )
            })?;
            for (key, value) in extra_map {
                map.insert(key.clone(), value.clone());
            }
        } else if !extra.is_null() {
            return Err(LanguageModelError::InvalidInput(
                "OpenAI extra must be a JSON object".to_string(),
            ));
        }
    }

    Ok(payload)
}

fn map_openai_message(
    message: &chat_api::ChatCompletionResponseMessage,
    audio_params: Option<&ChatCompletionAudioParams>,
) -> LanguageModelResult<Vec<Part>> {
    let mut parts = Vec::new();

    if let Some(content) = &message.content {
        if !content.is_empty() {
            parts.push(Part::Text(crate::TextPart {
                text: content.clone(),
                citations: None,
            }));
        }
    }

    if let Some(audio) = &message.audio {
        if let chat_api::AudioResponseData::Audio(audio_data) = audio {
            let audio_format = audio_params
                .map(|params| map_openai_audio_format(params.format.clone()))
                .ok_or_else(|| {
                    LanguageModelError::Invariant(
                        PROVIDER,
                        "Audio returned from OpenAI API but no audio parameter was provided"
                            .to_string(),
                    )
                })?;

            let mut audio_part = crate::AudioPart {
                audio_data: audio_data.data.clone(),
                format: audio_format,
                sample_rate: None,
                channels: None,
                transcript: Some(audio_data.transcript.clone()),
                id: Some(audio_data.id.clone()),
            };

            if audio_part.format == AudioFormat::Linear16 {
                audio_part.sample_rate = Some(OPENAI_AUDIO_SAMPLE_RATE);
                audio_part.channels = Some(OPENAI_AUDIO_CHANNELS);
            }

            parts.push(Part::Audio(audio_part));
        }
    }

    if let Some(tool_calls) = &message.tool_calls {
        for tool_call in tool_calls {
            match tool_call {
                ChatCompletionMessageToolCallUnion::Function(function_tool_call) => {
                    parts.push(Part::ToolCall(map_openai_function_tool_call(
                        function_tool_call,
                    )?));
                }
                ChatCompletionMessageToolCallUnion::Custom(custom_tool_call) => {
                    return Err(LanguageModelError::NotImplemented(
                        PROVIDER,
                        format!(
                            "Cannot map OpenAI tool call of type {} to ToolCallPart",
                            custom_tool_call.type_field
                        ),
                    ));
                }
            }
        }
    }

    Ok(parts)
}

fn map_openai_audio_format(format: chat_api::AudioFormat) -> AudioFormat {
    match format {
        chat_api::AudioFormat::Wav => AudioFormat::Wav,
        chat_api::AudioFormat::Mp3 => AudioFormat::Mp3,
        chat_api::AudioFormat::Flac => AudioFormat::Flac,
        chat_api::AudioFormat::Opus => AudioFormat::Opus,
        chat_api::AudioFormat::Pcm16 => AudioFormat::Linear16,
        chat_api::AudioFormat::Aac => AudioFormat::Aac,
    }
}

fn map_openai_function_tool_call(
    tool_call: &ChatCompletionMessageToolCall,
) -> LanguageModelResult<ToolCallPart> {
    if tool_call.type_field != "function" {
        return Err(LanguageModelError::NotImplemented(
            PROVIDER,
            format!(
                "Cannot map OpenAI tool call of type {} to ToolCallPart",
                tool_call.type_field
            ),
        ));
    }

    let args: Value = serde_json::from_str(&tool_call.function.arguments).map_err(|error| {
        LanguageModelError::Invariant(
            PROVIDER,
            format!("Failed to parse tool call arguments as JSON: {error}"),
        )
    })?;

    Ok(ToolCallPart {
        tool_call_id: tool_call.id.clone(),
        tool_name: tool_call.function.name.clone(),
        args,
        id: Some(tool_call.id.clone()),
    })
}

fn map_openai_delta(
    delta: ChatCompletionStreamResponseDelta,
    existing_content_deltas: &[ContentDelta],
    audio_params: Option<&ChatCompletionAudioParams>,
) -> LanguageModelResult<Vec<ContentDelta>> {
    let mut content_deltas = Vec::new();

    if let Some(content) = delta.content {
        if !content.is_empty() {
            let part = PartDelta::Text(crate::TextPartDelta {
                text: content,
                citation: None,
            });
            let mut combined = existing_content_deltas.to_vec();
            combined.extend(content_deltas.iter().cloned());
            let index = stream_utils::guess_delta_index(&part, &combined, None);
            content_deltas.push(ContentDelta { index, part });
        }
    }

    if let Some(audio) = delta.audio {
        let mut audio_part = crate::AudioPartDelta {
            audio_data: audio.data,
            format: audio_params.map(|params| map_openai_audio_format(params.format.clone())),
            sample_rate: None,
            channels: None,
            transcript: audio.transcript,
            id: audio.id,
        };

        if audio_part.format.as_ref() == Some(&AudioFormat::Linear16) {
            audio_part.sample_rate = Some(OPENAI_AUDIO_SAMPLE_RATE);
            audio_part.channels = Some(OPENAI_AUDIO_CHANNELS);
        }

        let part = PartDelta::Audio(audio_part);
        let mut combined = existing_content_deltas.to_vec();
        combined.extend(content_deltas.iter().cloned());
        let index = stream_utils::guess_delta_index(&part, &combined, None);
        content_deltas.push(ContentDelta { index, part });
    }

    if let Some(tool_calls) = delta.tool_calls {
        for tool_call in tool_calls {
            let mut part = crate::ToolCallPartDelta {
                tool_call_id: tool_call.id.clone(),
                tool_name: tool_call
                    .function
                    .as_ref()
                    .and_then(|function| function.name.clone()),
                args: tool_call
                    .function
                    .as_ref()
                    .and_then(|function| function.arguments.clone()),
                id: None,
            };

            if let Some(function) = tool_call.function.as_ref() {
                if part.tool_name.is_none() {
                    part.tool_name = function.name.clone();
                }
                if part.args.is_none() {
                    part.args = function.arguments.clone();
                }
            }

            let mut combined = existing_content_deltas.to_vec();
            combined.extend(content_deltas.iter().cloned());
            let index = stream_utils::guess_delta_index(
                &PartDelta::ToolCall(part.clone()),
                &combined,
                Some(usize::try_from(tool_call.index).map_err(|_| {
                    LanguageModelError::Invariant(
                        PROVIDER,
                        "Received negative tool call index from OpenAI stream".to_string(),
                    )
                })?),
            );
            content_deltas.push(ContentDelta {
                index,
                part: PartDelta::ToolCall(part),
            });
        }
    }

    Ok(content_deltas)
}

fn map_openai_usage(
    usage: CompletionUsage,
    input: &LanguageModelInput,
) -> LanguageModelResult<ModelUsage> {
    let input_tokens = u32::try_from(usage.prompt_tokens).map_err(|_| {
        LanguageModelError::Invariant(
            PROVIDER,
            "OpenAI prompt_tokens exceeded u32 range".to_string(),
        )
    })?;
    let output_tokens = u32::try_from(usage.completion_tokens).map_err(|_| {
        LanguageModelError::Invariant(
            PROVIDER,
            "OpenAI completion_tokens exceeded u32 range".to_string(),
        )
    })?;

    let mut result = ModelUsage {
        input_tokens,
        output_tokens,
        input_tokens_details: None,
        output_tokens_details: None,
    };

    if let Some(details) = usage.prompt_tokens_details {
        result.input_tokens_details = Some(map_openai_prompt_tokens_details(details, input)?);
    }

    if let Some(details) = usage.completion_tokens_details {
        result.output_tokens_details = Some(map_openai_completion_tokens_details(details)?);
    }

    Ok(result)
}

fn map_openai_prompt_tokens_details(
    details: chat_api::PromptTokensDetails,
    input: &LanguageModelInput,
) -> LanguageModelResult<crate::ModelTokensDetails> {
    let mut result = crate::ModelTokensDetails::default();

    if let Some(text_tokens) = details.text_tokens {
        result.text_tokens = Some(u32::try_from(text_tokens).map_err(|_| {
            LanguageModelError::Invariant(
                PROVIDER,
                "OpenAI text prompt tokens exceeded u32 range".to_string(),
            )
        })?);
    }

    if let Some(audio_tokens) = details.audio_tokens {
        result.audio_tokens = Some(u32::try_from(audio_tokens).map_err(|_| {
            LanguageModelError::Invariant(
                PROVIDER,
                "OpenAI audio prompt tokens exceeded u32 range".to_string(),
            )
        })?);
    }

    if let Some(image_tokens) = details.image_tokens {
        result.image_tokens = Some(u32::try_from(image_tokens).map_err(|_| {
            LanguageModelError::Invariant(
                PROVIDER,
                "OpenAI image prompt tokens exceeded u32 range".to_string(),
            )
        })?);
    }

    let has_text_part = input.messages.iter().any(|message| match message {
        Message::User(user_message) => user_message
            .content
            .iter()
            .any(|part| matches!(part, Part::Text(_))),
        _ => false,
    });
    let has_audio_part = input.messages.iter().any(|message| match message {
        Message::User(user_message) => user_message
            .content
            .iter()
            .any(|part| matches!(part, Part::Audio(_))),
        _ => false,
    });

    if let Some(cached_details) = details.cached_tokens_details {
        if let Some(text_tokens) = cached_details.text_tokens {
            result.cached_text_tokens = Some(u32::try_from(text_tokens).map_err(|_| {
                LanguageModelError::Invariant(
                    PROVIDER,
                    "OpenAI cached text prompt tokens exceeded u32 range".to_string(),
                )
            })?);
        }
        if let Some(audio_tokens) = cached_details.audio_tokens {
            result.cached_audio_tokens = Some(u32::try_from(audio_tokens).map_err(|_| {
                LanguageModelError::Invariant(
                    PROVIDER,
                    "OpenAI cached audio prompt tokens exceeded u32 range".to_string(),
                )
            })?);
        }
    } else if let Some(cached_tokens) = details.cached_tokens {
        let cached_tokens = u32::try_from(cached_tokens).map_err(|_| {
            LanguageModelError::Invariant(
                PROVIDER,
                "OpenAI cached prompt tokens exceeded u32 range".to_string(),
            )
        })?;
        if has_text_part {
            result.cached_text_tokens = Some(cached_tokens);
        }
        if has_audio_part {
            result.cached_audio_tokens = Some(cached_tokens);
        }
    }

    Ok(result)
}

fn map_openai_completion_tokens_details(
    details: chat_api::CompletionTokensDetails,
) -> LanguageModelResult<crate::ModelTokensDetails> {
    let mut result = crate::ModelTokensDetails::default();

    if let Some(text_tokens) = details.text_tokens {
        result.text_tokens = Some(u32::try_from(text_tokens).map_err(|_| {
            LanguageModelError::Invariant(
                PROVIDER,
                "OpenAI text completion tokens exceeded u32 range".to_string(),
            )
        })?);
    }

    if let Some(audio_tokens) = details.audio_tokens {
        result.audio_tokens = Some(u32::try_from(audio_tokens).map_err(|_| {
            LanguageModelError::Invariant(
                PROVIDER,
                "OpenAI audio completion tokens exceeded u32 range".to_string(),
            )
        })?);
    }

    Ok(result)
}
