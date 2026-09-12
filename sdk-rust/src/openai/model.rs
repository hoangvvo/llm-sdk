use crate::{
    client_utils, id_utils,
    openai::responses_api::{
        self, Annotation, CreateResponse, CreateResponsePromptCacheRetention, DetailEnum,
        FunctionCallItemStatus, FunctionCallOutputItemParam, FunctionCallOutputItemParamOutput,
        FunctionCallOutputItemParamOutputArrayItem, FunctionCallOutputItemParamType,
        FunctionCallOutputStatusEnum, FunctionCallStatus, FunctionTool, FunctionToolCall,
        FunctionToolCallType, FunctionToolType, ImageDetail, ImageGenTool, ImageGenToolCall,
        ImageGenToolCallType, ImageGenToolType, IncludeEnum, InputContent, InputFileContent,
        InputFileContentParam, InputFileContentType, InputImageContent,
        InputImageContentParamAutoParam, InputImageContentType, InputItem, InputMessage,
        InputMessageRole, InputMessageType, InputTextContent, InputTextContentParam,
        InputTextContentType, NamespaceToolParamToolsItem, OutputItem, OutputMessage,
        OutputMessageContent, OutputMessageRole, OutputMessageStatus, OutputMessageType,
        OutputTextContent, Reasoning, ReasoningItem, ReasoningItemType, ReasoningSummary, Response,
        ResponseFormatJsonObject, ResponseFormatText, ResponseStreamEvent, ResponseTextParam,
        ResponseUsage, SummaryTextContent, SummaryTextContentType, TextResponseFormatConfiguration,
        TextResponseFormatJsonSchema, Tool as OpenAITool, ToolChoiceFunction,
        ToolChoiceFunctionType, ToolChoiceOptions, ToolChoiceParam, ToolSearchCall,
        ToolSearchCallItemParam, ToolSearchCallItemParamType, ToolSearchExecutionType,
        ToolSearchOutput, ToolSearchOutputItemParam, ToolSearchOutputItemParamType,
        ToolSearchToolParam, ToolSearchToolParamType, UrlCitationBody,
        WebSearchApproximateLocationValue, WebSearchApproximateLocationValueType,
        WebSearchTool as OpenAIWebSearchTool, WebSearchToolFilters, WebSearchToolType,
    },
    source_part_utils,
    tool_result_utils::CANCELLED_TOOL_RESULT_FALLBACK_CONTENT,
    AssistantMessage, CacheRetention, Citation, CitationDelta, ContentDelta, FilePart, ImagePart,
    ImagePartDelta, LanguageModel, LanguageModelError, LanguageModelInput, LanguageModelMetadata,
    LanguageModelResult, LanguageModelStream, Message, ModelResponse, ModelServerToolUsage,
    ModelUsage, ModelUsageCostOptions, Part, PartDelta, PartialModelResponse, ReasoningOptions,
    ReasoningPart, ReasoningPartDelta, ResponseFormatJson, ResponseFormatOption, TextPart,
    TextPartDelta, Tool, ToolCall, ToolCallDelta, ToolCallPart, ToolCallPartDelta,
    ToolChoiceOption, ToolMessage, ToolResult, ToolResultPart, ToolResultPartDelta,
    ToolResultStatus, ToolSearchToolCall, ToolSearchToolCallDelta, ToolSearchToolCallStatus,
    ToolSearchToolResult, UserMessage,
};
use async_stream::try_stream;
use futures::{future::BoxFuture, StreamExt};
use reqwest::{
    header::{self, HeaderMap, HeaderName, HeaderValue},
    Client,
};
use serde_json::Value;
use std::{collections::HashMap, sync::Arc};

