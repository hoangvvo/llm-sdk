use crate::{
    client_utils, id_utils,
    openai::responses_api::{
        self, CreateModelResponseProperties, CreateModelResponsePropertiesAllOf2, CreateResponse,
        CreateResponseAllOf3, DetailEnum, FunctionCallOutputItemParam,
        FunctionCallOutputItemParamOutput, FunctionCallOutputItemParamOutputArrayItem,
        FunctionCallOutputItemParamType, FunctionTool, FunctionToolCall, FunctionToolType,
        ImageDetail, ImageGenTool, ImageGenToolCall, ImageGenToolType, IncludeEnum, InputContent,
        InputImageContent, InputImageContentParamAutoParam, InputItem, InputMessage,
        InputMessageRole, InputMessageType, InputTextContent, InputTextContentParam,
        ModelResponseProperties, OutputItem, OutputMessage, OutputMessageContent,
        OutputMessageRole, OutputMessageStatus, OutputTextContent, Reasoning, ReasoningItem,
        ReasoningSummary, Response, ResponseFormatJsonObject, ResponseFormatText,
        ResponseProperties, ResponseStreamEvent, ResponseTextParam, ResponseUsage,
        SummaryTextContent, SummaryTextContentType, TextResponseFormatConfiguration,
        TextResponseFormatJsonSchema, Tool as OpenAITool, ToolChoiceFunction,
        ToolChoiceFunctionType, ToolChoiceOptions, ToolChoiceParam,
    },
    source_part_utils, AssistantMessage, ContentDelta, ImagePart, ImagePartDelta, LanguageModel,
    LanguageModelError, LanguageModelInput, LanguageModelMetadata, LanguageModelResult,
    LanguageModelStream, Message, ModelResponse, ModelUsage, Part, PartDelta, PartialModelResponse,
    ReasoningOptions, ReasoningPart, ReasoningPartDelta, ResponseFormatJson, ResponseFormatOption,
    TextPartDelta, Tool, ToolCallPart, ToolCallPartDelta, ToolChoiceOption, ToolMessage,
    ToolResultPart, UserMessage,
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

pub struct OpenAIModel {
    model_id: String,
    api_key: String,
    base_url: String,
    client: Client,
    metadata: Option<Arc<LanguageModelMetadata>>,
    headers: HashMap<String, String>,
}

#[derive(Clone, Default)]
pub struct OpenAIModelOptions {
    pub base_url: Option<String>,
    pub api_key: String,
    pub headers: Option<HashMap<String, String>>,
    pub client: Option<Client>,
}

impl OpenAIModel {
    #[must_use]
    pub fn new(model_id: impl Into<String>, options: OpenAIModelOptions) -> Self {
        let OpenAIModelOptions {
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
                    let header_map = self.request_headers()?;

                    let json: Response = client_utils::send_json(
                        &self.client,
                        &format!("{}/responses", self.base_url),
                        &params,
                        header_map,
                    )
                    .await?;
                    let output = json.response_all_of_3.output;
                    let usage = json.response_all_of_3.usage;

                    let content = map_openai_output_items(output)?;
                    let usage = usage.map(|usage| map_openai_response_usage(&usage));

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
                    params.create_response_all_of_3.stream = Some(true);
                    let header_map = self.request_headers()?;

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

                            if let ResponseStreamEvent::ResponseCompleted(ref completed_event) = event {
                                if let Some(usage) = &completed_event.response.response_all_of_3.usage {
                                    let usage = map_openai_response_usage(usage);
                                    yield PartialModelResponse {
                                        delta: None,
                                        cost: metadata.as_ref().and_then(|m| m.pricing.as_ref()).map(|pricing| usage.calculate_cost(pricing)),
                                        usage: Some(usage),
                                    }
                                }
                            }

                            if let ResponseStreamEvent::ResponseRefusalDelta(ref refusal_delta_event) = event {
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
) -> LanguageModelResult<CreateResponse> {
    let LanguageModelInput {
        messages,
        system_prompt,
        max_tokens,
        temperature,
        top_p,
        response_format,
        tools,
        tool_choice,
        modalities,
        reasoning,
        ..
    } = input;

    let include_reasoning_encrypted = reasoning.as_ref().is_some_and(|r| r.enabled);

    let mut params = CreateResponse {
        create_model_response_properties: CreateModelResponseProperties {
            model_response_properties: ModelResponseProperties {
                metadata: None,
                prompt_cache_key: None,
                prompt_cache_retention: None,
                safety_identifier: None,
                service_tier: None,
                temperature,
                top_logprobs: None,
                top_p,
                user: None,
            },
            create_model_response_properties_all_of_2: CreateModelResponsePropertiesAllOf2 {
                top_logprobs: None,
            },
        },
        response_properties: ResponseProperties {
            background: None,
            max_output_tokens: max_tokens.map(i64::from),
            max_tool_calls: None,
            model: Some(Some(model_id.to_string())),
            previous_response_id: None,
            prompt: None,
            reasoning: reasoning.map(convert_to_openai_reasoning).transpose()?,
            text: response_format.map(Into::into),
            tool_choice: tool_choice.map(convert_to_openai_response_tool_choice),
            tools: tools
                .map(|ts| {
                    ts.into_iter()
                        .map(convert_to_openai_tool)
                        .collect::<LanguageModelResult<Vec<_>>>()
                })
                .transpose()?
                .map(Some),
            truncation: None,
        },
        create_response_all_of_3: CreateResponseAllOf3 {
            context_management: None,
            conversation: None,
            include: if include_reasoning_encrypted {
                Some(vec![IncludeEnum::ReasoningEncryptedContent])
            } else {
                None
            },
            input: Some(responses_api::InputParam::InputParamArray(Some(
                convert_to_openai_inputs(messages)?,
            ))),
            instructions: system_prompt,
            parallel_tool_calls: None,
            store: Some(false),
            stream: None,
            stream_options: None,
        },
    };

    if modalities.is_some_and(|m| m.contains(&crate::Modality::Image)) {
        params
            .response_properties
            .tools
            .get_or_insert_with(|| Some(Vec::new()))
            .get_or_insert_with(Vec::new)
            .push(OpenAITool::ImageGenTool(ImageGenTool {
                action: None,
                background: None,
                input_fidelity: None,
                input_image_mask: None,
                model: None,
                moderation: None,
                output_compression: None,
                output_format: None,
                partial_images: None,
                quality: None,
                size: None,
                r#type: ImageGenToolType::ImageGeneration,
            }));
    }

    Ok(params)
}

fn convert_to_openai_inputs(messages: Vec<Message>) -> LanguageModelResult<Vec<InputItem>> {
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

impl TryFrom<UserMessage> for InputItem {
    type Error = LanguageModelError;
    fn try_from(user_message: UserMessage) -> Result<Self, Self::Error> {
        let message_parts =
            source_part_utils::get_compatible_parts_without_source_parts(user_message.content);
        Ok(Self::Item(responses_api::Item::InputMessage(
            InputMessage {
                role: InputMessageRole::User,
                status: None,
                r#type: Some(InputMessageType::Message),
                content: Some(
                    message_parts
                        .into_iter()
                        .map(|part| {
                            Ok(match part {
                                Part::Text(text_part) => {
                                    InputContent::InputText(InputTextContent {
                                        text: text_part.text,
                                    })
                                }
                                Part::Image(image_part) => {
                                    InputContent::InputImage(InputImageContent {
                                        detail: ImageDetail::Auto,
                                        file_id: None,
                                        image_url: Some(format!(
                                            "data:{};base64,{}",
                                            image_part.mime_type, image_part.data
                                        )),
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
                ),
            },
        )))
    }
}

fn convert_assistant_message_to_response_input_items(
    assistant_message: AssistantMessage,
) -> LanguageModelResult<Vec<InputItem>> {
    let message_parts =
        source_part_utils::get_compatible_parts_without_source_parts(assistant_message.content);

    message_parts
        .into_iter()
        .try_fold(Vec::new(), |mut acc, part| {
            let item = match part {
                Part::Text(text_part) => {
                    Some(InputItem::Item(responses_api::Item::OutputMessage(
                        OutputMessage {
                            // Response output item requires an ID.
                            // This usually applies if we enable OpenAI "store".
                            // or that we propogate the message ID in output.
                            // For compatibility, we want to avoid doing that, so we use a generated
                            // ID to avoid the API from returning an
                            // error.
                            id: format!("msg_{}", id_utils::generate_string(15)),
                            role: OutputMessageRole::Assistant,
                            content: vec![OutputMessageContent::OutputText(OutputTextContent {
                                text: text_part.text,
                                annotations: vec![],
                                logprobs: vec![],
                            })],
                            phase: None,
                            status: OutputMessageStatus::Completed,
                        },
                    )))
                }
                Part::Reasoning(reasoning_part) => Some(InputItem::Item(
                    responses_api::Item::ReasoningItem(ReasoningItem {
                        id: reasoning_part.id.unwrap_or_default(),
                        summary: vec![SummaryTextContent {
                            text: reasoning_part.text,
                            r#type: SummaryTextContentType::SummaryText,
                        }],
                        content: None,
                        encrypted_content: reasoning_part.signature,
                        status: None,
                    }),
                )),
                Part::Image(image_part) => Some(InputItem::Item(
                    responses_api::Item::ImageGenToolCall(ImageGenToolCall {
                        action: None,
                        background: None,
                        id: image_part.id.unwrap_or_default(),
                        output_format: None,
                        quality: None,
                        status: responses_api::ImageGenToolCallStatus::Completed,
                        result: Some(format!(
                            "data:{};base64,{}",
                            image_part.mime_type, image_part.data
                        )),
                        revised_prompt: None,
                        size: None,
                    }),
                )),
                Part::ToolCall(tool_call_part) => Some(InputItem::Item(
                    responses_api::Item::FunctionToolCall(FunctionToolCall {
                        arguments: tool_call_part.args.to_string(),
                        call_id: tool_call_part.tool_call_id,
                        name: tool_call_part.tool_name,
                        id: tool_call_part.id,
                        namespace: None,
                        status: None,
                    }),
                )),
                _ => Err(LanguageModelError::Unsupported(
                    PROVIDER,
                    format!("Cannot convert part to OpenAI input item for part {part:?}"),
                ))?,
            };
            if let Some(item) = item {
                acc.push(item);
            }
            Ok(acc)
        })
}

fn convert_tool_message_to_response_input_items(
    tool_message: ToolMessage,
) -> LanguageModelResult<Vec<InputItem>> {
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
                        let output = match tool_result_part_part {
                            Part::Text(text_part) => FunctionCallOutputItemParamOutput::FunctionCallOutputItemParamOutputArray(Some(vec![
                                FunctionCallOutputItemParamOutputArrayItem::InputText(
                                    InputTextContentParam {
                                        text: text_part.text,
                                    },
                                ),
                            ])),
                            Part::Image(image_part) => FunctionCallOutputItemParamOutput::FunctionCallOutputItemParamOutputArray(Some(vec![
                                FunctionCallOutputItemParamOutputArrayItem::InputImage(
                                    InputImageContentParamAutoParam {
                                        detail: Some(DetailEnum::Auto),
                                        file_id: None,
                                        image_url: Some(format!(
                                            "data:{};base64,{}",
                                            image_part.mime_type, image_part.data
                                        )),
                                    },
                                ),
                            ])),
                            _ => Err(LanguageModelError::Unsupported(
                                PROVIDER,
                                format!(
                                    "Cannot convert tool result part to OpenAI input item for \
                                     part {tool_result_part_part:?}"
                                ),
                            ))?,
                        };

                        Ok(InputItem::Item(responses_api::Item::FunctionCallOutputItemParam(
                            FunctionCallOutputItemParam {
                                call_id: tool_call_id.clone(),
                                output,
                                id: None,
                                status: None,
                                r#type: FunctionCallOutputItemParamType::FunctionCallOutput,
                            },
                        )))
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

fn convert_to_openai_tool(tool: Tool) -> LanguageModelResult<OpenAITool> {
    Ok(OpenAITool::FunctionTool(FunctionTool {
        defer_loading: None,
        description: Some(tool.description),
        name: tool.name,
        parameters: Some(convert_json_object(tool.parameters)?),
        strict: Some(true),
        r#type: FunctionToolType::Function,
    }))
}

fn convert_to_openai_response_tool_choice(tool_choice: ToolChoiceOption) -> ToolChoiceParam {
    match tool_choice {
        ToolChoiceOption::None => ToolChoiceParam::ToolChoiceOptions(ToolChoiceOptions::None),
        ToolChoiceOption::Auto => ToolChoiceParam::ToolChoiceOptions(ToolChoiceOptions::Auto),
        ToolChoiceOption::Required => {
            ToolChoiceParam::ToolChoiceOptions(ToolChoiceOptions::Required)
        }
        ToolChoiceOption::Tool(tool) => ToolChoiceParam::ToolChoiceFunction(ToolChoiceFunction {
            name: tool.tool_name,
            r#type: ToolChoiceFunctionType::Function,
        }),
    }
}

impl From<ResponseFormatOption> for ResponseTextParam {
    fn from(value: ResponseFormatOption) -> Self {
        match value {
            ResponseFormatOption::Json(ResponseFormatJson {
                name,
                description,
                schema,
            }) => {
                if let Some(schema) = schema {
                    Self {
                        format: Some(TextResponseFormatConfiguration::JsonSchema(
                            TextResponseFormatJsonSchema {
                                name,
                                description,
                                schema: Some(schema),
                                strict: Some(true),
                            },
                        )),
                        verbosity: None,
                    }
                } else {
                    Self {
                        format: Some(TextResponseFormatConfiguration::JsonObject(
                            ResponseFormatJsonObject {},
                        )),
                        verbosity: None,
                    }
                }
            }
            ResponseFormatOption::Text => Self {
                format: Some(TextResponseFormatConfiguration::Text(ResponseFormatText {})),
                verbosity: None,
            },
        }
    }
}

fn convert_to_openai_reasoning(value: ReasoningOptions) -> LanguageModelResult<Reasoning> {
    Ok(Reasoning {
        effort: value
            .budget_tokens
            .map(crate::openai::types::reasoning_effort_from_budget)
            .transpose()?,
        generate_summary: None,
        summary: value.enabled.then_some(ReasoningSummary::Auto),
    })
}

fn map_openai_output_items(items: Vec<OutputItem>) -> LanguageModelResult<Vec<Part>> {
    items
        .into_iter()
        .try_fold(Vec::new(), |mut acc, item| match item {
            OutputItem::Message(msg) => {
                let parts = msg
                    .content
                    .into_iter()
                    .map(|content| match content {
                        OutputMessageContent::OutputText(output_text) => {
                            Ok(Part::text(output_text.text))
                        }
                        OutputMessageContent::Refusal(refusal) => {
                            Err(LanguageModelError::Refusal(refusal.refusal))
                        }
                    })
                    .collect::<LanguageModelResult<Vec<_>>>()?;

                acc.extend(parts);
                Ok(acc)
            }
            OutputItem::FunctionCall(function_tool_call) => {
                let args = serde_json::from_str(&function_tool_call.arguments).map_err(|e| {
                    LanguageModelError::Invariant(
                        PROVIDER,
                        format!("Failed to parse function tool call arguments: {e}"),
                    )
                })?;
                let mut tool_call_part =
                    ToolCallPart::new(function_tool_call.call_id, function_tool_call.name, args);

                tool_call_part.id = function_tool_call.id;
                let part = Part::ToolCall(tool_call_part);

                acc.push(part);
                Ok(acc)
            }
            OutputItem::ImageGenerationCall(image_gen_call) => {
                let mut image_part = ImagePart::new(
                    image_gen_call.result.ok_or_else(|| {
                        LanguageModelError::Invariant(
                            PROVIDER,
                            "Image generation call did not return a result".to_string(),
                        )
                    })?,
                    openai_image_format_to_mime_type(image_gen_call.output_format.as_ref()),
                )
                .with_id(image_gen_call.id);

                if let Some((width, height)) = parse_openai_image_size(image_gen_call.size.as_ref())
                {
                    image_part = image_part.with_width(width).with_height(height);
                }
                let part: Part = image_part.into();

                acc.push(part);
                Ok(acc)
            }
            OutputItem::Reasoning(reasoning_item) => {
                let summary_text = reasoning_item
                    .summary
                    .into_iter()
                    .map(|summary_text| summary_text.text)
                    .collect::<Vec<_>>()
                    .join("\n");

                let mut reasoning_part =
                    ReasoningPart::new(summary_text).with_id(reasoning_item.id);
                if let Some(signature) = reasoning_item.encrypted_content {
                    reasoning_part = reasoning_part.with_signature(signature);
                }
                let part: Part = reasoning_part.into();

                acc.push(part);
                Ok(acc)
            }
            OutputItem::WebSearchCall(_) => Ok(acc),
            _ => Ok(acc),
        })
}

fn map_openai_stream_event(
    event: ResponseStreamEvent,
) -> LanguageModelResult<Option<ContentDelta>> {
    match event {
        ResponseStreamEvent::ResponseFailed(_) => Err(LanguageModelError::Invariant(
            PROVIDER,
            "OpenAI stream event failed".to_string(),
        )),
        ResponseStreamEvent::ResponseOutputItemAdded(output_item_added_event) => {
            match output_item_added_event.item {
                OutputItem::FunctionCall(function_tool_call) => {
                    let tool_call_part = PartDelta::ToolCall(ToolCallPartDelta {
                        args: Some(function_tool_call.arguments),
                        tool_name: Some(function_tool_call.name),
                        tool_call_id: Some(function_tool_call.call_id),
                        signature: None,
                        id: function_tool_call.id,
                    });
                    Ok(Some(ContentDelta {
                        index: usize::try_from(output_item_added_event.output_index).unwrap_or(0),
                        part: tool_call_part,
                    }))
                }
                OutputItem::Reasoning(reasoning_item) => {
                    if let Some(encrypted_content) = reasoning_item.encrypted_content {
                        let reasoning_part = ReasoningPartDelta {
                            signature: Some(encrypted_content),
                            text: None,
                            id: Some(reasoning_item.id),
                        };
                        let reasoning_part = PartDelta::Reasoning(reasoning_part);
                        Ok(Some(ContentDelta {
                            index: usize::try_from(output_item_added_event.output_index)
                                .unwrap_or(0),
                            part: reasoning_part,
                        }))
                    } else {
                        Ok(None)
                    }
                }
                _ => Ok(None),
            }
        }
        ResponseStreamEvent::ResponseOutputTextDelta(text_delta_event) => {
            let text_part = PartDelta::Text(TextPartDelta {
                text: text_delta_event.delta,
                citation: None,
            });
            Ok(Some(ContentDelta {
                index: usize::try_from(text_delta_event.output_index).unwrap_or(0),
                part: text_part,
            }))
        }
        ResponseStreamEvent::ResponseFunctionCallArgumentsDelta(
            function_call_arguments_delta_event,
        ) => {
            let tool_call_part = PartDelta::ToolCall(ToolCallPartDelta {
                args: Some(function_call_arguments_delta_event.delta),
                ..Default::default()
            });

            Ok(Some(ContentDelta {
                index: usize::try_from(function_call_arguments_delta_event.output_index)
                    .unwrap_or(0),
                part: tool_call_part,
            }))
        }
        ResponseStreamEvent::ResponseImageGenerationCallPartialImage(partial_image_event) => {
            let (width, height) = match parse_openai_image_size(partial_image_event.size.as_ref()) {
                Some((width, height)) => (Some(width), Some(height)),
                None => (None, None),
            };
            let image_part = PartDelta::Image(ImagePartDelta {
                width,
                height,
                mime_type: Some(openai_image_format_to_mime_type(
                    partial_image_event.output_format.as_ref(),
                )),
                data: Some(partial_image_event.partial_image_b_64),
                id: Some(partial_image_event.item_id),
            });

            Ok(Some(ContentDelta {
                index: usize::try_from(partial_image_event.output_index).unwrap_or(0),
                part: image_part,
            }))
        }
        ResponseStreamEvent::ResponseReasoningSummaryTextDelta(
            reasoning_summary_text_delta_event,
        ) => {
            let reasoning_part = PartDelta::Reasoning(ReasoningPartDelta {
                text: Some(reasoning_summary_text_delta_event.delta),
                ..Default::default()
            });
            Ok(Some(ContentDelta {
                index: usize::try_from(reasoning_summary_text_delta_event.output_index)
                    .unwrap_or(0),
                part: reasoning_part,
            }))
        }
        _ => Ok(None),
    }
}

fn openai_image_format_to_mime_type(format: Option<&String>) -> String {
    format!("image/{}", format.map(String::as_str).unwrap_or("png"))
}

fn parse_openai_image_size(size: Option<&String>) -> Option<(u32, u32)> {
    let (width, height) = size?.split_once('x')?;
    Some((width.parse().ok()?, height.parse().ok()?))
}

fn map_openai_response_usage(value: &ResponseUsage) -> ModelUsage {
    ModelUsage {
        input_tokens: u32::try_from(value.input_tokens).unwrap_or(0),
        output_tokens: u32::try_from(value.output_tokens).unwrap_or(0),
        input_tokens_details: Some(crate::ModelTokensDetails {
            cached_text_tokens: u32::try_from(value.input_tokens_details.cached_tokens).ok(),
            ..Default::default()
        }),
        output_tokens_details: Some(crate::ModelTokensDetails {
            text_tokens: Some(u32::try_from(value.output_tokens).unwrap_or(0)),
            cached_text_tokens: None,
            audio_tokens: None,
            cached_audio_tokens: None,
            image_tokens: None,
            cached_image_tokens: None,
        }),
    }
}

fn convert_json_object(value: Value) -> LanguageModelResult<HashMap<String, Value>> {
    match value {
        Value::Object(map) => Ok(map.into_iter().collect()),
        Value::Null => Ok(HashMap::new()),
        _ => Err(LanguageModelError::InvalidInput(
            "OpenAI function parameters must be a JSON object".to_string(),
        )),
    }
}
