use crate::{
    anthropic::api::{
        self, Base64ImageSource, Base64ImageSourceMediaType, ContentBlock, ContentBlockDeltaEvent,
        ContentBlockDeltaEventDelta, ContentBlockStartEvent, ContentBlockStartEventContentBlock,
        CreateMessageParams, CreateMessageParamsSystem, CreateMessageParamsToolsItem,
        InputContentBlock, InputMessage, InputMessageContent, InputMessageRole,
        Message as AnthropicMessage, MessageDeltaEvent, MessageDeltaUsage, MessageStartEvent,
        MessageStreamEvent, OutputConfig, RequestCitationsConfig, RequestImageBlock,
        RequestImageBlockSource, RequestSearchResultBlock, RequestTextBlock,
        RequestTextBlockCitationsItem, RequestThinkingBlock, RequestToolResultBlock,
        RequestToolResultBlockContent, RequestToolResultBlockContentArrayItem, RequestToolUseBlock,
        RequestWebSearchResultLocationCitation, StopReason, ThinkingConfigAdaptive,
        ThinkingConfigDisabled, ThinkingConfigEnabled, ThinkingConfigParam, Tool, Usage,
        UserLocation, WebSearchTool20250305,
    },
    client_utils, stream_utils,
    tool_result_utils::CANCELLED_TOOL_RESULT_FALLBACK_CONTENT,
    Citation, CitationDelta, ContentDelta, ImagePart, LanguageModel, LanguageModelError,
    LanguageModelInput, LanguageModelMetadata, LanguageModelResult, LanguageModelStream, Message,
    ModelResponse, ModelUsage, Part, PartDelta, PartialModelResponse, ReasoningOptions,
    ReasoningPart, ReasoningPartDelta, ResponseFormatJson, ResponseFormatOption, TextPart,
    TextPartDelta, Tool as SdkTool, ToolCallPart, ToolCallPartDelta, ToolChoiceOption,
    ToolResultPart, ToolResultStatus,
};
use async_stream::try_stream;
use futures::{future::BoxFuture, StreamExt};
use reqwest::{
    header::{HeaderMap, HeaderName, HeaderValue},
    Client,
};
use serde_json::{Map, Value};
use std::{
    collections::{HashMap, HashSet},
    sync::Arc,
};

const PROVIDER: &str = "anthropic";
const DEFAULT_BASE_URL: &str = "https://api.anthropic.com";
const DEFAULT_API_VERSION: &str = "2023-06-01";

pub struct AnthropicModel {
    model_id: String,
    api_key: String,
    base_url: String,
    api_version: String,
    client: Client,
    metadata: Option<Arc<LanguageModelMetadata>>,
    headers: HashMap<String, String>,
}

#[derive(Clone, Default)]
pub struct AnthropicModelOptions {
    pub base_url: Option<String>,
    pub api_key: String,
    pub api_version: Option<String>,
    pub headers: Option<HashMap<String, String>>,
    pub client: Option<Client>,
}

