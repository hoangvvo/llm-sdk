use crate::{
    client_utils, id_utils,
    openai::responses_api::{
        self, FunctionTool, ResponseCreateParams, ResponseFormatJSONObject, ResponseFormatText,
        ResponseFormatTextConfig, ResponseFormatTextJSONSchemaConfig, ResponseFunctionToolCall,
        ResponseIncludable, ResponseInputAudio, ResponseInputAudioInputAudio, ResponseInputContent,
        ResponseInputImage, ResponseInputItem, ResponseInputItemFunctionCallOutput,
        ResponseInputItemMessage, ResponseInputText, ResponseOutputContent, ResponseOutputItem,
        ResponseOutputItemImageGenerationCall, ResponseOutputMessage, ResponseOutputText,
        ResponseReasoningItem, ResponseReasoningItemSummary, ResponseReasoningItemSummaryUnion,
        ResponseStreamEvent, ResponseTextConfig, ResponseUsage, ToolChoiceFunction,
        ToolImageGeneration,
    },
    source_part_utils, AssistantMessage, AudioFormat, ContentDelta, ImagePart, ImagePartDelta,
    LanguageModel, LanguageModelError, LanguageModelInput, LanguageModelMetadata,
    LanguageModelResult, LanguageModelStream, Message, ModelResponse, ModelUsage, Part, PartDelta,
    PartialModelResponse, ReasoningOptions, ReasoningPart, ReasoningPartDelta, ResponseFormatJson,
    ResponseFormatOption, TextPartDelta, Tool, ToolCallPart, ToolCallPartDelta, ToolChoiceOption,
    ToolMessage, ToolResultPart, UserMessage,
};
use async_stream::try_stream;
use futures::{future::BoxFuture, StreamExt};
use reqwest::{header, Client};
use std::sync::Arc;

const PROVIDER: &str = "openai";

pub struct OpenAIModel {
    model_id: String,
    api_key: String,
    base_url: String,
    client: Client,
    metadata: Option<Arc<LanguageModelMetadata>>,
}

#[derive(Clone, Default)]
pub struct OpenAIModelOptions {
    pub base_url: Option<String>,
    pub api_key: String,
}

impl OpenAIModel {
    #[must_use]
    pub fn new(model_id: impl Into<String>, options: OpenAIModelOptions) -> Self {
        let client = Client::new();

        let base_url = options
            .base_url
            .unwrap_or_else(|| "https://api.openai.com/v1".to_string());
        let api_key = options.api_key;

        Self {
            model_id: model_id.into(),
            api_key,
            base_url,
            client,
            metadata: None,
        }
    }

    #[must_use]
    pub fn with_metadata(mut self, metadata: LanguageModelMetadata) -> Self {
        self.metadata = Some(Arc::new(metadata));
        self
    }
}

impl LanguageModel for OpenAIModel {
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
                    let params = convert_to_response_create_params(input, &self.model_id())?;

                    let mut header_map = reqwest::header::HeaderMap::new();
                    if let Ok(header_val) =
                        header::HeaderValue::from_str(&format!("Bearer {}", self.api_key))
                    {
                        header_map.insert(header::AUTHORIZATION, header_val);
                    }

                    let json: responses_api::Response = client_utils::send_json(
                        &self.client,
                        &format!("{}/responses", self.base_url),
                        &params,
                        header_map,
                    )
                    .await?;

                    let responses_api::Response { output, usage, .. } = json;

                    let content = map_openai_output_items(output)?;
                    let usage = usage.map(ModelUsage::from);

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
                    let mut params = convert_to_response_create_params(input, &self.model_id())?;
                    params.stream = Some(true);

                    let mut header_map = reqwest::header::HeaderMap::new();
                    if let Ok(header_val) =
                        header::HeaderValue::from_str(&format!("Bearer {}", self.api_key))
                    {
                        header_map.insert(header::AUTHORIZATION, header_val);
                    }

                    let mut chunk_stream = client_utils::send_sse_stream::<_, ResponseStreamEvent>(
                        &self.client,
                        &format!("{}/responses", self.base_url),
                        &params,
                        header_map,
                        self.provider(),
                    )
                    .await?;

