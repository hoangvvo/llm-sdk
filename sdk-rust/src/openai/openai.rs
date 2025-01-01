use crate::{
    errors::{LanguageModelError, LanguageModelResult},
    language_model::LanguageModel,
    types::{
        AudioFormat, AudioPart, LanguageModelInput, Message, Modality, ModelResponse, Part,
        ResponseFormatOption, TextPart, Tool, ToolCallPart, ToolChoiceOption,
    },
};
use reqwest::{
    header::{self, HeaderValue},
    Client,
};

use super::openai_api;

const OPENAI_AUDIO_SAMPLE_RATE: i64 = 24_000;
const OPENAI_AUDIO_CHANNELS: i64 = 1;

pub struct OpenAIModel {
    pub model_id: String,
    pub base_url: String,
    pub client: Client,
    pub structured_outputs: bool,
}

pub struct OpenAIModelOptions {
    pub base_url: Option<String>,
    pub model_id: String,
    pub api_key: String,
    pub structured_outputs: bool,
}

impl OpenAIModel {
    pub fn new(options: OpenAIModelOptions) -> OpenAIModel {
        let mut headers = header::HeaderMap::new();
        let mut auth_header_value: HeaderValue =
            format!("Bearer {}", options.api_key).try_into().unwrap();
        auth_header_value.set_sensitive(true);
        headers.insert(header::AUTHORIZATION, auth_header_value);
        headers.insert(
            header::CONTENT_TYPE,
            HeaderValue::from_static("application/json"),
        );

        OpenAIModel {
            model_id: options.model_id,
            base_url: options
                .base_url
                .unwrap_or("https://api.openai.com/v1".to_string()),
            client: Client::builder().default_headers(headers).build().unwrap(),
            structured_outputs: options.structured_outputs,
        }
    }
}

#[async_trait::async_trait]
impl LanguageModel for OpenAIModel {
    fn provider(&self) -> &'static str {
        "openai"
    }

    fn model_id(&self) -> String {
        self.model_id.to_string()
    }

    async fn generate(&self, input: LanguageModelInput) -> LanguageModelResult<ModelResponse> {
        let params =
            convert_to_openai_params(input, self.model_id.clone(), self.structured_outputs)?;

        let response = self
            .client
            .post(format!("{}/chat/completions", self.base_url))
            .json(&params)
            .send()
            .await?;

        if response.status().is_client_error() {
            return Err(LanguageModelError::ClientError(response.text().await?));
        }

        let json = response.json::<openai_api::ChatCompletion>().await?;

        let choice = json.choices.first().ok_or(LanguageModelError::Invariant(
            "no choices in response".to_string(),
        ))?;

        let openai_api::ChatCompletionChoice { message, .. } = choice;

        if let Some(refusal) = &message.refusal {
            return Err(LanguageModelError::Refusal(refusal.to_string()));
        }

        let content = map_openai_message(message, params.audio.map(|audio| audio.format))?;

        Ok(ModelResponse {
            content,
            cost: None,
            usage: None,
        })
    }
}

fn convert_to_openai_params(
    input: LanguageModelInput,
    model_id: String,
    structured_outputs: bool,
) -> LanguageModelResult<openai_api::ChatCompletionCreateParamsBase> {
    Ok(openai_api::ChatCompletionCreateParamsBase {
        model: model_id,
        messages: convert_to_openai_messages(&input)?,
        max_tokens: input.max_tokens,
        temperature: input.temperature,
        top_p: input.top_p,
        presence_penalty: input.presence_penalty,
        frequency_penalty: input.frequency_penalty,
        seed: input.seed,
        tools: input.tools.as_ref().map(|tools| {
            tools
                .iter()
                .map(|tool| convert_to_openai_tool(tool, structured_outputs))
                .collect()
        }),
        tool_choice: input
            .tool_choice
            .as_ref()
            .map(convert_to_openai_tool_choice),
        response_format: input.response_format.as_ref().map(|response_format| {
            convert_to_openai_response_format(response_format, structured_outputs)
        }),
        modalities: input
            .modalities
            .map(|modalities| modalities.iter().map(convert_to_openai_modality).collect()),
        audio: input
            .extra
            .as_ref()
            .and_then(|extra| extra.get("audio"))
            .and_then(|value| {
                serde_json::from_value::<openai_api::ChatCompletionAudioParam>(value.clone()).ok()
            }),
        extra: input.extra,
    })
}