const USAGE_COST_OPTIONS: ModelUsageCostOptions = ModelUsageCostOptions {
    input_cache_tokens_are_additional: false,
    output_reasoning_tokens_are_additional: false,
};

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
                    let mut params = convert_to_response_create_params(input, &self.model_id())?;
                    params.stream = Some(false);
                    let header_map = self.request_headers()?;

                    let json: Response = client_utils::send_json(
                        &self.client,
                        &format!("{}/responses", self.base_url),
                        &params,
                        header_map,
                    )
                    .await?;
                    let web_search_requests = json
                        .output
                        .iter()
                        .filter(|item| is_billable_web_search(item))
                        .count();
                    let content = map_openai_output_items(json.output)?;
                    let usage = json
                        .usage
                        .map(|usage| map_openai_response_usage(&usage, web_search_requests));

                    let cost = if let (Some(usage), Some(pricing)) = (
                        usage.as_ref(),
                        self.metadata().and_then(|m| m.pricing.as_ref()),
                    ) {
                        Some(usage.calculate_cost(pricing, &USAGE_COST_OPTIONS))
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
                        let mut normalized_output_indexes = HashMap::new();
                        let mut next_content_index = 0_usize;
                        let mut web_search_requests = 0_usize;
                        let mut stream_state = OpenAIStreamState::default();

                        while let Some(event) = chunk_stream.next().await {
                            let event = event?;

                            if let ResponseStreamEvent::ResponseOutputItemDone(ref done_event) = event {
                                if is_billable_web_search(&done_event.item) {
                                    web_search_requests += 1;
                                }
                            }

                            if let ResponseStreamEvent::ResponseCompleted(ref completed_event) = event {
                                if let Some(usage) = &completed_event.response.usage {
                                    let usage = map_openai_response_usage(usage, web_search_requests);
                                    yield PartialModelResponse {
                                        delta: None,
                                        cost: metadata.as_ref().and_then(|m| m.pricing.as_ref()).map(|pricing| usage.calculate_cost(pricing, &USAGE_COST_OPTIONS)),
                                        usage: Some(usage),
                                    }
                                }
                            }

                            if let ResponseStreamEvent::ResponseRefusalDelta(ref refusal_delta_event) = event {
                                refusal.push_str(&refusal_delta_event.delta);
                            }

                            let web_search_result = map_openai_stream_web_search_result(&event);
                            let part_delta = map_openai_stream_event(event, &mut stream_state)?;
                            if let Some(mut part_delta) = part_delta {
                                let provider_output_index = part_delta.index;
                                let normalized_output_index = *normalized_output_indexes
                                    .entry(provider_output_index)
                                    .or_insert_with(|| {
                                        let index = next_content_index;
                                        next_content_index += 1;
                                        index
                                    });
                                part_delta.index = normalized_output_index;
                                yield PartialModelResponse {
                                    delta: Some(part_delta),
                                    ..Default::default()
                                }
                            }
                            if let Some(result) = web_search_result {
                                let result_delta = ContentDelta {
                                    index: next_content_index,
                                    part: PartDelta::ToolResult(result),
                                };
                                next_content_index += 1;
                                yield PartialModelResponse {
                                    delta: Some(result_delta),
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
        metadata,
        cache_retention,
        ..
    } = input;

    let include_reasoning_encrypted = reasoning.is_some();
    let include_web_search_sources = tools
        .as_ref()
        .is_some_and(|tools| tools.iter().any(|tool| matches!(tool, Tool::WebSearch(_))));
    let mut include = Vec::new();
    if include_web_search_sources {
        include.push(IncludeEnum::WebSearchCallActionSources);
    }
    if include_reasoning_encrypted {
        include.push(IncludeEnum::ReasoningEncryptedContent);
    }

    let input_items = convert_to_openai_inputs(messages, tools.as_deref().unwrap_or_default())?;

    let mut params = CreateResponse {
        metadata: metadata.map(|metadata| Some(Some(metadata))),
        prompt_cache_key: None,
        prompt_cache_retention: cache_retention.map(|retention| match retention {
            CacheRetention::Standard => CreateResponsePromptCacheRetention::InMemory,
            CacheRetention::Extended => CreateResponsePromptCacheRetention::N24H,
        }),
        safety_identifier: None,
        service_tier: None,
        temperature,
        top_logprobs: None,
        top_p,
        user: None,
        background: None,
        max_output_tokens: max_tokens.map(i64::from),
        max_tool_calls: None,
        model: Some(Some(model_id.to_string())),
        previous_response_id: None,
        prompt: None,
        reasoning: reasoning
            .as_ref()
            .map(convert_to_openai_reasoning)
            .transpose()?,
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
        context_management: None,
        conversation: None,
        include: (!include.is_empty()).then_some(include),
        input: Some(responses_api::InputParam::InputParamArray(Some(
            input_items,
        ))),
        instructions: system_prompt,
        parallel_tool_calls: None,
        store: Some(false),
        stream: None,
        stream_options: None,
    };

    if modalities.is_some_and(|m| m.contains(&crate::Modality::Image)) {
        params
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

fn convert_to_openai_inputs(
    messages: Vec<Message>,
    tools: &[Tool],
) -> LanguageModelResult<Vec<InputItem>> {
    messages
        .into_iter()
        .try_fold(Vec::new(), |mut acc, message| {
            let mut items = match message {
                Message::User(user_message) => vec![user_message.try_into()?],
                Message::Assistant(assistant_message) => {
                    convert_assistant_message_to_response_input_items(assistant_message, tools)?
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
                                        r#type: InputTextContentType::InputText,
                                    })
                                }
                                Part::Image(image_part) => {
                                    InputContent::InputImage(InputImageContent {
                                        detail: Some(ImageDetail::Auto),
                                        file_id: None,
                                        image_url: Some(convert_to_openai_image_url(image_part)),
                                        r#type: InputImageContentType::InputImage,
                                    })
                                }
                                Part::File(file_part) => {
                                    let file = convert_to_openai_input_file(file_part);
                                    InputContent::InputFile(InputFileContent {
                                        detail: None,
                                        file_data: file.file_data,
                                        file_id: None,
                                        file_url: file.file_url,
                                        filename: file.filename,
                                        r#type: InputFileContentType::InputFile,
                                    })
                                }
                                _ => Err(LanguageModelError::Unsupported(
                                    PROVIDER,
                                    format!(
                                        "Cannot convert part to OpenAI input content for part \
                                         {part:?}"
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

/// Converts a media part to the `image_url` of an `OpenAI` input image.
fn convert_to_openai_image_url(image_part: ImagePart) -> String {
    image_part.url.unwrap_or_else(|| {
        format!(
            "data:{};base64,{}",
            image_part.mime_type,
            image_part.data.unwrap_or_default()
        )
    })
}

/// The fields shared by the `input_file` content of user messages and tool
/// results.
struct OpenAIInputFile {
    file_data: Option<String>,
    file_url: Option<String>,
    filename: Option<String>,
}

fn convert_to_openai_input_file(file_part: FilePart) -> OpenAIInputFile {
    let FilePart {
        mime_type,
        data,
        url,
        filename,
    } = file_part;
    match url {
        Some(url) => OpenAIInputFile {
            file_data: None,
            file_url: Some(url),
            filename,
        },
        None => OpenAIInputFile {
            file_data: Some(format!(
                "data:{mime_type};base64,{}",
                data.unwrap_or_default()
            )),
            file_url: None,
            filename,
        },
    }
}

#[allow(clippy::too_many_lines)]
fn convert_assistant_message_to_response_input_items(
    assistant_message: AssistantMessage,
    tools: &[Tool],
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
                            // Output messages require an ID, but the SDK does not expose provider
                            // IDs.
                            id: format!("msg_{}", id_utils::generate_string(15)),
                            role: OutputMessageRole::Assistant,
                            content: vec![OutputMessageContent::OutputText(OutputTextContent {
                                text: text_part.text,
                                annotations: vec![],
                                logprobs: vec![],
                            })],
                            phase: None,
                            status: OutputMessageStatus::Completed,
                            r#type: OutputMessageType::Message,
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
                        r#type: ReasoningItemType::Reasoning,
                    }),
                )),
                Part::Image(image_part) => Some(InputItem::Item(
                    responses_api::Item::ImageGenToolCall(ImageGenToolCall {
                        id: image_part.id.unwrap_or_default(),
                        output_format: None,
                        status: responses_api::ImageGenToolCallStatus::Completed,
                        result: Some(format!(
                            "data:{};base64,{}",
                            image_part.mime_type,
                            image_part.data.unwrap_or_default()
                        )),
                        size: None,
                        r#type: ImageGenToolCallType::ImageGenerationCall,
                    }),
                )),
                Part::ToolCall(tool_call_part) => match tool_call_part.call {
                    ToolCall::Function(call) => {
                        // Calls to deferred tools must be replayed with the namespace OpenAI
                        // assigned them, which for top-level functions is the function name.
                        let namespace = tools
                            .iter()
                            .any(|tool| {
                                matches!(
                                    tool,
                                    Tool::Function(function)
                                        if function.name == call.name
                                            && function.defer_loading == Some(true)
                                )
                            })
                            .then(|| call.name.clone());
                        Some(InputItem::Item(responses_api::Item::FunctionToolCall(
                            FunctionToolCall {
                                arguments: call.args.to_string(),
                                call_id: tool_call_part.tool_call_id,
                                name: call.name,
                                id: tool_call_part.id,
                                namespace,
                                status: None,
                                r#type: FunctionToolCallType::FunctionCall,
                            },
                        )))
                    }
                    ToolCall::WebSearch(call) => Some(convert_to_openai_web_search_call(
                        tool_call_part.tool_call_id,
                        call,
                    )?),
                    ToolCall::ToolSearch(call) => Some(InputItem::Item(
                        responses_api::Item::ToolSearchCallItemParam(ToolSearchCallItemParam {
                            arguments: Some(call.args),
                            call_id: None,
                            execution: Some(ToolSearchExecutionType::Server),
                            id: Some(tool_call_part.id.unwrap_or(tool_call_part.tool_call_id)),
                            status: Some(FunctionCallItemStatus::Completed),
                            r#type: ToolSearchCallItemParamType::ToolSearchCall,
                        }),
                    )),
                },
                // OpenAI replays hosted search results through the web_search_call item.
                Part::ToolResult(ToolResultPart {
                    result: ToolResult::WebSearch(_),
                    ..
                }) => None,
                Part::ToolResult(ToolResultPart {
                    result: ToolResult::ToolSearch(result),
                    status,
                    ..
                }) => Some(convert_to_openai_tool_search_output(
                    status, &result, tools,
                )?),
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
    let mut items = Vec::new();
    for part in tool_message.content {
        let Part::ToolResult(ToolResultPart {
            tool_call_id,
            result,
            status,
        }) = part
        else {
            return Err(LanguageModelError::InvalidInput(
                "Tool messages must contain only tool result parts".to_string(),
            ));
        };

        // Hosted tool results are replayed through their assistant-message items.
        let ToolResult::Function(result) = result else {
            continue;
        };

        let tool_result_part_content =
            source_part_utils::get_compatible_parts_without_source_parts(result.content);

        let output = if tool_result_part_content.is_empty() {
            FunctionCallOutputItemParamOutput::FunctionCallOutputItemParamOutputString(Some(
                if status == ToolResultStatus::Cancelled {
                    CANCELLED_TOOL_RESULT_FALLBACK_CONTENT.to_string()
                } else {
                    String::new()
                },
            ))
        } else {
            // A call has exactly one output item, so every result part becomes an
            // entry of the same output list.
            FunctionCallOutputItemParamOutput::FunctionCallOutputItemParamOutputArray(Some(
                tool_result_part_content
                    .into_iter()
                    .map(convert_to_openai_function_call_output_item)
                    .collect::<LanguageModelResult<Vec<_>>>()?,
            ))
        };

        items.push(InputItem::Item(
            responses_api::Item::FunctionCallOutputItemParam(FunctionCallOutputItemParam {
                call_id: tool_call_id,
                output,
                id: None,
                status: None,
                r#type: FunctionCallOutputItemParamType::FunctionCallOutput,
            }),
        ));
    }
    Ok(items)
}

fn convert_to_openai_function_call_output_item(
    part: Part,
) -> LanguageModelResult<FunctionCallOutputItemParamOutputArrayItem> {
    match part {
        Part::Text(text_part) => Ok(FunctionCallOutputItemParamOutputArrayItem::InputText(
            InputTextContentParam {
                text: text_part.text,
            },
        )),
        Part::Image(image_part) => Ok(FunctionCallOutputItemParamOutputArrayItem::InputImage(
            InputImageContentParamAutoParam {
                detail: Some(DetailEnum::Auto),
                file_id: None,
                image_url: Some(convert_to_openai_image_url(image_part)),
            },
        )),
        Part::File(file_part) => {
            let file = convert_to_openai_input_file(file_part);
            Ok(FunctionCallOutputItemParamOutputArrayItem::InputFile(
                InputFileContentParam {
                    detail: None,
                    file_data: file.file_data,
                    file_id: None,
                    file_url: file.file_url,
                    filename: file.filename,
                },
            ))
        }
        _ => Err(LanguageModelError::Unsupported(
            PROVIDER,
            format!("Cannot convert tool result part to OpenAI input item for part {part:?}"),
        )),
    }
}

fn convert_to_openai_tool_search_output(
    status: ToolResultStatus,
    result: &ToolSearchToolResult,
    tools: &[Tool],
) -> LanguageModelResult<InputItem> {
    // OpenAI needs the full definitions of the discovered tools, which the
    // request already declares as deferred function tools.
    let discovered_tools = result
        .tool_names
        .iter()
        .filter_map(|tool_name| {
            tools
                .iter()
                .find(
                    |tool| matches!(tool, Tool::Function(function) if function.name == *tool_name),
                )
                .cloned()
        })
        .map(convert_to_openai_tool)
        .collect::<LanguageModelResult<Vec<_>>>()?;
    Ok(InputItem::Item(
        responses_api::Item::ToolSearchOutputItemParam(ToolSearchOutputItemParam {
            call_id: None,
            execution: Some(ToolSearchExecutionType::Server),
            id: None,
            status: Some(if status == ToolResultStatus::Completed {
                FunctionCallItemStatus::Completed
            } else {
                FunctionCallItemStatus::Incomplete
            }),
            tools: discovered_tools,
            r#type: ToolSearchOutputItemParamType::ToolSearchOutput,
        }),
    ))
}

fn convert_to_openai_tool(tool: Tool) -> LanguageModelResult<OpenAITool> {
    match tool {
        Tool::Function(tool) => Ok(OpenAITool::FunctionTool(FunctionTool {
            defer_loading: tool.defer_loading.filter(|deferred| *deferred),
            description: Some(tool.description),
            name: tool.name,
            parameters: Some(convert_json_object(tool.parameters)?),
            strict: Some(true),
            r#type: FunctionToolType::Function,
        })),
        // OpenAI's hosted search has a single algorithm, so strategy is ignored.
        Tool::ToolSearch(_) => Ok(OpenAITool::ToolSearchToolParam(ToolSearchToolParam {
            description: None,
            execution: None,
            parameters: None,
            r#type: ToolSearchToolParamType::ToolSearch,
        })),
        Tool::WebSearch(tool) => {
            let user_location = tool.user_location.map(|location| {
                Some(Some(WebSearchApproximateLocationValue {
                    city: location.city,
                    country: location.country,
                    region: location.region,
                    timezone: location.timezone,
                    r#type: Some(WebSearchApproximateLocationValueType::Approximate),
                }))
            });
            Ok(OpenAITool::WebSearchTool(OpenAIWebSearchTool {
                filters: tool
                    .allowed_domains
                    .map(|allowed_domains| WebSearchToolFilters {
                        allowed_domains: Some(allowed_domains),
                    }),
                search_context_size: None,
                r#type: WebSearchToolType::WebSearch,
                user_location,
            }))
        }
    }
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

fn convert_to_openai_reasoning(value: &ReasoningOptions) -> LanguageModelResult<Reasoning> {
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
    let mut parts = Vec::new();
    // Hosted tool searches have no call_id, so their output is matched to the
    // preceding search call.
    let mut last_tool_search_call_id: Option<String> = None;
    for item in items {
        match item {
            OutputItem::OutputMessage(msg) => {
                parts.extend(map_openai_output_message(msg)?);
            }
            OutputItem::FunctionToolCall(function_tool_call) => {
                let args = serde_json::from_str(&function_tool_call.arguments).map_err(|e| {
                    LanguageModelError::Invariant(
                        PROVIDER,
                        format!("Failed to parse function tool call arguments: {e}"),
                    )
                })?;
                let mut tool_call_part =
                    ToolCallPart::new(function_tool_call.call_id, function_tool_call.name, args);

                tool_call_part.id = function_tool_call.id;
                parts.push(Part::ToolCall(tool_call_part));
            }
            OutputItem::ToolSearchCall(item) => {
                last_tool_search_call_id =
                    Some(item.call_id.clone().unwrap_or_else(|| item.id.clone()));
                parts.push(Part::ToolCall(map_openai_tool_search_call(item)));
            }
            OutputItem::ToolSearchOutput(item) => {
                let tool_call_id = item
                    .call_id
                    .clone()
                    .or_else(|| last_tool_search_call_id.clone());
                parts.push(Part::ToolResult(map_openai_tool_search_output(
                    item,
                    tool_call_id,
                )));
            }
            OutputItem::WebSearchToolCall(web) => {
                parts.extend(map_openai_web_search_call(web));
            }
            OutputItem::ImageGenToolCall(image_gen_call) => {
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
                parts.push(image_part.into());
            }
            OutputItem::ReasoningItem(reasoning_item) => {
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
                parts.push(reasoning_part.into());
            }
            _ => {}
        }
    }
    Ok(parts)
}

fn map_openai_tool_search_status(status: &FunctionCallStatus) -> ToolSearchToolCallStatus {
    match status {
        FunctionCallStatus::InProgress => ToolSearchToolCallStatus::InProgress,
        FunctionCallStatus::Completed => ToolSearchToolCallStatus::Completed,
        FunctionCallStatus::Incomplete | FunctionCallStatus::Unknown => {
            ToolSearchToolCallStatus::Failed
        }
    }
}

/// Hosted tool search arguments are always a JSON object.
fn openai_tool_search_args(arguments: Value) -> Value {
    match arguments {
        Value::Object(_) => arguments,
        _ => Value::Object(serde_json::Map::new()),
    }
}

fn map_openai_tool_search_call(item: ToolSearchCall) -> ToolCallPart {
    let ToolSearchCall {
        arguments,
        call_id,
        id,
        status,
        ..
    } = item;
    ToolCallPart {
        tool_call_id: call_id.unwrap_or_else(|| id.clone()),
        call: ToolCall::ToolSearch(ToolSearchToolCall {
            args: openai_tool_search_args(arguments),
            status: Some(map_openai_tool_search_status(&status)),
        }),
        signature: None,
        id: Some(id),
    }
}

fn map_openai_tool_search_output(
    item: ToolSearchOutput,
    tool_call_id: Option<String>,
) -> ToolResultPart {
    let ToolSearchOutput {
        id, status, tools, ..
    } = item;
    ToolResultPart {
        tool_call_id: tool_call_id.unwrap_or(id),
        result: ToolResult::ToolSearch(ToolSearchToolResult {
            tool_names: map_openai_discovered_tool_names(tools),
            error_code: None,
        }),
        status: if matches!(status, FunctionCallOutputStatusEnum::Incomplete) {
            ToolResultStatus::Failed
        } else {
            ToolResultStatus::Completed
        },
    }
}

fn map_openai_discovered_tool_names(tools: Vec<OpenAITool>) -> Vec<String> {
    tools
        .into_iter()
        .flat_map(|tool| match tool {
            OpenAITool::FunctionTool(function) => vec![function.name],
            OpenAITool::CustomToolParam(custom) => vec![custom.name],
            OpenAITool::NamespaceToolParam(namespace) => namespace
                .tools
                .into_iter()
                .filter_map(|member| match member {
                    NamespaceToolParamToolsItem::Function(function) => Some(function.name),
                    NamespaceToolParamToolsItem::Custom(custom) => Some(custom.name),
                    NamespaceToolParamToolsItem::Unknown => None,
                })
                .collect(),
            _ => vec![],
        })
        .collect()
}

fn map_openai_output_message(msg: OutputMessage) -> LanguageModelResult<Vec<Part>> {
    msg.content.into_iter().try_fold(
        Vec::new(),
        |mut parts, content| -> LanguageModelResult<Vec<Part>> {
            match content {
                OutputMessageContent::OutputText(output_text) => {
                    let citations = output_text
                        .annotations
                        .into_iter()
                        .filter_map(|annotation| match annotation {
                            Annotation::UrlCitation(citation) => {
                                Some(map_openai_url_citation(citation))
                            }
                            _ => None,
                        })
                        .collect::<Vec<_>>();
                    parts.push(Part::Text(TextPart {
                        text: output_text.text,
                        citations: (!citations.is_empty()).then_some(citations),
                        signature: None,
                    }));
                }
                OutputMessageContent::Refusal(refusal) => {
                    return Err(LanguageModelError::Refusal(refusal.refusal));
                }
                OutputMessageContent::Unknown => {}
            }
            Ok(parts)
        },
    )
}

fn map_openai_web_search_call(web: responses_api::WebSearchToolCall) -> Vec<Part> {
    let (action, sources) = map_openai_web_search_action(web.action);
    let status = match web.status {
        responses_api::WebSearchToolCallStatus::InProgress => {
            crate::WebSearchToolCallStatus::InProgress
        }
        responses_api::WebSearchToolCallStatus::Searching => {
            crate::WebSearchToolCallStatus::Searching
        }
        responses_api::WebSearchToolCallStatus::Completed => {
            crate::WebSearchToolCallStatus::Completed
        }
        responses_api::WebSearchToolCallStatus::Failed
        | responses_api::WebSearchToolCallStatus::Unknown => crate::WebSearchToolCallStatus::Failed,
    };
    let id = web.id;
    let mut parts = vec![Part::ToolCall(crate::ToolCallPart {
        tool_call_id: id.clone(),
        call: crate::ToolCall::WebSearch(crate::WebSearchToolCall {
            action,
            status: Some(status),
        }),
        signature: None,
        id: None,
    })];
    if !sources.is_empty() {
        parts.push(Part::ToolResult(crate::ToolResultPart {
            tool_call_id: id,
            result: crate::ToolResult::WebSearch(crate::WebSearchToolResult {
                sources,
                error_code: None,
            }),
            status: ToolResultStatus::Completed,
        }));
    }
    parts
}

#[derive(Default)]
struct OpenAIStreamState {
    /// Hosted tool searches have no `call_id`, so their output is matched to
    /// the preceding search call.
    last_tool_search_call_id: Option<String>,
}

#[allow(clippy::too_many_lines)]
fn map_openai_stream_event(
    event: ResponseStreamEvent,
    state: &mut OpenAIStreamState,
) -> LanguageModelResult<Option<ContentDelta>> {
    match event {
        ResponseStreamEvent::ResponseFailed(failed_event) => {
            let message = failed_event.response.error.flatten().map_or_else(
                || "OpenAI Response Stream failed".to_string(),
                |error| format!("OpenAI Response Stream failed: {}", error.message),
            );
            Err(LanguageModelError::Invariant(PROVIDER, message))
        }
        ResponseStreamEvent::ResponseOutputItemAdded(output_item_added_event) => {
            match output_item_added_event.item {
                OutputItem::FunctionToolCall(function_tool_call) => {
                    let tool_call_part = PartDelta::ToolCall(ToolCallPartDelta {
                        call: crate::ToolCallDelta::Function(crate::FunctionToolCallDelta {
                            args: Some(function_tool_call.arguments),
                            name: Some(function_tool_call.name),
                        }),
                        tool_call_id: Some(function_tool_call.call_id),
                        signature: None,
                        id: function_tool_call.id,
                    });
                    Ok(Some(ContentDelta {
                        index: usize::try_from(output_item_added_event.output_index).unwrap_or(0),
                        part: tool_call_part,
                    }))
                }
                OutputItem::WebSearchToolCall(web) => {
                    let (action, _) = map_openai_web_search_action(web.action);
                    let status = match web.status {
                        responses_api::WebSearchToolCallStatus::InProgress => {
                            crate::WebSearchToolCallStatus::InProgress
                        }
                        responses_api::WebSearchToolCallStatus::Searching => {
                            crate::WebSearchToolCallStatus::Searching
                        }
                        responses_api::WebSearchToolCallStatus::Completed => {
                            crate::WebSearchToolCallStatus::Completed
                        }
                        _ => crate::WebSearchToolCallStatus::Failed,
                    };
                    Ok(Some(ContentDelta {
                        index: usize::try_from(output_item_added_event.output_index).unwrap_or(0),
                        part: PartDelta::ToolCall(ToolCallPartDelta {
                            tool_call_id: Some(web.id),
                            call: crate::ToolCallDelta::WebSearch(crate::WebSearchToolCallDelta {
                                action,
                                status: Some(status),
                            }),
                            signature: None,
                            id: None,
                        }),
                    }))
                }
                OutputItem::ToolSearchCall(item) => {
                    let tool_call_id = item.call_id.unwrap_or_else(|| item.id.clone());
                    state.last_tool_search_call_id = Some(tool_call_id.clone());
                    Ok(Some(ContentDelta {
                        index: usize::try_from(output_item_added_event.output_index).unwrap_or(0),
                        part: PartDelta::ToolCall(ToolCallPartDelta {
                            tool_call_id: Some(tool_call_id),
                            call: ToolCallDelta::ToolSearch(ToolSearchToolCallDelta {
                                args: None,
                                status: Some(map_openai_tool_search_status(&item.status)),
                            }),
                            signature: None,
                            id: Some(item.id),
                        }),
                    }))
                }
                OutputItem::ReasoningItem(reasoning_item) => {
                    if let Some(encrypted_content) = reasoning_item.encrypted_content {
                        let reasoning_part = ReasoningPartDelta {
                            signature: Some(encrypted_content),
                            text: String::new(),
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
        ResponseStreamEvent::ResponseOutputItemDone(output_item_done_event) => {
            let index = usize::try_from(output_item_done_event.output_index).unwrap_or(0);
            match output_item_done_event.item {
                OutputItem::WebSearchToolCall(web) => {
                    let (action, _) = map_openai_web_search_action(web.action);
                    let status = match web.status {
                        responses_api::WebSearchToolCallStatus::InProgress => {
                            crate::WebSearchToolCallStatus::InProgress
                        }
                        responses_api::WebSearchToolCallStatus::Searching => {
                            crate::WebSearchToolCallStatus::Searching
                        }
                        responses_api::WebSearchToolCallStatus::Completed => {
                            crate::WebSearchToolCallStatus::Completed
                        }
                        _ => crate::WebSearchToolCallStatus::Failed,
                    };
                    Ok(Some(ContentDelta {
                        index,
                        part: PartDelta::ToolCall(ToolCallPartDelta {
                            tool_call_id: Some(web.id),
                            call: crate::ToolCallDelta::WebSearch(crate::WebSearchToolCallDelta {
                                action,
                                status: Some(status),
                            }),
                            signature: None,
                            id: None,
                        }),
                    }))
                }
                OutputItem::ToolSearchCall(item) => {
                    // Search arguments arrive whole with the completed item.
                    let tool_call_id = item.call_id.unwrap_or_else(|| item.id.clone());
                    state.last_tool_search_call_id = Some(tool_call_id.clone());
                    Ok(Some(ContentDelta {
                        index,
                        part: PartDelta::ToolCall(ToolCallPartDelta {
                            tool_call_id: Some(tool_call_id),
                            call: ToolCallDelta::ToolSearch(ToolSearchToolCallDelta {
                                args: Some(openai_tool_search_args(item.arguments).to_string()),
                                status: Some(map_openai_tool_search_status(&item.status)),
                            }),
                            signature: None,
                            id: Some(item.id),
                        }),
                    }))
                }
                OutputItem::ToolSearchOutput(item) => {
                    let tool_call_id = item
                        .call_id
                        .clone()
                        .or_else(|| state.last_tool_search_call_id.clone());
                    let result = map_openai_tool_search_output(item, tool_call_id);
                    Ok(Some(ContentDelta {
                        index,
                        part: PartDelta::ToolResult(ToolResultPartDelta {
                            tool_call_id: result.tool_call_id,
                            result: result.result,
                            status: result.status,
                        }),
                    }))
                }
                _ => Ok(None),
            }
        }
        ResponseStreamEvent::ResponseOutputTextDelta(text_delta_event) => {
            let text_part = PartDelta::Text(TextPartDelta {
                text: text_delta_event.delta,
                citation: None,
                signature: None,
            });
            Ok(Some(ContentDelta {
                index: usize::try_from(text_delta_event.output_index).unwrap_or(0),
                part: text_part,
            }))
        }
        ResponseStreamEvent::ResponseOutputTextAnnotationAdded(annotation_event) => {
            // The generated API represents streaming annotations as JSON, so
            // validate the tagged annotation before mapping a citation delta.
            let annotation = serde_json::from_value::<Annotation>(annotation_event.annotation)
                .map_err(|error| {
                    LanguageModelError::Invariant(
                        PROVIDER,
                        format!("Failed to parse OpenAI citation annotation: {error}"),
                    )
                })?;
            if let Annotation::UrlCitation(citation) = annotation {
                let citation = map_openai_url_citation(citation);
                Ok(Some(ContentDelta {
                    index: usize::try_from(annotation_event.output_index).unwrap_or(0),
                    part: PartDelta::Text(TextPartDelta {
                        text: String::new(),
                        citation: Some(CitationDelta {
                            r#type: "citation".to_string(),
                            source: Some(citation.source),
                            title: citation.title,
                            cited_text: citation.cited_text,
                            start_index: citation.start_index,
                            end_index: citation.end_index,
                            signature: citation.signature,
                        }),
                        signature: None,
                    }),
                }))
            } else {
                Ok(None)
            }
        }
        ResponseStreamEvent::ResponseFunctionCallArgumentsDelta(
            function_call_arguments_delta_event,
        ) => {
            let tool_call_part = PartDelta::ToolCall(ToolCallPartDelta {
                call: crate::ToolCallDelta::Function(crate::FunctionToolCallDelta {
                    args: Some(function_call_arguments_delta_event.delta),
                    name: None,
                }),
                ..Default::default()
            });

            Ok(Some(ContentDelta {
                index: usize::try_from(function_call_arguments_delta_event.output_index)
                    .unwrap_or(0),
                part: tool_call_part,
            }))
        }
        ResponseStreamEvent::ResponseWebSearchCallInProgress(event) => {
            Ok(Some(map_openai_web_search_status(
                event.output_index,
                event.item_id,
                crate::WebSearchToolCallStatus::InProgress,
            )))
        }
        ResponseStreamEvent::ResponseWebSearchCallSearching(event) => {
            Ok(Some(map_openai_web_search_status(
                event.output_index,
                event.item_id,
                crate::WebSearchToolCallStatus::Searching,
            )))
        }
        ResponseStreamEvent::ResponseWebSearchCallCompleted(event) => {
            Ok(Some(map_openai_web_search_status(
                event.output_index,
                event.item_id,
                crate::WebSearchToolCallStatus::Completed,
            )))
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
        ResponseStreamEvent::ResponseReasoningTextDelta(reasoning_text_delta_event) => {
            Ok(Some(ContentDelta {
                index: usize::try_from(reasoning_text_delta_event.output_index).unwrap_or(0),
                part: PartDelta::Reasoning(ReasoningPartDelta::new(
                    reasoning_text_delta_event.delta,
                )),
            }))
        }
        ResponseStreamEvent::ResponseReasoningSummaryTextDelta(
            reasoning_summary_text_delta_event,
        ) => {
            let reasoning_part = PartDelta::Reasoning(ReasoningPartDelta::new(
                reasoning_summary_text_delta_event.delta,
            ));
            Ok(Some(ContentDelta {
                index: usize::try_from(reasoning_summary_text_delta_event.output_index)
                    .unwrap_or(0),
                part: reasoning_part,
            }))
        }
        _ => Ok(None),
    }
}

fn map_openai_stream_web_search_result(
    event: &ResponseStreamEvent,
) -> Option<crate::ToolResultPartDelta> {
    let ResponseStreamEvent::ResponseOutputItemDone(done) = event else {
        return None;
    };
    let OutputItem::WebSearchToolCall(web) = &done.item else {
        return None;
    };
    let responses_api::WebSearchToolCallAction::Search(search) = &web.action else {
        return None;
    };
    let sources = search
        .sources
        .as_ref()?
        .iter()
        .map(|source| crate::WebSearchSource {
            url: source.url.clone(),
            title: None,
            page_age: None,
            signature: None,
        })
        .collect::<Vec<_>>();
    if sources.is_empty() {
        return None;
    }
    Some(crate::ToolResultPartDelta {
        tool_call_id: web.id.clone(),
        result: crate::ToolResult::WebSearch(crate::WebSearchToolResult {
            sources,
            error_code: None,
        }),
        status: ToolResultStatus::Completed,
    })
}

fn convert_to_openai_web_search_call(
    tool_call_id: String,
    call: crate::WebSearchToolCall,
) -> LanguageModelResult<InputItem> {
    // Calls without actions cannot be replayed.
    let Some(action) = call.action else {
        return Err(LanguageModelError::InvalidInput(
            "OpenAI web-search history requires an action".to_string(),
        ));
    };
    Ok(InputItem::Item(responses_api::Item::WebSearchToolCall(
        responses_api::WebSearchToolCall {
            action: convert_to_openai_web_search_action(action),
            id: tool_call_id,
            status: match call
                .status
                .unwrap_or(crate::WebSearchToolCallStatus::Completed)
            {
                crate::WebSearchToolCallStatus::InProgress => {
                    responses_api::WebSearchToolCallStatus::InProgress
                }
                crate::WebSearchToolCallStatus::Searching => {
                    responses_api::WebSearchToolCallStatus::Searching
                }
                crate::WebSearchToolCallStatus::Completed => {
                    responses_api::WebSearchToolCallStatus::Completed
                }
                crate::WebSearchToolCallStatus::Failed => {
                    responses_api::WebSearchToolCallStatus::Failed
                }
            },
            r#type: responses_api::WebSearchToolCallType::WebSearchCall,
        },
    )))
}

fn convert_to_openai_web_search_action(
    action: crate::WebSearchAction,
) -> responses_api::WebSearchToolCallAction {
    match action {
        crate::WebSearchAction::Search { queries } => {
            responses_api::WebSearchToolCallAction::Search(responses_api::WebSearchActionSearch {
                queries: Some(queries),
                query: String::new(),
                sources: None,
            })
        }
        crate::WebSearchAction::OpenPage { url } => {
            responses_api::WebSearchToolCallAction::OpenPage(
                responses_api::WebSearchActionOpenPage { url: Some(url) },
            )
        }
        crate::WebSearchAction::FindInPage { url, pattern } => {
            responses_api::WebSearchToolCallAction::FindInPage(responses_api::WebSearchActionFind {
                pattern,
                url,
            })
        }
    }
}

fn map_openai_web_search_action(
    action: responses_api::WebSearchToolCallAction,
) -> (Option<crate::WebSearchAction>, Vec<crate::WebSearchSource>) {
    match action {
        responses_api::WebSearchToolCallAction::Search(search) => {
            let queries = search.queries.unwrap_or_else(|| {
                if search.query.is_empty() {
                    vec![]
                } else {
                    vec![search.query]
                }
            });
            let sources = search
                .sources
                .unwrap_or_default()
                .into_iter()
                .map(|source| crate::WebSearchSource {
                    url: source.url,
                    title: None,
                    page_age: None,
                    signature: None,
                })
                .collect();
            (Some(crate::WebSearchAction::Search { queries }), sources)
        }
        responses_api::WebSearchToolCallAction::OpenPage(open) => (
            open.url.map(|url| crate::WebSearchAction::OpenPage { url }),
            vec![],
        ),
        responses_api::WebSearchToolCallAction::FindInPage(find) => (
            Some(crate::WebSearchAction::FindInPage {
                url: find.url,
                pattern: find.pattern,
            }),
            vec![],
        ),
        responses_api::WebSearchToolCallAction::Unknown => (None, vec![]),
    }
}

fn map_openai_web_search_status(
    index: i64,
    id: String,
    status: crate::WebSearchToolCallStatus,
) -> ContentDelta {
    ContentDelta {
        index: usize::try_from(index).unwrap_or(0),
        part: PartDelta::ToolCall(ToolCallPartDelta {
            tool_call_id: Some(id),
            call: crate::ToolCallDelta::WebSearch(crate::WebSearchToolCallDelta {
                action: None,
                status: Some(status),
            }),
            signature: None,
            id: None,
        }),
    }
}

fn map_openai_url_citation(value: UrlCitationBody) -> Citation {
    Citation {
        source: value.url,
        title: Some(value.title),
        cited_text: None,
        start_index: usize::try_from(value.start_index).ok(),
        end_index: usize::try_from(value.end_index).ok(),
        signature: None,
    }
}

fn openai_image_format_to_mime_type(format: Option<&String>) -> String {
    format!("image/{}", format.map_or("png", String::as_str))
}

fn parse_openai_image_size(size: Option<&String>) -> Option<(u32, u32)> {
    let (width, height) = size?.split_once('x')?;
    Some((width.parse().ok()?, height.parse().ok()?))
}

fn is_billable_web_search(item: &OutputItem) -> bool {
    let OutputItem::WebSearchToolCall(web) = item else {
        return false;
    };
    matches!(
        web.action,
        responses_api::WebSearchToolCallAction::Search(_)
    )
}

fn map_openai_response_usage(value: &ResponseUsage, web_search_requests: usize) -> ModelUsage {
    ModelUsage {
        input_tokens: u32::try_from(value.input_tokens).unwrap_or(0),
        output_tokens: u32::try_from(value.output_tokens).unwrap_or(0),
        input_tokens_details: Some(crate::ModelTokensDetails {
            cached_tokens: Some(
                u32::try_from(value.input_tokens_details.cached_tokens).unwrap_or(0),
            ),
            cache_write_tokens: value
                .input_tokens_details
                .cache_write_tokens
                .map(|tokens| u32::try_from(tokens).unwrap_or(0)),
            ..Default::default()
        }),
        output_tokens_details: Some(crate::ModelTokensDetails {
            reasoning_tokens: Some(
                u32::try_from(value.output_tokens_details.reasoning_tokens).unwrap_or(0),
            ),
            ..Default::default()
        }),
        server_tool_use: (web_search_requests > 0).then(|| ModelServerToolUsage {
            web_search_requests: Some(u32::try_from(web_search_requests).unwrap_or(u32::MAX)),
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