                    let stream = try_stream! {
                        let mut refusal = String::new();

                        while let Some(event) = chunk_stream.next().await {
                            let event = event?;

                            if let ResponseStreamEvent::Completed(ref completed_event) = event {
                                if let Some(usage) = &completed_event.response.usage {
                                    let usage = ModelUsage::from(usage.clone());
                                    yield PartialModelResponse {
                                        delta: None,
                                        cost: metadata.as_ref().and_then(|m| m.pricing.as_ref()).map(|pricing| usage.calculate_cost(pricing)),
                                        usage: Some(usage),
                                    }
                                }
                            }

                            if let ResponseStreamEvent::RefusalDelta(ref refusal_delta_event) = event {
                                refusal.push_str(&refusal_delta_event.delta);
                            }

                            let part_delta = map_openai_stream_event(event)?;
                            if let Some(part_delta) = part_delta {
                                yield PartialModelResponse {
                                    delta: Some(part_delta),
                                    ..Default::default()
                                }
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

fn convert_to_response_create_params(
    input: LanguageModelInput,
    model_id: &str,
) -> LanguageModelResult<ResponseCreateParams> {
    let LanguageModelInput {
        messages,
        system_prompt,
        max_tokens,
        temperature,
        top_p,
        response_format,
        tools,
        tool_choice,
        extra,
        modalities,
        reasoning,
        ..
    } = input;

    let mut params = ResponseCreateParams {
        store: Some(false),
        model: Some(model_id.to_string()),
        input: Some(convert_to_openai_inputs(messages)?),
        instructions: system_prompt,
        max_output_tokens: max_tokens,
        temperature,
        top_p,
        tools: tools.map(|ts| ts.into_iter().map(Into::into).collect()),
        tool_choice: tool_choice
            .map(convert_to_openai_response_tool_choice)
            .transpose()?,
        text: response_format.map(Into::into),
        include: if reasoning.as_ref().is_some_and(|r| r.enabled) {
            Some(vec![ResponseIncludable::ReasoningEncryptedContent])
        } else {
            None
        },
        reasoning: reasoning.map(TryInto::try_into).transpose()?,
        extra,
        ..Default::default()
    };

    if modalities.is_some_and(|m| m.contains(&crate::Modality::Image)) {
        params
            .tools
            .get_or_insert_with(Vec::new)
            .push(responses_api::Tool::ImageGeneration(ToolImageGeneration {
                ..Default::default()
            }));
    }

    Ok(params)
}

fn convert_to_openai_inputs(messages: Vec<Message>) -> LanguageModelResult<Vec<ResponseInputItem>> {
    messages
        .into_iter()
        .try_fold(Vec::new(), |mut acc, message| {
            let mut items = match message {
                Message::User(user_message) => vec![user_message.try_into()?],
                Message::Assistant(assistant_message) => {
                    convert_assistant_message_to_response_input_items(assistant_message)?
                }
                Message::Tool(tool_message) => {
                    convert_tool_message_to_response_input_items(tool_message)?
                }
            };
            acc.append(&mut items);
            Ok(acc)
        })
}

impl TryFrom<UserMessage> for ResponseInputItem {
    type Error = LanguageModelError;
    fn try_from(user_message: UserMessage) -> Result<Self, Self::Error> {
        let message_parts =
            source_part_utils::get_compatible_parts_without_source_parts(user_message.content);
        Ok(Self::Message(ResponseInputItemMessage {
            role: "user".to_string(),
            content: message_parts
                .into_iter()
                .map(|part| {
                    Ok(match part {
                        Part::Text(text_part) => {
                            ResponseInputContent::InputText(ResponseInputText {
                                text: text_part.text,
                            })
                        }
                        Part::Image(image_part) => {
                            ResponseInputContent::InputImage(ResponseInputImage {
                                file_id: None,
                                image_url: format!(
                                    "data:{};base64,{}",
                                    image_part.mime_type, image_part.image_data
                                )
                                .into(),
                                detail: "auto".to_string(),
                            })
                        }
                        Part::Audio(audio_part) => {
                            let format = match audio_part.format {
                                AudioFormat::Mp3 => Ok("mp3"),
                                AudioFormat::Wav => Ok("wav"),
                                _ => Err(LanguageModelError::Unsupported(
                                    PROVIDER,
                                    format!(
                                        "Cannot convert audio format to OpenAI InputAudio format \
                                         for format {:?}",
                                        audio_part.format
                                    ),
                                )),
                            }?;

                            ResponseInputContent::InputAudio(ResponseInputAudio {
                                input_audio: ResponseInputAudioInputAudio {
                                    data: audio_part.audio_data,
                                    format: format.to_string(),
                                },
                            })
                        }
                        _ => Err(LanguageModelError::Unsupported(
                            PROVIDER,
                            format!(
                                "Cannot convert part to OpenAI input content for part {part:?}"
                            ),
                        ))?,
                    })
                })
                .collect::<LanguageModelResult<Vec<_>>>()?,
            status: None,
        }))
    }
}

fn convert_assistant_message_to_response_input_items(
    assistant_message: AssistantMessage,
) -> LanguageModelResult<Vec<ResponseInputItem>> {
    let message_parts =
        source_part_utils::get_compatible_parts_without_source_parts(assistant_message.content);

    message_parts
        .into_iter()
        .map(|part| {
            Ok(match part {
                Part::Text(text_part) => {
                    ResponseInputItem::OutputMessage(ResponseOutputMessage {
                        // Response output item requires an ID.
                        // This usually applies if we enable OpenAI "store".
                        // or that we propogate the message ID in output.
                        // For compatibility, we want to avoid doing that, so we use a generated
                        // ID to avoid the API from returning an
                        // error.
                        id: format!("msg_{}", id_utils::generate_string(15)),
                        role: "assistant".to_string(),
                        content: vec![ResponseOutputContent::OutputText(ResponseOutputText {
                            text: text_part.text,
                            annotations: vec![],
                        })],
                        status: "completed".to_string(),
                    })
                }
                Part::Reasoning(reasoning_part) => {
                    ResponseInputItem::Reasoning(ResponseReasoningItem {
                        id: reasoning_part.id.unwrap_or_default(),
                        summary: vec![ResponseReasoningItemSummaryUnion::SummaryText(
                            ResponseReasoningItemSummary {
                                text: reasoning_part.text,
                            },
                        )],
                        // ReasoningInputItem can not have content
                        content: None,
                        encrypted_content: reasoning_part.signature,
                        status: None,
                    })
                }
                Part::Image(image_part) => {
                    ResponseInputItem::ImageGenerationCall(ResponseOutputItemImageGenerationCall {
                        id: image_part.id.unwrap_or_default(),
                        status: "completed".to_string(),
                        result: Some(format!(
                            "data:{};base64,{}",
                            image_part.mime_type, image_part.image_data
                        )),
                        output_format: image_part
                            .mime_type
                            .strip_prefix("image/")
                            .unwrap_or("png")
                            .to_string(),
                        size: if let (Some(width), Some(height)) =
                            (image_part.width, image_part.height)
                        {
                            Some(format!("{width}x{height}"))
                        } else {
                            None
                        },
                    })
                }
                Part::ToolCall(tool_call_part) => {
                    ResponseInputItem::FunctionCall(ResponseFunctionToolCall {
                        arguments: tool_call_part.args.to_string(),
                        call_id: tool_call_part.tool_call_id,
                        name: tool_call_part.tool_name,
                        id: tool_call_part.id,
                        status: None,
                    })
                }
                _ => Err(LanguageModelError::Unsupported(
                    PROVIDER,
                    format!("Cannot convert part to OpenAI input item for part {part:?}"),
                ))?,
            })
        })
        .collect::<LanguageModelResult<_>>()
}

fn convert_tool_message_to_response_input_items(
    tool_message: ToolMessage,
) -> LanguageModelResult<Vec<ResponseInputItem>> {
    tool_message
        .content
        .into_iter()
        .try_fold(Vec::new(), |mut acc, part| {
            if let Part::ToolResult(ToolResultPart {
                content,
                tool_call_id,
                ..
            }) = part
            {
                let tool_result_part_content =
                    source_part_utils::get_compatible_parts_without_source_parts(content);

                let items = tool_result_part_content
                    .into_iter()
                    .map(|tool_result_part_part| {
                        Ok(match tool_result_part_part {
                            Part::Text(text_part) => ResponseInputItem::FunctionCallOutput(
                                ResponseInputItemFunctionCallOutput {
                                    call_id: tool_call_id.clone(),
                                    output: text_part.text,
                                    id: None,
                                    status: None,
                                },
                            ),
                            _ => Err(LanguageModelError::Unsupported(
                                PROVIDER,
                                format!(
                                    "Cannot convert tool result part to OpenAI input item for \
                                     part {tool_result_part_part:?}"
                                ),
                            ))?,
                        })
                    })
                    .collect::<LanguageModelResult<Vec<_>>>()?;

                acc.extend(items);

                Ok(acc)
            } else {
                Err(LanguageModelError::InvalidInput(
                    "Tool messages must contain only tool result parts".to_string(),
                ))
            }
        })
}

impl From<Tool> for responses_api::Tool {
    fn from(tool: Tool) -> Self {
        Self::Function(FunctionTool {
            name: tool.name,
            description: Some(tool.description),
            parameters: Some(tool.parameters),
            strict: Some(true),
        })
    }
}

fn convert_to_openai_response_tool_choice(
    tool_choice: ToolChoiceOption,
) -> LanguageModelResult<serde_json::Value> {
    match tool_choice {
        ToolChoiceOption::None => Ok("none".into()),
        ToolChoiceOption::Auto => Ok("auto".into()),
        ToolChoiceOption::Required => Ok("required".into()),
        ToolChoiceOption::Tool(tool) => serde_json::to_value(ToolChoiceFunction {
            choice_type: "function".into(),
            name: tool.tool_name,
        })
        .map_err(|e| {
            LanguageModelError::InvalidInput(format!(
                "Failed to convert tool choice to OpenAI format: {e}"
            ))
        }),
    }
}

impl From<ResponseFormatOption> for ResponseTextConfig {
    fn from(value: ResponseFormatOption) -> Self {
        match value {
            ResponseFormatOption::Json(ResponseFormatJson {
                name,
                description,
                schema,
            }) => {
                if let Some(schema) = schema {
                    Self {
                        format: Some(ResponseFormatTextConfig::JsonSchema(
                            ResponseFormatTextJSONSchemaConfig {
                                name,
                                description,
                                schema,
                                strict: Some(true),
                            },
                        )),
                        verbosity: None,
                    }
                } else {
                    Self {
                        format: Some(ResponseFormatTextConfig::JsonObject(
                            ResponseFormatJSONObject {},
                        )),
                        verbosity: None,
                    }
                }
            }
            ResponseFormatOption::Text => Self {
                format: Some(ResponseFormatTextConfig::Text(ResponseFormatText {})),
                verbosity: None,
            },
        }
    }
}

impl TryFrom<ReasoningOptions> for responses_api::Reasoning {
    type Error = LanguageModelError;

    fn try_from(value: ReasoningOptions) -> Result<Self, Self::Error> {
        Ok(Self {
            summary: value.enabled.then(|| "auto".to_string()),
            effort: value.budget_tokens.map(TryInto::try_into).transpose()?,
        })
    }
}

fn map_openai_output_items(items: Vec<ResponseOutputItem>) -> LanguageModelResult<Vec<Part>> {
    items
        .into_iter()
        .try_fold(Vec::new(), |mut acc, item| match item {
            ResponseOutputItem::Message(msg) => {
                let parts = msg
                    .content
                    .into_iter()
                    .map(|content| match content {
                        ResponseOutputContent::OutputText(output_text) => {
                            Ok(Part::text(output_text.text))
                        }
                        ResponseOutputContent::Refusal(refusal) => {
                            Err(LanguageModelError::Refusal(refusal.refusal))
                        }
                    })
                    .collect::<LanguageModelResult<Vec<_>>>()?;

                acc.extend(parts);
                Ok(acc)
            }
            ResponseOutputItem::FunctionCall(function_tool_call) => {
                let args = serde_json::from_str(&function_tool_call.arguments).map_err(|e| {
                    LanguageModelError::Invariant(
                        PROVIDER,
                        format!("Failed to parse function tool call arguments: {e}"),
                    )
                })?;
                let mut tool_call_part =
                    ToolCallPart::new(function_tool_call.call_id, function_tool_call.name, args);

                if let Some(id) = function_tool_call.id {
                    tool_call_part.id = Some(id);
                }
                let part = Part::ToolCall(tool_call_part);

                acc.push(part);
                Ok(acc)
            }
            ResponseOutputItem::ImageGenerationCall(image_gen_call) => {
                let (width, height) = if let Some(size) = image_gen_call.size {
                    parse_openai_image_size(&size)
                } else {
                    (None, None)
                };

                let part = Part::Image(ImagePart {
                    image_data: image_gen_call.result.ok_or_else(|| {
                        LanguageModelError::Invariant(
                            PROVIDER,
                            "Image generation call did not return a result".to_string(),
                        )
                    })?,
                    width,
                    height,
                    mime_type: format!("image/{}", image_gen_call.output_format),
                    id: Some(image_gen_call.id),
                });

                acc.push(part);
                Ok(acc)
            }
            ResponseOutputItem::Reasoning(reasoning_item) => {
                let summary_text = reasoning_item
                    .summary
                    .into_iter()
                    .map(|summary_union| match summary_union {
                        ResponseReasoningItemSummaryUnion::SummaryText(summary_text) => {
                            summary_text.text
                        }
                    })
                    .collect::<Vec<_>>()
                    .join("\n");

                let part = Part::Reasoning(ReasoningPart {
                    text: summary_text,
                    signature: reasoning_item.encrypted_content,
                    id: Some(reasoning_item.id),
                });

                acc.push(part);
                Ok(acc)
            }
            ResponseOutputItem::WebSearchCall(_) => Ok(acc),
        })
}

fn map_openai_stream_event(
    event: ResponseStreamEvent,
) -> LanguageModelResult<Option<ContentDelta>> {
    match event {
        ResponseStreamEvent::Failed(_) => Err(LanguageModelError::Invariant(
            PROVIDER,
            "OpenAI stream event failed".to_string(),
        )),
        ResponseStreamEvent::OutputItemAdded(output_item_added_event) => {
            match output_item_added_event.item {
                ResponseOutputItem::FunctionCall(function_tool_call) => {
                    let tool_call_part = PartDelta::ToolCall(ToolCallPartDelta {
                        args: Some(function_tool_call.arguments),
                        tool_name: Some(function_tool_call.name),
                        tool_call_id: Some(function_tool_call.call_id),
                        id: function_tool_call.id,
                    });
                    Ok(Some(ContentDelta {
                        index: output_item_added_event.output_index,
                        part: tool_call_part,
                    }))
                }
                ResponseOutputItem::Reasoning(reasoning_item) => {
                    if let Some(encrypted_content) = reasoning_item.encrypted_content {
                        let reasoning_part = PartDelta::Reasoning(ReasoningPartDelta {
                            signature: Some(encrypted_content),
                            text: None,
                            id: Some(reasoning_item.id),
                        });
                        Ok(Some(ContentDelta {
                            index: output_item_added_event.output_index,
                            part: reasoning_part,
                        }))
                    } else {
                        Ok(None)
                    }
                }
                _ => Ok(None),
            }
        }
        ResponseStreamEvent::TextDelta(text_delta_event) => {
            let text_part = PartDelta::Text(TextPartDelta {
                text: text_delta_event.delta,
                citation: None,
            });
            Ok(Some(ContentDelta {
                index: text_delta_event.output_index,
                part: text_part,
            }))
        }
        ResponseStreamEvent::FunctionCallArgumentsDelta(function_call_arguments_delta_event) => {
            let tool_call_part = PartDelta::ToolCall(ToolCallPartDelta {
                args: Some(function_call_arguments_delta_event.delta),
                ..Default::default()
            });

            Ok(Some(ContentDelta {
                index: function_call_arguments_delta_event.output_index,
                part: tool_call_part,
            }))
        }
        ResponseStreamEvent::ImageGenCallPartialImage(partial_image_event) => {
            let (width, height) = if let Some(size) = partial_image_event.size {
                parse_openai_image_size(&size)
            } else {
                (None, None)
            };

            let image_part = PartDelta::Image(ImagePartDelta {
                width,
                height,
                mime_type: Some(format!("image/{}", partial_image_event.output_format)),
                image_data: Some(partial_image_event.partial_image_b64),
                id: Some(partial_image_event.item_id),
            });

            Ok(Some(ContentDelta {
                index: partial_image_event.output_index,
                part: image_part,
            }))
        }
        ResponseStreamEvent::ReasoningSummaryTextDelta(reasoning_summary_text_delta_event) => {
            let reasoning_part = PartDelta::Reasoning(ReasoningPartDelta {
                text: Some(reasoning_summary_text_delta_event.delta),
                ..Default::default()
            });
            Ok(Some(ContentDelta {
                index: reasoning_summary_text_delta_event.output_index,
                part: reasoning_part,
            }))
        }
        _ => Ok(None),
    }
}

impl From<ResponseUsage> for ModelUsage {
    fn from(value: ResponseUsage) -> Self {
        Self {
            input_tokens: value.input_tokens,
            output_tokens: value.output_tokens,
            ..Default::default()
        }
    }
}

// image size from openai is in the format of {number}x{number}, we parse it
// into width, height if available
fn parse_openai_image_size(size_dim: &str) -> (Option<u32>, Option<u32>) {
    let parts: Vec<&str> = size_dim.split('x').collect();
    let width = parts.first().and_then(|w| w.parse().ok());
    let height = parts.get(1).and_then(|h| h.parse().ok());
    (width, height)
}