impl AnthropicModel {
    #[must_use]
    pub fn new(model_id: impl Into<String>, mut options: AnthropicModelOptions) -> Self {
        let base_url = options
            .base_url
            .take()
            .unwrap_or_else(|| DEFAULT_BASE_URL.to_string())
            .trim_end_matches('/')
            .to_string();

        let api_version = options
            .api_version
            .take()
            .unwrap_or_else(|| DEFAULT_API_VERSION.to_string());

        let client = options.client.take().unwrap_or_default();

        let headers = options.headers.unwrap_or_default();

        Self {
            model_id: model_id.into(),
            api_key: options.api_key,
            base_url,
            api_version,
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

        headers.insert(
            "x-api-key",
            HeaderValue::from_str(&self.api_key).map_err(|error| {
                LanguageModelError::InvalidInput(format!(
                    "Invalid Anthropic API key header value: {error}"
                ))
            })?,
        );
        headers.insert(
            "anthropic-version",
            HeaderValue::from_str(&self.api_version).map_err(|error| {
                LanguageModelError::InvalidInput(format!(
                    "Invalid Anthropic version header value: {error}"
                ))
            })?,
        );

        for (key, value) in &self.headers {
            let header_name = HeaderName::from_bytes(key.as_bytes()).map_err(|error| {
                LanguageModelError::InvalidInput(format!(
                    "Invalid Anthropic header name '{key}': {error}"
                ))
            })?;
            let header_value = HeaderValue::from_str(value).map_err(|error| {
                LanguageModelError::InvalidInput(format!(
                    "Invalid Anthropic header value for '{key}': {error}"
                ))
            })?;
            headers.insert(header_name, header_value);
        }

        Ok(headers)
    }
}

impl LanguageModel for AnthropicModel {
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
                &self.model_id,
                input,
                |input| async move {
                    let params = convert_to_anthropic_create_params(input, &self.model_id, false)?;

                    let headers = self.request_headers()?;

                    let response: AnthropicMessage = client_utils::send_json(
                        &self.client,
                        &format!("{}/v1/messages", self.base_url),
                        &params,
                        headers,
                    )
                    .await?;

                    if matches!(response.stop_reason, Some(StopReason::Refusal)) {
                        return Err(LanguageModelError::Refusal(anthropic_refusal_message(
                            response.stop_details.as_ref(),
                        )));
                    }

                    let content = map_anthropic_message(response.content);
                    let usage = Some(map_anthropic_usage(&response.usage));

                    let cost =
                        if let (Some(usage), Some(metadata)) = (usage.as_ref(), self.metadata()) {
                            metadata
                                .pricing
                                .as_ref()
                                .map(|pricing| usage.calculate_cost(pricing))
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

    #[allow(clippy::too_many_lines)]
    fn stream(
        &self,
        input: LanguageModelInput,
    ) -> BoxFuture<'_, LanguageModelResult<LanguageModelStream>> {
        Box::pin(async move {
            crate::opentelemetry::trace_stream(
                self.provider(),
                &self.model_id,
                input,
                |input| async move {
                    let params = convert_to_anthropic_create_params(input, &self.model_id, true)?;

                    let headers = self.request_headers()?;
                    let mut chunk_stream = client_utils::send_sse_stream::<_, MessageStreamEvent>(
                        &self.client,
                        &format!("{}/v1/messages", self.base_url),
                        &params,
                        headers,
                        self.provider(),
                    )
                    .await?;

                    let metadata = self.metadata.clone();

                    let stream = try_stream! {
                        let mut provider_tool_block_indexes = HashSet::new();
                        let mut server_tool_blocks = HashMap::<i64, (String, String)>::new();
                        let mut server_tool_call_indexes = HashMap::new();
                        while let Some(event) = chunk_stream.next().await {
                            match event? {
                                MessageStreamEvent::MessageStart(MessageStartEvent { message }) => {
                                    let usage = map_anthropic_usage(&message.usage);
                                    let cost = metadata
                                        .as_ref()
                                        .and_then(|meta| meta.pricing.as_ref())
                                        .map(|pricing| usage.calculate_cost(pricing));

                                    yield PartialModelResponse {
                                        delta: None,
                                        usage: Some(usage),
                                        cost,
                                    };
                                    if matches!(message.stop_reason, Some(StopReason::Refusal)) {
                                        Err(LanguageModelError::Refusal(anthropic_refusal_message(
                                            message.stop_details.as_ref(),
                                        )))?;
                                    }
                                }
                                MessageStreamEvent::MessageDelta(MessageDeltaEvent { delta, usage }) => {
                                    let usage = map_anthropic_message_delta_usage(&usage);
                                    let cost = metadata
                                        .as_ref()
                                        .and_then(|meta| meta.pricing.as_ref())
                                        .map(|pricing| usage.calculate_cost(pricing));

                                    yield PartialModelResponse {
                                        delta: None,
                                        usage: Some(usage),
                                        cost,
                                    };
                                    if matches!(delta.stop_reason, Some(StopReason::Refusal)) {
                                        Err(LanguageModelError::Refusal(anthropic_refusal_message(
                                            delta.stop_details.as_ref(),
                                        )))?;
                                    }
                                }
                                MessageStreamEvent::ContentBlockStart(ContentBlockStartEvent { content_block, index }) => {
                                    if let ContentBlockStartEventContentBlock::ServerToolUse(block) = &content_block {
                                        provider_tool_block_indexes.insert(index);
                                        server_tool_call_indexes.insert(block.id.clone(), index);
                                        if matches!(block.name, api::ResponseServerToolUseBlockName::WebSearch) {
                                            server_tool_blocks.insert(index, (block.id.clone(), String::new()));
                                        }
                                    }
                                    if let ContentBlockStartEventContentBlock::WebSearchToolResult(block) = &content_block {
                                        if let Some(call_index) = server_tool_call_indexes.get(&block.tool_use_id) {
                                            yield PartialModelResponse {
                                                delta: Some(ContentDelta {
                                                    index: usize::try_from(*call_index).map_err(|_| {
                                                        LanguageModelError::Invariant(
                                                            PROVIDER,
                                                            format!("Anthropic stream content block index out of range: {call_index}"),
                                                        )
                                                    })?,
                                                    part: PartDelta::ToolCall(ToolCallPartDelta {
                                                        tool_call_id: None,
                                                        call: crate::ToolCallDelta::WebSearch(
                                                            crate::WebSearchToolCallDelta {
                                                                action: None,
                                                                status: Some(anthropic_web_search_result_status(&block.content)),
                                                            },
                                                        ),
                                                        signature: None,
                                                        id: None,
                                                    }),
                                                }),
                                                ..Default::default()
                                            };
                                        }
                                    }
                                    let deltas = map_anthropic_content_block_start_event(
                                        content_block,
                                        usize::try_from(index).map_err(|_| {
                                            LanguageModelError::Invariant(
                                                PROVIDER,
                                                format!(
                                                    "Anthropic stream content block index out of range: {index}"
                                                ),
                                            )
                                        })?,
                                    )?;
                                    for delta in deltas {
                                        yield PartialModelResponse {
                                            delta: Some(delta),
                                            ..Default::default()
                                        };
                                    }
                                }
                                MessageStreamEvent::ContentBlockDelta(ContentBlockDeltaEvent { delta, index }) => {
                                    if let Some((_, input)) = server_tool_blocks.get_mut(&index) {
                                        if let ContentBlockDeltaEventDelta::InputJsonDelta(input_delta) = &delta {
                                            input.push_str(&input_delta.partial_json);
                                            continue;
                                        }
                                    }
                                    if provider_tool_block_indexes.contains(&index) {
                                        continue;
                                    }
                                    if let Some(delta) = map_anthropic_content_block_delta_event(
                                        delta,
                                        usize::try_from(index).map_err(|_| {
                                            LanguageModelError::Invariant(
                                                PROVIDER,
                                                format!(
                                                    "Anthropic stream content block index out of range: {index}"
                                                ),
                                            )
                                        })?,
                                    ) {
                                        yield PartialModelResponse {
                                            delta: Some(delta),
                                            ..Default::default()
                                        };
                                    }
                                }
                                MessageStreamEvent::ContentBlockStop(event) => {
                                    if let Some((id, input)) = server_tool_blocks.remove(&event.index) {
                                        let query = serde_json::from_str::<Value>(&input)
                                            .ok()
                                            .and_then(|value| value.get("query")?.as_str().map(str::to_owned));
                                        if let Some(query) = query {
                                            yield PartialModelResponse {
                                                delta: Some(ContentDelta {
                                                    index: usize::try_from(event.index).map_err(|_| {
                                                        LanguageModelError::Invariant(
                                                            PROVIDER,
                                                            format!("Anthropic stream content block index out of range: {}", event.index),
                                                        )
                                                    })?,
                                                    part: PartDelta::ToolCall(ToolCallPartDelta {
                                                        tool_call_id: Some(id),
                                                        call: crate::ToolCallDelta::WebSearch(
                                                            crate::WebSearchToolCallDelta {
                                                                action: Some(crate::WebSearchAction::Search {
                                                                    queries: vec![query],
                                                                }),
                                                                status: None,
                                                            },
                                                        ),
                                                        signature: None,
                                                        id: None,
                                                    }),
                                                }),
                                                ..Default::default()
                                            };
                                        }
                                    }
                                }
                                _ => {}
                            }
                        }
                    };

                    Ok(LanguageModelStream::from_stream(stream))
                },
            )
            .await
        })
    }
}

fn anthropic_refusal_message(details: Option<&api::RefusalStopDetails>) -> String {
    details
        .and_then(|details| {
            details.explanation.clone().or_else(|| {
                details
                    .category
                    .as_ref()
                    .map(|_| "Anthropic policy category refusal".to_string())
            })
        })
        .unwrap_or_else(|| "Anthropic refused the request".to_string())
}

fn convert_to_anthropic_create_params(
    input: LanguageModelInput,
    model_id: &str,
    stream: bool,
) -> LanguageModelResult<CreateMessageParams> {
    let LanguageModelInput {
        system_prompt,
        messages,
        tools,
        tool_choice,
        response_format,
        max_tokens,
        temperature,
        top_p,
        top_k,
        presence_penalty: _,
        frequency_penalty: _,
        seed: _,
        modalities: _,
        metadata: _,
        audio: _,
        reasoning,
    } = input;

    let max_tokens = i64::from(max_tokens.unwrap_or(4096));

    let message_params = convert_to_anthropic_messages(messages)?;

    let params = CreateMessageParams {
        cache_control: None,
        container: None,
        inference_geo: None,
        max_tokens,
        messages: message_params,
        metadata: None,
        model: Some(model_id.to_string()),
        output_config: response_format.and_then(convert_to_anthropic_output_config),
        service_tier: None,
        stop_sequences: None,
        stream: Some(stream),
        system: system_prompt
            .map(|prompt| CreateMessageParamsSystem::CreateMessageParamsSystemString(Some(prompt))),
        temperature,
        thinking: reasoning.map(|options| convert_to_anthropic_thinking_config(&options)),
        tool_choice: tool_choice.map(convert_to_anthropic_tool_choice),
        tools: tools.map(|tool_list| tool_list.into_iter().map(convert_tool).collect()),
        top_k: top_k.map(i64::from),
        top_p,
    };

    Ok(params)
}

fn convert_tool(tool: SdkTool) -> CreateMessageParamsToolsItem {
    match tool {
        SdkTool::Function(tool) => CreateMessageParamsToolsItem::Tool(Tool {
            allowed_callers: None,
            name: tool.name,
            description: Some(tool.description),
            input_schema: Some(tool.parameters),
            cache_control: None,
            defer_loading: None,
            eager_input_streaming: None,
            input_examples: None,
            strict: Some(true),
            r#type: None,
        }),
        SdkTool::WebSearch(tool) => CreateMessageParamsToolsItem::WebSearchTool20250305(
            // The basic version supports both common options without enabling
            // Anthropic's newer code-execution filtering flow.
            WebSearchTool20250305 {
                allowed_callers: None,
                allowed_domains: tool.allowed_domains,
                blocked_domains: None,
                cache_control: None,
                defer_loading: None,
                max_uses: tool.max_uses.map(i64::from),
                name: "web_search".to_string(),
                strict: None,
                r#type: "web_search_20250305".to_string(),
                user_location: tool.user_location.map(|location| UserLocation {
                    city: location.city,
                    country: location.country,
                    region: location.region,
                    timezone: location.timezone,
                    r#type: "approximate".to_string(),
                }),
            },
        ),
    }
}

fn convert_to_anthropic_output_config(
    response_format: ResponseFormatOption,
) -> Option<OutputConfig> {
    match response_format {
        ResponseFormatOption::Text => None,
        ResponseFormatOption::Json(ResponseFormatJson { schema, .. }) => {
            schema.map(|schema| OutputConfig {
                effort: None,
                format: Some(api::JsonOutputFormat {
                    schema,
                    r#type: "json_schema".to_string(),
                }),
            })
        }
    }
}

fn convert_to_anthropic_messages(messages: Vec<Message>) -> LanguageModelResult<Vec<InputMessage>> {
    messages
        .into_iter()
        .map(|message| match message {
            Message::User(user) => convert_message_parts_to_input_message("user", user.content),
            Message::Assistant(assistant) => {
                convert_message_parts_to_input_message("assistant", assistant.content)
            }
            Message::Tool(tool) => convert_message_parts_to_input_message("user", tool.content),
        })
        .collect()
}

fn convert_message_parts_to_input_message(
    role: &str,
    parts: Vec<Part>,
) -> LanguageModelResult<InputMessage> {
    let content_blocks = convert_parts_to_content_blocks(parts)?;
    Ok(InputMessage {
        content: InputMessageContent::InputMessageContentArray(Some(content_blocks)),
        role: match role {
            "user" => InputMessageRole::User,
            "assistant" => InputMessageRole::Assistant,
            _ => {
                return Err(LanguageModelError::InvalidInput(format!(
                    "Unsupported Anthropic message role: {role}"
                )))
            }
        },
    })
}

fn convert_parts_to_content_blocks(
    parts: Vec<Part>,
) -> LanguageModelResult<Vec<InputContentBlock>> {
    parts
        .into_iter()
        .map(convert_part_to_content_block)
        .collect()
}

fn convert_part_to_content_block(part: Part) -> LanguageModelResult<InputContentBlock> {
    match part {
        Part::Text(text_part) => Ok(InputContentBlock::Text(create_request_text_block(
            text_part,
        ))),
        Part::Image(image_part) => Ok(InputContentBlock::Image(create_request_image_block(
            image_part,
        )?)),
        Part::Source(source_part) => Ok(InputContentBlock::SearchResult(convert_source_part(
            source_part,
        )?)),
        Part::ToolCall(tool_call) => match tool_call.call {
            crate::ToolCall::Function(call) => {
                Ok(InputContentBlock::ToolUse(RequestToolUseBlock {
                    cache_control: None,
                    caller: None,
                    id: tool_call.tool_call_id,
                    input: normalize_tool_args(call.args)?,
                    name: call.name,
                }))
            }
            crate::ToolCall::WebSearch(call) => {
                let input = match call.action {
                    Some(crate::WebSearchAction::Search { queries }) => {
                        serde_json::json!({"query": queries.into_iter().next().unwrap_or_default()})
                    }
                    _ => serde_json::json!({}),
                };
                Ok(InputContentBlock::ServerToolUse(
                    api::RequestServerToolUseBlock {
                        cache_control: None,
                        caller: None,
                        id: tool_call.tool_call_id,
                        input,
                        name: api::RequestServerToolUseBlockName::WebSearch,
                    },
                ))
            }
        },
        Part::ToolResult(tool_result) => match tool_result.result {
            crate::ToolResult::Function(result) => Ok(InputContentBlock::ToolResult(
                convert_tool_result_part(ToolResultPart {
                    tool_call_id: tool_result.tool_call_id,
                    result: crate::ToolResult::Function(result),
                    status: tool_result.status,
                })?,
            )),
            crate::ToolResult::WebSearch(result) => {
                let content = if let Some(code) = result.error_code {
                    api::RequestWebSearchToolResultBlockContent::RequestWebSearchToolResultError(
                        api::RequestWebSearchToolResultError {
                            error_code: match code.as_str() {
                                "unavailable" => api::WebSearchToolResultErrorCode::Unavailable,
                                "max_uses_exceeded" => {
                                    api::WebSearchToolResultErrorCode::MaxUsesExceeded
                                }
                                "too_many_requests" => {
                                    api::WebSearchToolResultErrorCode::TooManyRequests
                                }
                                "query_too_long" => api::WebSearchToolResultErrorCode::QueryTooLong,
                                "request_too_large" => {
                                    api::WebSearchToolResultErrorCode::RequestTooLarge
                                }
                                _ => api::WebSearchToolResultErrorCode::InvalidToolInput,
                            },
                            r#type: "web_search_tool_result_error".to_string(),
                        },
                    )
                } else {
                    api::RequestWebSearchToolResultBlockContent::RequestWebSearchToolResultBlockContentArray(Some(result.sources.into_iter().map(|source| api::RequestWebSearchResultBlock { encrypted_content: source.signature.unwrap_or_default(), page_age: source.page_age, title: source.title.unwrap_or_default(), r#type: "web_search_result".to_string(), url: source.url }).collect()))
                };
                Ok(InputContentBlock::WebSearchToolResult(
                    api::RequestWebSearchToolResultBlock {
                        cache_control: None,
                        caller: None,
                        content,
                        tool_use_id: tool_result.tool_call_id,
                    },
                ))
            }
        },
        Part::Reasoning(reasoning_part) => Ok(convert_reasoning_part(reasoning_part)),
        Part::Audio(_) => Err(LanguageModelError::Unsupported(
            PROVIDER,
            "Anthropic does not support audio parts".to_string(),
        )),
    }
}

fn convert_reasoning_part(reasoning_part: ReasoningPart) -> InputContentBlock {
    if reasoning_part.text.is_empty() && reasoning_part.signature.is_some() {
        return InputContentBlock::RedactedThinking(api::RequestRedactedThinkingBlock {
            data: reasoning_part.signature.unwrap_or_default(),
        });
    }

    InputContentBlock::Thinking(RequestThinkingBlock {
        thinking: reasoning_part.text,
        signature: reasoning_part.signature.unwrap_or_default(),
    })
}

fn convert_tool_result_part(
    tool_result: ToolResultPart,
) -> LanguageModelResult<RequestToolResultBlock> {
    let mut content_blocks = Vec::new();
    let crate::ToolResult::Function(result) = tool_result.result else {
        return Err(LanguageModelError::Unsupported(
            PROVIDER,
            "Expected function tool result".to_string(),
        ));
    };
    for part in result.content {
        let block = convert_part_to_tool_result_content_block(part)?;
        content_blocks.push(block);
    }

    let content = if content_blocks.is_empty() {
        match tool_result.status {
            ToolResultStatus::Completed | ToolResultStatus::Failed => None,
            ToolResultStatus::Cancelled => Some(
                RequestToolResultBlockContent::RequestToolResultBlockContentString(Some(
                    CANCELLED_TOOL_RESULT_FALLBACK_CONTENT.to_string(),
                )),
            ),
        }
    } else {
        Some(
            RequestToolResultBlockContent::RequestToolResultBlockContentArray(Some(content_blocks)),
        )
    };

    Ok(RequestToolResultBlock {
        cache_control: None,
        content,
        is_error: (tool_result.status != ToolResultStatus::Completed).then_some(true),
        tool_use_id: tool_result.tool_call_id,
    })
}

fn convert_part_to_tool_result_content_block(
    part: Part,
) -> LanguageModelResult<RequestToolResultBlockContentArrayItem> {
    match part {
        Part::Text(text_part) => Ok(RequestToolResultBlockContentArrayItem::Text(
            create_request_text_block(text_part),
        )),
        Part::Image(image_part) => Ok(RequestToolResultBlockContentArrayItem::Image(
            create_request_image_block(image_part)?,
        )),
        Part::Source(source_part) => Ok(RequestToolResultBlockContentArrayItem::SearchResult(
            convert_source_part(source_part)?,
        )),
        _ => Err(LanguageModelError::Unsupported(
            PROVIDER,
            "Cannot convert tool result part to Anthropic content".to_string(),
        )),
    }
}

fn create_request_text_block(text_part: TextPart) -> RequestTextBlock {
    let citations = text_part.citations.and_then(|citations| {
        let citations = citations
            .into_iter()
            .filter_map(|citation| {
                Some(RequestTextBlockCitationsItem::WebSearchResultLocation(
                    RequestWebSearchResultLocationCitation {
                        cited_text: citation.cited_text.unwrap_or_default(),
                        encrypted_index: citation.signature?,
                        title: citation.title,
                        url: citation.source,
                    },
                ))
            })
            .collect::<Vec<_>>();
        (!citations.is_empty()).then_some(citations)
    });

    RequestTextBlock {
        cache_control: None,
        // encrypted_index is the provider state Anthropic accepts when a
        // web-search citation is returned in a later assistant message.
        citations,
        text: text_part.text,
        r#type: "text".to_string(),
    }
}

fn create_request_image_block(image_part: ImagePart) -> LanguageModelResult<RequestImageBlock> {
    Ok(RequestImageBlock {
        cache_control: None,
        source: RequestImageBlockSource::Base64(Base64ImageSource {
            data: image_part.data,
            media_type: map_anthropic_image_media_type(&image_part.mime_type)?,
        }),
    })
}

fn convert_source_part(
    source_part: crate::SourcePart,
) -> LanguageModelResult<RequestSearchResultBlock> {
    let mut content = Vec::new();
    for part in source_part.content {
        match part {
            Part::Text(text_part) => content.push(create_request_text_block(text_part)),
            _ => {
                return Err(LanguageModelError::Unsupported(
                    PROVIDER,
                    "Anthropic source part only supports text content".to_string(),
                ))
            }
        }
    }

    Ok(RequestSearchResultBlock {
        cache_control: None,
        citations: Some(RequestCitationsConfig {
            enabled: Some(true),
        }),
        content,
        source: source_part.source,
        title: source_part.title,
    })
}

fn normalize_tool_args(args: Value) -> LanguageModelResult<Value> {
    match args {
        Value::Object(_) => Ok(args),
        Value::Null => Ok(Value::Object(Map::new())),
        _ => Err(LanguageModelError::InvalidInput(
            "Anthropic tool call arguments must be a JSON object".to_string(),
        )),
    }
}

fn convert_to_anthropic_tool_choice(choice: ToolChoiceOption) -> api::ToolChoice {
    match choice {
        ToolChoiceOption::Auto => api::ToolChoice::Auto(api::ToolChoiceAuto {
            disable_parallel_tool_use: None,
        }),
        ToolChoiceOption::None => api::ToolChoice::None(api::ToolChoiceNone {}),
        ToolChoiceOption::Required => api::ToolChoice::Any(api::ToolChoiceAny {
            disable_parallel_tool_use: None,
        }),
        ToolChoiceOption::Tool(tool) => api::ToolChoice::Tool(api::ToolChoiceTool {
            disable_parallel_tool_use: None,
            name: tool.tool_name,
        }),
    }
}

fn convert_to_anthropic_thinking_config(reasoning: &ReasoningOptions) -> ThinkingConfigParam {
    if !reasoning.enabled {
        return ThinkingConfigParam::Disabled(ThinkingConfigDisabled {});
    }

    // Without an explicit token budget, let Anthropic choose the thinking depth.
    let Some(budget_tokens) = reasoning.budget_tokens else {
        return ThinkingConfigParam::Adaptive(ThinkingConfigAdaptive::default());
    };

    ThinkingConfigParam::Enabled(ThinkingConfigEnabled {
        budget_tokens: i64::from(budget_tokens),
        display: None,
    })
}

fn map_anthropic_message(content: Vec<ContentBlock>) -> Vec<Part> {
    let mut parts = Vec::new();
    let call_statuses: HashMap<String, crate::WebSearchToolCallStatus> = content
        .iter()
        .filter_map(|block| match block {
            ContentBlock::WebSearchToolResult(result) => Some((
                result.tool_use_id.clone(),
                anthropic_web_search_result_status(&result.content),
            )),
            _ => None,
        })
        .collect();
    for block in content {
        if let Some(part) = map_content_block(block) {
            let mut part = part;
            if let Part::ToolCall(call) = &mut part {
                if let Some(status) = call_statuses.get(&call.tool_call_id) {
                    if let crate::ToolCall::WebSearch(web) = &mut call.call {
                        web.status = Some(status.clone());
                    }
                }
            }
            parts.push(part);
        }
    }
    parts
}

fn anthropic_web_search_result_status(
    content: &api::ResponseWebSearchToolResultBlockContent,
) -> crate::WebSearchToolCallStatus {
    if matches!(
        content,
        api::ResponseWebSearchToolResultBlockContent::ResponseWebSearchToolResultError(_)
    ) {
        crate::WebSearchToolCallStatus::Failed
    } else {
        crate::WebSearchToolCallStatus::Completed
    }
}

fn map_content_block(block: ContentBlock) -> Option<Part> {
    match block {
        ContentBlock::Text(text_block) => Some(Part::Text(map_text_block(text_block))),
        ContentBlock::Thinking(thinking_block) => {
            Some(Part::Reasoning(map_thinking_block(thinking_block)))
        }
        ContentBlock::RedactedThinking(redacted_block) => {
            Some(Part::Reasoning(map_redacted_thinking_block(redacted_block)))
        }
        ContentBlock::ToolUse(tool_use) => Some(Part::ToolCall(map_tool_use_block(tool_use))),
        ContentBlock::ServerToolUse(block)
            if matches!(block.name, api::ResponseServerToolUseBlockName::WebSearch) =>
        {
            let action = block
                .input
                .get("query")
                .and_then(Value::as_str)
                .map(|query| crate::WebSearchAction::Search {
                    queries: vec![query.to_string()],
                });
            Some(Part::ToolCall(ToolCallPart {
                tool_call_id: block.id,
                call: crate::ToolCall::WebSearch(crate::WebSearchToolCall {
                    action,
                    status: Some(crate::WebSearchToolCallStatus::InProgress),
                }),
                signature: None,
                id: None,
            }))
        }
        ContentBlock::WebSearchToolResult(block) => {
            let (sources, error_code) = match block.content {
                api::ResponseWebSearchToolResultBlockContent::ResponseWebSearchToolResultBlockContentArray(values) => (values.unwrap_or_default().into_iter().map(|source| crate::WebSearchSource { url: source.url, title: Some(source.title), page_age: source.page_age, signature: Some(source.encrypted_content) }).collect(), None),
                api::ResponseWebSearchToolResultBlockContent::ResponseWebSearchToolResultError(error) => (vec![], Some(match error.error_code {
                    api::WebSearchToolResultErrorCode::InvalidToolInput => "invalid_tool_input", api::WebSearchToolResultErrorCode::Unavailable => "unavailable", api::WebSearchToolResultErrorCode::MaxUsesExceeded => "max_uses_exceeded", api::WebSearchToolResultErrorCode::TooManyRequests => "too_many_requests", api::WebSearchToolResultErrorCode::QueryTooLong => "query_too_long", api::WebSearchToolResultErrorCode::RequestTooLarge => "request_too_large", api::WebSearchToolResultErrorCode::Unknown => "unknown",
                }.to_string())),
                api::ResponseWebSearchToolResultBlockContent::Unknown(_) => (vec![], None),
            };
            let status = if error_code.is_some() {
                ToolResultStatus::Failed
            } else {
                ToolResultStatus::Completed
            };
            Some(Part::ToolResult(ToolResultPart {
                tool_call_id: block.tool_use_id,
                result: crate::ToolResult::WebSearch(crate::WebSearchToolResult {
                    sources,
                    error_code,
                }),
                status,
            }))
        }
        _ => None,
    }
}

fn map_text_block(block: api::ResponseTextBlock) -> TextPart {
    let citations = map_text_citations(block.citations);
    TextPart {
        text: block.text,
        citations,
        signature: None,
    }
}

fn map_text_citations(
    citations: Option<Vec<api::ResponseTextBlockCitationsItem>>,
) -> Option<Vec<Citation>> {
    let citations = citations?;

    let mut results = Vec::new();

    for citation in citations {
        match citation {
            api::ResponseTextBlockCitationsItem::SearchResultLocation(
                api::ResponseSearchResultLocationCitation {
                    cited_text,
                    end_block_index,
                    search_result_index: _,
                    source,
                    start_block_index,
                    title,
                },
            ) => {
                if source.is_empty() {
                    continue;
                }

                let mapped = Citation {
                    source,
                    title,
                    cited_text: if cited_text.is_empty() {
                        None
                    } else {
                        Some(cited_text)
                    },
                    start_index: usize::try_from(start_block_index).ok(),
                    end_index: usize::try_from(end_block_index).ok(),
                    signature: None,
                };

                results.push(mapped);
            }
            api::ResponseTextBlockCitationsItem::WebSearchResultLocation(citation)
                if !citation.url.is_empty() =>
            {
                results.push(Citation {
                    source: citation.url,
                    title: citation.title,
                    cited_text: (!citation.cited_text.is_empty()).then_some(citation.cited_text),
                    start_index: None,
                    end_index: None,
                    signature: Some(citation.encrypted_index),
                });
            }
            _ => {}
        }
    }

    if results.is_empty() {
        None
    } else {
        Some(results)
    }
}

fn map_thinking_block(block: api::ResponseThinkingBlock) -> ReasoningPart {
    ReasoningPart {
        text: block.thinking,
        signature: if block.signature.is_empty() {
            None
        } else {
            Some(block.signature)
        },
        id: None,
    }
}

fn map_redacted_thinking_block(block: api::ResponseRedactedThinkingBlock) -> ReasoningPart {
    ReasoningPart {
        text: String::new(),
        signature: Some(block.data),
        id: None,
    }
}

fn map_tool_use_block(block: api::ResponseToolUseBlock) -> ToolCallPart {
    ToolCallPart {
        tool_call_id: block.id,
        call: crate::ToolCall::Function(crate::FunctionToolCall {
            name: block.name,
            args: block.input,
        }),
        signature: None,
        id: None,
    }
}

fn map_anthropic_usage(usage: &Usage) -> ModelUsage {
    ModelUsage {
        input_tokens: u32::try_from(usage.input_tokens).unwrap_or(0),
        output_tokens: u32::try_from(usage.output_tokens).unwrap_or(0),
        ..Default::default()
    }
}

fn map_anthropic_message_delta_usage(usage: &MessageDeltaUsage) -> ModelUsage {
    ModelUsage {
        input_tokens: usage
            .input_tokens
            .and_then(|value| u32::try_from(value).ok())
            .unwrap_or(0),
        output_tokens: u32::try_from(usage.output_tokens).unwrap_or(0),
        ..Default::default()
    }
}

fn map_anthropic_content_block_start_event(
    content_block: ContentBlockStartEventContentBlock,
    index: usize,
) -> LanguageModelResult<Vec<ContentDelta>> {
    let Some(content_block) = map_start_content_block(content_block) else {
        return Ok(vec![]);
    };

    if let Some(part) = map_content_block(content_block) {
        let mut delta = stream_utils::loosely_convert_part_to_part_delta(part)?;
        if let PartDelta::ToolCall(tool_call_delta) = &mut delta {
            if let crate::ToolCallDelta::Function(call) = &mut tool_call_delta.call {
                call.args = Some(String::new());
            }
        }
        Ok(vec![ContentDelta { index, part: delta }])
    } else {
        Ok(vec![])
    }
}

fn map_anthropic_content_block_delta_event(
    delta: ContentBlockDeltaEventDelta,
    index: usize,
) -> Option<ContentDelta> {
    let part_delta = match delta {
        ContentBlockDeltaEventDelta::TextDelta(delta) => PartDelta::Text(TextPartDelta {
            text: delta.text,
            citation: None,
            signature: None,
        }),
        ContentBlockDeltaEventDelta::InputJsonDelta(delta) => {
            PartDelta::ToolCall(ToolCallPartDelta {
                call: crate::ToolCallDelta::Function(crate::FunctionToolCallDelta {
                    name: None,
                    args: Some(delta.partial_json),
                }),
                tool_call_id: None,
                signature: None,
                id: None,
            })
        }
        ContentBlockDeltaEventDelta::ThinkingDelta(delta) => {
            PartDelta::Reasoning(ReasoningPartDelta {
                text: Some(delta.thinking),
                signature: None,
                id: None,
            })
        }
        ContentBlockDeltaEventDelta::SignatureDelta(delta) => {
            PartDelta::Reasoning(ReasoningPartDelta {
                text: None,
                signature: Some(delta.signature),
                id: None,
            })
        }
        ContentBlockDeltaEventDelta::CitationsDelta(delta) => {
            let citation = map_citation_delta(delta.citation)?;
            PartDelta::Text(TextPartDelta {
                text: String::new(),
                citation: Some(citation),
                signature: None,
            })
        }
        ContentBlockDeltaEventDelta::Unknown => return None,
    };

    Some(ContentDelta {
        index,
        part: part_delta,
    })
}

fn map_citation_delta(citation: api::CitationsDeltaCitation) -> Option<CitationDelta> {
    match citation {
        api::CitationsDeltaCitation::SearchResultLocation(
            api::ResponseSearchResultLocationCitation {
                cited_text,
                end_block_index,
                search_result_index: _,
                source,
                start_block_index,
                title,
            },
        ) => Some(CitationDelta {
            r#type: "citation".to_string(),
            source: Some(source),
            title,
            cited_text: if cited_text.is_empty() {
                None
            } else {
                Some(cited_text)
            },
            start_index: usize::try_from(start_block_index).ok(),
            end_index: usize::try_from(end_block_index).ok(),
            signature: None,
        }),
        api::CitationsDeltaCitation::WebSearchResultLocation(citation) => Some(CitationDelta {
            r#type: "citation".to_string(),
            source: Some(citation.url),
            title: citation.title,
            cited_text: (!citation.cited_text.is_empty()).then_some(citation.cited_text),
            start_index: None,
            end_index: None,
            signature: Some(citation.encrypted_index),
        }),
        _ => None,
    }
}

fn map_anthropic_image_media_type(
    mime_type: &str,
) -> LanguageModelResult<Base64ImageSourceMediaType> {
    match mime_type {
        "image/jpeg" => Ok(Base64ImageSourceMediaType::ImageJpeg),
        "image/png" => Ok(Base64ImageSourceMediaType::ImagePng),
        "image/gif" => Ok(Base64ImageSourceMediaType::ImageGif),
        "image/webp" => Ok(Base64ImageSourceMediaType::ImageWebp),
        _ => Err(LanguageModelError::Unsupported(
            PROVIDER,
            format!("Unsupported Anthropic image mime type: {mime_type}"),
        )),
    }
}

fn map_start_content_block(
    content_block: ContentBlockStartEventContentBlock,
) -> Option<ContentBlock> {
    match content_block {
        ContentBlockStartEventContentBlock::Text(block) => Some(ContentBlock::Text(block)),
        ContentBlockStartEventContentBlock::Thinking(block) => Some(ContentBlock::Thinking(block)),
        ContentBlockStartEventContentBlock::RedactedThinking(block) => {
            Some(ContentBlock::RedactedThinking(block))
        }
        ContentBlockStartEventContentBlock::ToolUse(block) => Some(ContentBlock::ToolUse(block)),
        ContentBlockStartEventContentBlock::ServerToolUse(block) => {
            Some(ContentBlock::ServerToolUse(block))
        }
        ContentBlockStartEventContentBlock::WebSearchToolResult(block) => {
            Some(ContentBlock::WebSearchToolResult(block))
        }
        ContentBlockStartEventContentBlock::WebFetchToolResult(block) => {
            Some(ContentBlock::WebFetchToolResult(block))
        }
        ContentBlockStartEventContentBlock::CodeExecutionToolResult(block) => {
            Some(ContentBlock::CodeExecutionToolResult(block))
        }
        ContentBlockStartEventContentBlock::BashCodeExecutionToolResult(block) => {
            Some(ContentBlock::BashCodeExecutionToolResult(block))
        }
        ContentBlockStartEventContentBlock::TextEditorCodeExecutionToolResult(block) => {
            Some(ContentBlock::TextEditorCodeExecutionToolResult(block))
        }
        ContentBlockStartEventContentBlock::ToolSearchToolResult(block) => {
            Some(ContentBlock::ToolSearchToolResult(block))
        }
        ContentBlockStartEventContentBlock::ContainerUpload(block) => {
            Some(ContentBlock::ContainerUpload(block))
        }
        ContentBlockStartEventContentBlock::Unknown => None,
    }
}