fn convert_to_openai_messages(
    input: &LanguageModelInput,
) -> LanguageModelResult<Vec<openai_api::ChatCompletionMessageParam>> {
    let mut openai_messages = vec![];

    if let Some(system_prompt) = &input.system_prompt {
        openai_messages.push(openai_api::ChatCompletionMessageParam::System(
            openai_api::ChatCompletionSystemMessageParam {
                content: system_prompt.to_string(),
            },
        ));
    }

    for message in &input.messages {
        match message {
            Message::Assistant(message) => {
                let mut openai_message_param = openai_api::ChatCompletionAssistantMessageParam {
                    content: None,
                    audio: None,
                    tool_calls: None,
                };

                for content in &message.content {
                    match content {
                        Part::Text(part) => {
                            openai_message_param
                                .content
                                .get_or_insert_with(Vec::new)
                                .push(openai_api::ChatCompletionContentPartText {
                                    text: part.text.clone(),
                                });
                        }
                        Part::ToolCall(part) => {
                            openai_message_param
                                .tool_calls
                                .get_or_insert_with(Vec::new)
                                .push(openai_api::ChatCompletionMessageToolCall {
                                    id: part.tool_call_id.clone(),
                                    function: openai_api::ChatCompletionMessageToolCallFunction {
                                        name: part.tool_name.clone(),
                                        arguments: part
                                            .args
                                            .as_ref()
                                            .map(|args| args.to_string())
                                            .unwrap_or_default(),
                                    },
                                    type_: "function".to_string(),
                                });
                        }
                        Part::Audio(part) => {
                            if let Some(id) = &part.id {
                                openai_message_param.audio =
                                    Some(openai_api::ChatCompletionAssistantMessageParamAudio {
                                        id: id.to_string(),
                                    });
                            } else {
                                return Err(LanguageModelError::InvalidInput(
                                    "audio part must have an id".to_string(),
                                ));
                            }
                        }
                        _ => Err(LanguageModelError::InvalidInput(
                            "unsupported content part for assistaant message".to_string(),
                        ))?,
                    }
                }

                openai_messages.push(openai_api::ChatCompletionMessageParam::Assistant(
                    openai_message_param,
                ));
            }
            Message::Tool(message) => {
                for content in &message.content {
                    openai_messages.push(openai_api::ChatCompletionMessageParam::Tool(
                        openai_api::ChatCompletionToolMessageParam {
                            tool_call_id: content.tool_call_id.clone(),
                            content: content.result.to_string(),
                        },
                    ));
                }
            }
            Message::User(message) => {
                let mut openai_message_param =
                    openai_api::ChatCompletionUserMessageParam { content: vec![] };

                for content in &message.content {
                    match content {
                        Part::Text(part) => {
                            openai_message_param.content.push(
                                openai_api::ChatCompletionContentPart::Text(
                                    openai_api::ChatCompletionContentPartText {
                                        text: part.text.clone(),
                                    },
                                ),
                            );
                        }
                        Part::Image(part) => {
                            openai_message_param.content.push(
                                openai_api::ChatCompletionContentPart::Image(
                                    openai_api::ChatCompletionContentPartImage {
                                        image_url: openai_api::ChatCompletionContentPartImageUrl {
                                            url: format!(
                                                "data:{};base64,{}",
                                                part.mime_type, part.image_data
                                            ),
                                            detail: None,
                                        },
                                    },
                                ),
                            );
                        }
                        Part::Audio(part) => {
                            openai_message_param.content.push(
                                openai_api::ChatCompletionContentPart::InputAudio(
                                    openai_api::ChatCompletionContentPartInputAudio {
                                        input_audio:
                                            openai_api::ChatCompletionContentPartInputAudioData {
                                                data: part.audio_data.clone(),
                                                format: match part.format {
                                                    None => Err(LanguageModelError::InvalidInput(
                                                        "audio part must have a format".to_string(),
                                                    )),
                                                    Some(AudioFormat::Wav) => Ok(openai_api::ChatCompletionContentPartInputAudioFormat::Wav),
                                                    Some(AudioFormat::Mp3) => Ok(openai_api::ChatCompletionContentPartInputAudioFormat::Mp3),
                                                    _ => Err(LanguageModelError::InvalidInput(
                                                        "unsupported audio format".to_string(),
                                                    )),
                                                }?,
                                            },
                                    },
                                ),
                            );
                        }
                        _ => {
                            return Err(LanguageModelError::InvalidInput(
                                "unsupported content part for user message".to_string(),
                            ))
                        }
                    }
                }

                openai_messages.push(openai_api::ChatCompletionMessageParam::User(
                    openai_message_param,
                ));
            }
        }
    }

    Ok(openai_messages)
}

