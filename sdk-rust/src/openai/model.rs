use reqwest::Client;

use crate::{
    language_model::{LanguageModelMetadata, LanguageModelStreamResult},
    openai::api as openai_api,
    usage::calculate_cost,
    AssistantMessage, AudioFormat, AudioPart, ImagePart, LanguageModel, LanguageModelError,
    LanguageModelInput, LanguageModelResult, Message, Modality, ModelResponse, ModelUsage, Part,
    ResponseFormatJson, ResponseFormatOption, TextPart, Tool, ToolCallPart, ToolChoiceOption,
    ToolMessage, UserMessage,
};

pub struct OpenAIModel {
    model_id: String,
    api_key: String,
    base_url: String,
    client: Client,
    metadata: Option<LanguageModelMetadata>,
}

impl OpenAIModel {
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

    pub fn with_base_url(mut self, base_url: String) -> Self {
        self.base_url = base_url;
        self
    }

    pub fn with_metadata(mut self, metadata: LanguageModelMetadata) -> Self {
        self.metadata = Some(metadata);
        self
    }
}

#[async_trait::async_trait]
impl LanguageModel for OpenAIModel {
    fn provider(&self) -> &'static str {
        return "openai";
    }

    fn model_id(&self) -> String {
        return self.model_id.clone();
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
            return Err(LanguageModelError::StatusCode(response.status(), response));
        }

        let json = response.json::<openai_api::ChatCompletion>().await?;

        let choice = json
            .choices
            .first()
            .ok_or_else(|| LanguageModelError::Invariant("no choices in response".to_string()))?;

        let openai_api::ChatCompletionChoice { message, .. } = choice;

        if let Some(refusal) = &message.refusal {
            return Err(LanguageModelError::Refusal(refusal.to_string()));
        }

        let content = map_openai_message(message)?;

        let usage: Option<ModelUsage> = json.usage.map(Into::into);

        // Calculate cost if both usage and self.metadata.pricing are available
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

    async fn stream(&self, input: LanguageModelInput) -> LanguageModelStreamResult {}
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
                        Part::Audio(part) => {
                            openai_message_param.audio = Some(part.try_into()?);
                        }
                        _ => Err(LanguageModelError::Unsupported(format!(
                            "Unsupported part in assistant message {:?}",
                            part
                        )))?,
                    }
                }

                openai_messages.push(openai_api::ChatCompletionMessageParam::Assistant(
                    openai_message_param,
                ))
            }

            Message::Tool(ToolMessage { content }) => {
                for part in content {
                    openai_messages.push(openai_api::ChatCompletionMessageParam::Tool(
                        openai_api::ChatCompletionToolMessageParam {
                            content: part
                                .content
                                .iter()
                                .map(|p| match p {
                                    Part::Text(part) => Ok(part.into()),
                                    _ => {
                                        return Err(LanguageModelError::Unsupported(format!(
                                            "Unsupported part in tool message {:?}",
                                            p
                                        )))
                                    }
                                })
                                .collect::<LanguageModelResult<_>>()?,
                            tool_call_id: part.tool_call_id.to_string(),
                        },
                    ))
                }
            }

            Message::User(UserMessage { content }) => {
                let mut openai_message_param =
                    openai_api::ChatCompletionUserMessageParam::default();

                for part in content {
                    match part {
                        Part::Text(part) => openai_message_param
                            .content
                            .push(openai_api::ChatCompletionContentPart::Text(part.into())),
                        Part::Image(part) => openai_message_param
                            .content
                            .push(openai_api::ChatCompletionContentPart::Image(part.into())),
                        Part::Audio(part) => openai_message_param.content.push(
                            openai_api::ChatCompletionContentPart::InputAudio(part.try_into()?),
                        ),
                        _ => Err(LanguageModelError::Unsupported(format!(
                            "Unsupported part in user message {:?}",
                            part
                        )))?,
                    }
                }
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
                openai_api::ChatCompletionMessageToolCall::Function(
                    openai_api::ChatCompletionMessageFunctionToolCall { id, function },
                ) => {
                    parts.push(Part::ToolCall(ToolCallPart {
                        tool_call_id: id.to_string(),
                        tool_name: function.name.to_string(),
                        args: serde_json::to_value(&function.arguments)
                            .map_err(|e| LanguageModelError::InvalidInput(e.to_string()))?,
                        id: None,
                    }));
                }
            }
        }
    }

    Ok(parts)
}

impl From<openai_api::CompletionsAPICompletionUsage> for ModelUsage {
    fn from(value: openai_api::CompletionsAPICompletionUsage) -> Self {
        let usage = ModelUsage {
            input_tokens: value.prompt_tokens,
            output_tokens: value.completion_tokens,
            ..Default::default()
        };
        // TODO: map details token
        usage
    }
}