fn convert_to_openai_tool(tool: &Tool, structured_outputs: bool) -> openai_api::ChatCompletionTool {
    openai_api::ChatCompletionTool {
        function: openai_api::ChatCompletionToolFunction {
            name: tool.name.clone(),
            description: Some(tool.description.clone()),
            parameters: tool.parameters.clone(),
            strict: Some(structured_outputs),
        },
        type_: "function".to_string(),
    }
}

fn convert_to_openai_tool_choice(
    tool_choice: &ToolChoiceOption,
) -> openai_api::ChatCompletionToolChoiceOption {
    match tool_choice {
        ToolChoiceOption::None => openai_api::ChatCompletionToolChoiceOption::None,
        ToolChoiceOption::Auto => openai_api::ChatCompletionToolChoiceOption::Auto,
        ToolChoiceOption::Required => openai_api::ChatCompletionToolChoiceOption::Required,
        ToolChoiceOption::Tool(tool) => openai_api::ChatCompletionToolChoiceOption::Named(
            openai_api::ChatCompletionNamedToolChoice {
                function: openai_api::ChatCompletionNamedToolChoiceFunction {
                    name: tool.tool_name.clone(),
                },
            },
        ),
    }
}

fn convert_to_openai_response_format(
    response_format: &ResponseFormatOption,
    structured_outputs: bool,
) -> openai_api::ResponseFormat {
    match response_format {
        ResponseFormatOption::Text => openai_api::ResponseFormat::Text,
        ResponseFormatOption::Json(format) => {
            if structured_outputs {
                if let Some(schema) = format.schema.as_ref() {
                    return openai_api::ResponseFormat::JSONSchema(
                        openai_api::ResponseFormatJSONSchema {
                            json_schema: openai_api::ResponseFormatJSONSchemaContent {
                                name: format.name.clone(),
                                strict: true,
                                schema: schema.clone(),
                            },
                        },
                    );
                }
            }
            openai_api::ResponseFormat::JSONObject
        }
    }
}

fn convert_to_openai_modality(modality: &Modality) -> openai_api::ChatCompletionModality {
    match modality {
        Modality::Text => openai_api::ChatCompletionModality::Text,
        Modality::Audio => openai_api::ChatCompletionModality::Audio,
    }
}

fn map_openai_message(
    message: &openai_api::ChatCompletionMessage,
    input_audio_format: Option<openai_api::ChatCompletionAudioParamFormat>,
) -> LanguageModelResult<Vec<Part>> {
    let mut parts: Vec<Part> = vec![];

    if let Some(content) = message.content.as_ref() {
        parts.push(Part::Text(TextPart {
            text: content.to_string(),
            id: None,
        }));
    }

    if let Some(completion_audio) = message.audio.as_ref() {
        let input_audio_format = {
            if let Some(format) = input_audio_format {
                format
            } else {
                return Err(LanguageModelError::Invariant(
                    "receive audio without input audio format".to_string(),
                ));
            }
        };

        let (channels, sample_rate) = match input_audio_format {
            openai_api::ChatCompletionAudioParamFormat::Pcm16 => {
                (Some(OPENAI_AUDIO_CHANNELS), Some(OPENAI_AUDIO_SAMPLE_RATE))
            }
            _ => (None, None),
        };

        let audio_part = AudioPart {
            id: Some(completion_audio.id.clone()),
            audio_data: completion_audio.data.clone(),
            // set to constant if input_audio_format = pcm, if not, None
            channels,
            sample_rate,
            transcript: Some(completion_audio.transcript.clone()),
            format: Some(match input_audio_format {
                openai_api::ChatCompletionAudioParamFormat::Pcm16 => AudioFormat::Linear16,
                openai_api::ChatCompletionAudioParamFormat::Mp3 => AudioFormat::Mp3,
                openai_api::ChatCompletionAudioParamFormat::Flac => AudioFormat::Flac,
                openai_api::ChatCompletionAudioParamFormat::Opus => AudioFormat::Opus,
                openai_api::ChatCompletionAudioParamFormat::Wav => AudioFormat::Wav,
            }),
        };

        parts.push(Part::Audio(audio_part));
    }

    if let Some(tool_calls) = message.tool_calls.as_ref() {
        for tool_call in tool_calls {
            parts.push(Part::ToolCall(ToolCallPart {
                tool_call_id: tool_call.id.clone(),
                tool_name: tool_call.function.name.clone(),
                args: serde_json::from_str(tool_call.function.arguments.as_str()).map_err(|e| {
                    LanguageModelError::InvalidInput(format!("invalid json: {}", e))
                })?,
                id: None,
            }))
        }
    }

    Ok(parts)
}
