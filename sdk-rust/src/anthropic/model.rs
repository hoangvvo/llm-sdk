use crate::{
    anthropic::api::{
        self, Base64ImageSource, Base64ImageSourceMediaType, Base64PDFSource,
        CacheControlEphemeral, CacheControlEphemeralTtl, ContentBlock, ContentBlockDeltaEvent,
        ContentBlockDeltaEventDelta, ContentBlockStartEvent, ContentBlockStartEventContentBlock,
        CreateMessageParams, CreateMessageParamsCacheControl, CreateMessageParamsSystem,
        CreateMessageParamsToolsItem, InputContentBlock, InputMessage, InputMessageContent,
        InputMessageRole, Message as AnthropicMessage, MessageDeltaEvent, MessageDeltaUsage,
        MessageStartEvent, MessageStreamEvent, OutputConfig, PlainTextSource,
        RequestCitationsConfig, RequestDocumentBlock, RequestDocumentBlockSource,
        RequestImageBlock, RequestImageBlockSource, RequestSearchResultBlock, RequestTextBlock,
        RequestTextBlockCitationsItem, RequestThinkingBlock, RequestToolResultBlock,
        RequestToolResultBlockContent, RequestToolResultBlockContentArrayItem, RequestToolUseBlock,
        RequestWebSearchResultLocationCitation, StopReason, ThinkingConfigAdaptive,
        ThinkingConfigDisabled, ThinkingConfigEnabled, ThinkingConfigParam, Tool,
        ToolSearchToolBM2520251119, ToolSearchToolBM2520251119Type, ToolSearchToolRegex20251119,
        ToolSearchToolRegex20251119Type, URLImageSource, URLPDFSource, Usage, UserLocation,
        WebSearchTool20250305,
    },
    client_utils, stream_utils,
    tool_result_utils::CANCELLED_TOOL_RESULT_FALLBACK_CONTENT,
    CacheRetention, Citation, CitationDelta, ContentDelta, FilePart, ImagePart, LanguageModel,
    LanguageModelError, LanguageModelInput, LanguageModelMetadata, LanguageModelResult,
    LanguageModelStream, Message, ModelResponse, ModelServerToolUsage, ModelTokensDetails,
    ModelUsage, ModelUsageCostOptions, Part, PartDelta, PartialModelResponse, ReasoningOptions,
    ReasoningPart, ReasoningPartDelta, ResponseFormatJson, ResponseFormatOption, TextPart,
    TextPartDelta, Tool as SdkTool, ToolCall, ToolCallDelta, ToolCallPart, ToolCallPartDelta,
    ToolChoiceOption, ToolResult, ToolResultPart, ToolResultStatus, ToolSearchStrategy,
    ToolSearchToolCall, ToolSearchToolCallDelta, ToolSearchToolCallStatus, ToolSearchToolResult,
    WebSearchToolCallDelta, WebSearchToolCallStatus,
};
use async_stream::try_stream;
use futures::{future::BoxFuture, StreamExt};
use reqwest::{
    header::{HeaderMap, HeaderName, HeaderValue},
    Client,
};
use serde_json::{Map, Value};
use std::{collections::HashMap, sync::Arc};

const USAGE_COST_OPTIONS: ModelUsageCostOptions = ModelUsageCostOptions {
    input_cache_tokens_are_additional: true,
    output_reasoning_tokens_are_additional: false,
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
                    let usage = map_anthropic_usage(&AnthropicUsage::from(&response.usage));

                    let cost = self
                        .metadata()
                        .and_then(|metadata| metadata.pricing.as_ref())
                        .map(|pricing| usage.calculate_cost(pricing, &USAGE_COST_OPTIONS));

                    Ok(ModelResponse {
                        content,
                        usage: Some(usage),
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
                        // Hosted search arguments are only usable once complete.
                        let mut server_tool_blocks = HashMap::<usize, AnthropicServerToolBlock>::new();
                        let mut server_tool_call_indexes = HashMap::<String, usize>::new();
                        // Hosted search can produce multiple billed messages with restarting block indexes.
                        let mut index_offset = 0_usize;
                        let mut max_index: Option<usize> = None;
                        let mut completed_usages: Vec<ModelUsage> = Vec::new();
                        let mut message_usage: Option<AnthropicUsage> = None;
                        while let Some(event) = chunk_stream.next().await {
                            match event? {
                                MessageStreamEvent::MessageStart(MessageStartEvent { message }) => {
                                    if let Some(usage) = message_usage.take() {
                                        completed_usages.push(map_anthropic_usage(&usage));
                                        index_offset = max_index.map_or(0, |index| index + 1);
                                    }
                                    message_usage = Some(AnthropicUsage::from(&message.usage));
                                    if matches!(message.stop_reason, Some(StopReason::Refusal)) {
                                        Err(LanguageModelError::Refusal(anthropic_refusal_message(
                                            message.stop_details.as_ref(),
                                        )))?;
                                    }
                                }
                                MessageStreamEvent::MessageDelta(MessageDeltaEvent { delta, usage }) => {
                                    message_usage
                                        .get_or_insert_default()
                                        .merge_message_delta(&usage);
                                    if matches!(delta.stop_reason, Some(StopReason::Refusal)) {
                                        Err(LanguageModelError::Refusal(anthropic_refusal_message(
                                            delta.stop_details.as_ref(),
                                        )))?;
                                    }
                                }
                                MessageStreamEvent::ContentBlockStart(ContentBlockStartEvent { content_block, index }) => {
                                    let index = anthropic_block_index(index, index_offset)?;
                                    max_index = Some(max_index.map_or(index, |max| max.max(index)));
                                    let content_block = match content_block {
                                        ContentBlockStartEventContentBlock::ServerToolUse(block) => {
                                            server_tool_call_indexes.insert(block.id.clone(), index);
                                            let call = map_anthropic_server_tool_use_start(&block.name);
                                            let tool_call_id = block.id.clone();
                                            server_tool_blocks.insert(index, AnthropicServerToolBlock {
                                                id: block.id,
                                                name: block.name,
                                                input: String::new(),
                                            });
                                            if let Some(call) = call {
                                                yield PartialModelResponse {
                                                    delta: Some(ContentDelta {
                                                        index,
                                                        part: PartDelta::ToolCall(ToolCallPartDelta {
                                                            tool_call_id: Some(tool_call_id),
                                                            call,
                                                            signature: None,
                                                            id: None,
                                                        }),
                                                    }),
                                                    ..Default::default()
                                                };
                                            }
                                            continue;
                                        }
                                        content_block => content_block,
                                    };
                                    let server_tool_result = match &content_block {
                                        ContentBlockStartEventContentBlock::WebSearchToolResult(block) => Some((
                                            &block.tool_use_id,
                                            ToolCallDelta::WebSearch(WebSearchToolCallDelta {
                                                action: None,
                                                status: Some(anthropic_web_search_call_status(
                                                    anthropic_web_search_result_status(&block.content),
                                                )),
                                            }),
                                        )),
                                        ContentBlockStartEventContentBlock::ToolSearchToolResult(block) => Some((
                                            &block.tool_use_id,
                                            ToolCallDelta::ToolSearch(ToolSearchToolCallDelta {
                                                args: None,
                                                status: Some(anthropic_tool_search_call_status(
                                                    anthropic_tool_search_result_status(&block.content),
                                                )),
                                            }),
                                        )),
                                        _ => None,
                                    };
                                    if let Some((tool_use_id, call)) = server_tool_result {
                                        if let Some(call_index) = server_tool_call_indexes.get(tool_use_id) {
                                            yield PartialModelResponse {
                                                delta: Some(ContentDelta {
                                                    index: *call_index,
                                                    part: PartDelta::ToolCall(ToolCallPartDelta {
                                                        tool_call_id: Some(tool_use_id.clone()),
                                                        call,
                                                        signature: None,
                                                        id: None,
                                                    }),
                                                }),
                                                ..Default::default()
                                            };
                                        }
                                    }
                                    let deltas = map_anthropic_content_block_start_event(content_block, index)?;
                                    for delta in deltas {
                                        yield PartialModelResponse {
                                            delta: Some(delta),
                                            ..Default::default()
                                        };
                                    }
                                }
                                MessageStreamEvent::ContentBlockDelta(ContentBlockDeltaEvent { delta, index }) => {
                                    let index = anthropic_block_index(index, index_offset)?;
                                    if let Some(block) = server_tool_blocks.get_mut(&index) {
                                        if let ContentBlockDeltaEventDelta::InputJsonDelta(input_delta) = &delta {
                                            block.input.push_str(&input_delta.partial_json);
                                        }
                                        continue;
                                    }
                                    if let Some(delta) = map_anthropic_content_block_delta_event(delta, index) {
                                        yield PartialModelResponse {
                                            delta: Some(delta),
                                            ..Default::default()
                                        };
                                    }
                                }
                                MessageStreamEvent::ContentBlockStop(event) => {
                                    let index = anthropic_block_index(event.index, index_offset)?;
                                    let Some(block) = server_tool_blocks.remove(&index) else {
                                        continue;
                                    };
                                    if let Some(call) = map_anthropic_server_tool_use_stop(&block.name, &block.input) {
                                        yield PartialModelResponse {
                                            delta: Some(ContentDelta {
                                                index,
                                                part: PartDelta::ToolCall(ToolCallPartDelta {
                                                    tool_call_id: Some(block.id),
                                                    call,
                                                    signature: None,
                                                    id: None,
                                                }),
                                            }),
                                            ..Default::default()
                                        };
                                    }
                                }
                                _ => {}
                            }
                        }

                        if let Some(usage) = message_usage {
                            completed_usages.push(map_anthropic_usage(&usage));
                        }
                        if !completed_usages.is_empty() {
                            let mut usage = ModelUsage::default();
                            for completed_usage in &completed_usages {
                                usage.add(completed_usage);
                            }
                            let cost = metadata
                                .as_ref()
                                .and_then(|meta| meta.pricing.as_ref())
                                .map(|pricing| usage.calculate_cost(pricing, &USAGE_COST_OPTIONS));
                            yield PartialModelResponse {
                                delta: None,
                                usage: Some(usage),
                                cost,
                            };
                        }
                    };

                    Ok(LanguageModelStream::from_stream(stream))
                },
            )
            .await
        })
    }
}

/// A hosted tool block whose JSON input is buffered until `content_block_stop`.
struct AnthropicServerToolBlock {
    id: String,
    name: api::ResponseServerToolUseBlockName,
    input: String,
}

fn anthropic_block_index(index: i64, offset: usize) -> LanguageModelResult<usize> {
    usize::try_from(index)
        .ok()
        .and_then(|index| index.checked_add(offset))
        .ok_or_else(|| {
            LanguageModelError::Invariant(
                PROVIDER,
                format!("Anthropic stream content block index out of range: {index}"),
            )
        })
}

/// Hosted calls exposed in conversation history:
/// - web search
/// - regex or BM25 tool search
/// - other hosted tools are omitted
fn map_anthropic_server_tool_use_start(
    name: &api::ResponseServerToolUseBlockName,
) -> Option<ToolCallDelta> {
    match name {
        api::ResponseServerToolUseBlockName::WebSearch => {
            Some(ToolCallDelta::WebSearch(WebSearchToolCallDelta {
                action: None,
                status: Some(WebSearchToolCallStatus::InProgress),
            }))
        }
        api::ResponseServerToolUseBlockName::ToolSearchToolRegex
        | api::ResponseServerToolUseBlockName::ToolSearchToolBm25 => {
            Some(ToolCallDelta::ToolSearch(ToolSearchToolCallDelta {
                args: None,
                status: Some(ToolSearchToolCallStatus::InProgress),
            }))
        }
        _ => None,
    }
}

fn map_anthropic_server_tool_use_stop(
    name: &api::ResponseServerToolUseBlockName,
    input: &str,
) -> Option<ToolCallDelta> {
    let parsed_input =
        serde_json::from_str::<Value>(if input.is_empty() { "{}" } else { input }).ok();
    match name {
        api::ResponseServerToolUseBlockName::WebSearch => {
            let query = parsed_input.as_ref()?.get("query")?.as_str()?;
            Some(ToolCallDelta::WebSearch(WebSearchToolCallDelta {
                action: Some(crate::WebSearchAction::Search {
                    queries: vec![query.to_string()],
                }),
                status: None,
            }))
        }
        api::ResponseServerToolUseBlockName::ToolSearchToolRegex
        | api::ResponseServerToolUseBlockName::ToolSearchToolBm25 => {
            Some(ToolCallDelta::ToolSearch(ToolSearchToolCallDelta {
                args: Some(
                    anthropic_tool_search_args(parsed_input.unwrap_or(Value::Null)).to_string(),
                ),
                status: None,
            }))
        }
        _ => None,
    }
}

/// Hosted tool search arguments are always a JSON object.
fn anthropic_tool_search_args(input: Value) -> Value {
    match input {
        Value::Object(_) => input,
        _ => Value::Object(Map::new()),
    }
}

fn anthropic_refusal_message(details: Option<&api::RefusalStopDetails>) -> String {
    details
        .and_then(|details| {
            details.explanation.clone().or_else(|| {
                details.category.as_ref().map(|category| {
                    let category = match category {
                        api::RefusalStopDetailsCategory::Cyber => "cyber",
                        api::RefusalStopDetailsCategory::Bio => "bio",
                        _ => "unknown",
                    };
                    format!("Anthropic policy category: {category}")
                })
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
        cache_retention,
    } = input;

    let max_tokens = i64::from(max_tokens.unwrap_or(4096));

    let message_params = convert_to_anthropic_messages(messages)?;

    let params = CreateMessageParams {
        // Top-level cache_control caches through the last cacheable block.
        cache_control: cache_retention.map(|retention| {
            CreateMessageParamsCacheControl::Ephemeral(CacheControlEphemeral {
                ttl: (retention == CacheRetention::Extended)
                    .then_some(CacheControlEphemeralTtl::N1H),
            })
        }),
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
            defer_loading: tool.defer_loading.filter(|deferred| *deferred),
            eager_input_streaming: None,
            input_examples: None,
            strict: Some(true),
            r#type: None,
        }),
        SdkTool::ToolSearch(tool) => match tool.strategy {
            Some(ToolSearchStrategy::Regex) => {
                CreateMessageParamsToolsItem::ToolSearchToolRegex20251119(
                    ToolSearchToolRegex20251119 {
                        allowed_callers: None,
                        cache_control: None,
                        defer_loading: None,
                        name: "tool_search_tool_regex".to_string(),
                        strict: None,
                        r#type: ToolSearchToolRegex20251119Type::ToolSearchToolRegex20251119,
                    },
                )
            }
            Some(ToolSearchStrategy::Bm25) | None => {
                CreateMessageParamsToolsItem::ToolSearchToolBM2520251119(
                    ToolSearchToolBM2520251119 {
                        allowed_callers: None,
                        cache_control: None,
                        defer_loading: None,
                        name: "tool_search_tool_bm25".to_string(),
                        strict: None,
                        r#type: ToolSearchToolBM2520251119Type::ToolSearchToolBm2520251119,
                    },
                )
            }
        },
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

#[allow(clippy::too_many_lines)]
fn convert_part_to_content_block(part: Part) -> LanguageModelResult<InputContentBlock> {
    match part {
        Part::Text(text_part) => Ok(InputContentBlock::Text(create_request_text_block(
            text_part,
        ))),
        Part::Image(image_part) => Ok(InputContentBlock::Image(create_request_image_block(
            image_part,
        ))),
        Part::File(file_part) => Ok(InputContentBlock::Document(create_request_document_block(
            file_part,
        ))),
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
            crate::ToolCall::ToolSearch(call) => {
                let input = normalize_tool_args(call.args)?;
                // Regex searches carry a pattern; anything else replays as a BM25 query.
                let name = if input.get("pattern").is_some() {
                    api::RequestServerToolUseBlockName::ToolSearchToolRegex
                } else {
                    api::RequestServerToolUseBlockName::ToolSearchToolBm25
                };
                Ok(InputContentBlock::ServerToolUse(
                    api::RequestServerToolUseBlock {
                        cache_control: None,
                        caller: None,
                        id: tool_call.tool_call_id,
                        input,
                        name,
                    },
                ))
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
            crate::ToolResult::ToolSearch(result) => Ok(InputContentBlock::ToolSearchToolResult(
                api::RequestToolSearchToolResultBlock {
                    cache_control: None,
                    content: convert_tool_search_result_content(result),
                    tool_use_id: tool_result.tool_call_id,
                },
            )),
        },
        Part::Reasoning(reasoning_part) => Ok(convert_reasoning_part(reasoning_part)),
        Part::Audio(_) => Err(LanguageModelError::Unsupported(
            PROVIDER,
            "Anthropic does not support audio parts".to_string(),
        )),
    }
}

fn convert_tool_search_result_content(
    result: ToolSearchToolResult,
) -> api::RequestToolSearchToolResultBlockContent {
    if let Some(code) = result.error_code {
        return api::RequestToolSearchToolResultBlockContent::ToolSearchToolResultError(
            api::RequestToolSearchToolResultError {
                error_code: match code.as_str() {
                    "unavailable" => api::ToolSearchToolResultErrorCode::Unavailable,
                    "too_many_requests" => api::ToolSearchToolResultErrorCode::TooManyRequests,
                    "execution_time_exceeded" => {
                        api::ToolSearchToolResultErrorCode::ExecutionTimeExceeded
                    }
                    _ => api::ToolSearchToolResultErrorCode::InvalidToolInput,
                },
            },
        );
    }
    api::RequestToolSearchToolResultBlockContent::ToolSearchToolSearchResult(
        api::RequestToolSearchToolSearchResultBlock {
            tool_references: result
                .tool_names
                .into_iter()
                .map(|tool_name| api::RequestToolReferenceBlock {
                    cache_control: None,
                    tool_name,
                    r#type: "tool_reference".to_string(),
                })
                .collect(),
        },
    )
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
        is_error: Some(tool_result.status != ToolResultStatus::Completed),
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
            create_request_image_block(image_part),
        )),
        Part::File(file_part) => Ok(RequestToolResultBlockContentArrayItem::Document(
            create_request_document_block(file_part),
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

fn create_request_image_block(image_part: ImagePart) -> RequestImageBlock {
    let ImagePart {
        mime_type,
        data,
        url,
        ..
    } = image_part;
    let source = match url {
        Some(url) => RequestImageBlockSource::Url(URLImageSource { url }),
        None => RequestImageBlockSource::Base64(Base64ImageSource {
            media_type: map_anthropic_image_media_type(&mime_type),
            data: data.unwrap_or_default(),
        }),
    };
    RequestImageBlock {
        cache_control: None,
        source,
    }
}

fn create_request_document_block(file_part: FilePart) -> RequestDocumentBlock {
    let FilePart {
        mime_type,
        data,
        url,
        filename,
    } = file_part;
    let source = if let Some(url) = url {
        RequestDocumentBlockSource::Url(URLPDFSource { url })
    } else if mime_type == "text/plain" {
        RequestDocumentBlockSource::Text(PlainTextSource {
            data: data.unwrap_or_default(),
            media_type: mime_type,
        })
    } else {
        RequestDocumentBlockSource::Base64(Base64PDFSource {
            data: data.unwrap_or_default(),
            media_type: mime_type,
        })
    };
    RequestDocumentBlock {
        cache_control: None,
        citations: None,
        context: None,
        source,
        title: filename,
        r#type: "document".to_string(),
    }
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
    let call_statuses: HashMap<String, ToolResultStatus> = content
        .iter()
        .filter_map(|block| match block {
            ContentBlock::WebSearchToolResult(result) => Some((
                result.tool_use_id.clone(),
                anthropic_web_search_result_status(&result.content),
            )),
            ContentBlock::ToolSearchToolResult(result) => Some((
                result.tool_use_id.clone(),
                anthropic_tool_search_result_status(&result.content),
            )),
            _ => None,
        })
        .collect();
    for block in content {
        if let Some(mut part) = map_content_block(block) {
            if let Part::ToolCall(call) = &mut part {
                if let Some(status) = call_statuses.get(&call.tool_call_id) {
                    match &mut call.call {
                        ToolCall::WebSearch(web) => {
                            web.status = Some(anthropic_web_search_call_status(*status));
                        }
                        ToolCall::ToolSearch(search) => {
                            search.status = Some(anthropic_tool_search_call_status(*status));
                        }
                        ToolCall::Function(_) => {}
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
) -> ToolResultStatus {
    if matches!(
        content,
        api::ResponseWebSearchToolResultBlockContent::ResponseWebSearchToolResultError(_)
    ) {
        ToolResultStatus::Failed
    } else {
        ToolResultStatus::Completed
    }
}

fn anthropic_tool_search_result_status(
    content: &api::ResponseToolSearchToolResultBlockContent,
) -> ToolResultStatus {
    if matches!(
        content,
        api::ResponseToolSearchToolResultBlockContent::ToolSearchToolResultError(_)
    ) {
        ToolResultStatus::Failed
    } else {
        ToolResultStatus::Completed
    }
}

fn anthropic_web_search_call_status(status: ToolResultStatus) -> WebSearchToolCallStatus {
    if status == ToolResultStatus::Failed {
        WebSearchToolCallStatus::Failed
    } else {
        WebSearchToolCallStatus::Completed
    }
}

fn anthropic_tool_search_call_status(status: ToolResultStatus) -> ToolSearchToolCallStatus {
    if status == ToolResultStatus::Failed {
        ToolSearchToolCallStatus::Failed
    } else {
        ToolSearchToolCallStatus::Completed
    }
}

#[allow(clippy::too_many_lines)]
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
        ContentBlock::ServerToolUse(block) => match block.name {
            api::ResponseServerToolUseBlockName::WebSearch => {
                let action = block
                    .input
                    .get("query")
                    .and_then(Value::as_str)
                    .map(|query| crate::WebSearchAction::Search {
                        queries: vec![query.to_string()],
                    });
                Some(Part::ToolCall(ToolCallPart {
                    tool_call_id: block.id,
                    call: ToolCall::WebSearch(crate::WebSearchToolCall {
                        action,
                        status: Some(WebSearchToolCallStatus::InProgress),
                    }),
                    signature: None,
                    id: None,
                }))
            }
            api::ResponseServerToolUseBlockName::ToolSearchToolRegex
            | api::ResponseServerToolUseBlockName::ToolSearchToolBm25 => {
                Some(Part::ToolCall(ToolCallPart {
                    tool_call_id: block.id,
                    call: ToolCall::ToolSearch(ToolSearchToolCall {
                        args: anthropic_tool_search_args(block.input),
                        status: Some(ToolSearchToolCallStatus::InProgress),
                    }),
                    signature: None,
                    id: None,
                }))
            }
            // Hosted tools the SDK does not model (web fetch, code execution, ...)
            // are ignored.
            _ => None,
        },
        ContentBlock::ToolSearchToolResult(block) => {
            let (tool_names, error_code) = match block.content {
                api::ResponseToolSearchToolResultBlockContent::ToolSearchToolSearchResult(
                    result,
                ) => (
                    result
                        .tool_references
                        .into_iter()
                        .map(|reference| reference.tool_name)
                        .collect(),
                    None,
                ),
                api::ResponseToolSearchToolResultBlockContent::ToolSearchToolResultError(error) => {
                    (
                        vec![],
                        Some(
                            match error.error_code {
                                api::ToolSearchToolResultErrorCode::InvalidToolInput => {
                                    "invalid_tool_input"
                                }
                                api::ToolSearchToolResultErrorCode::Unavailable => "unavailable",
                                api::ToolSearchToolResultErrorCode::TooManyRequests => {
                                    "too_many_requests"
                                }
                                api::ToolSearchToolResultErrorCode::ExecutionTimeExceeded => {
                                    "execution_time_exceeded"
                                }
                                api::ToolSearchToolResultErrorCode::Unknown => "unknown",
                            }
                            .to_string(),
                        ),
                    )
                }
                api::ResponseToolSearchToolResultBlockContent::Unknown => (vec![], None),
            };
            let status = if error_code.is_some() {
                ToolResultStatus::Failed
            } else {
                ToolResultStatus::Completed
            };
            Some(Part::ToolResult(ToolResultPart {
                tool_call_id: block.tool_use_id,
                result: ToolResult::ToolSearch(ToolSearchToolResult {
                    tool_names,
                    error_code,
                }),
                status,
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

/// The usage fields shared by `message_start` and the cumulative
/// `message_delta` events. Fields of later events overwrite earlier values when
/// present.
#[derive(Default)]
struct AnthropicUsage {
    input_tokens: Option<i64>,
    output_tokens: Option<i64>,
    cache_read_input_tokens: Option<i64>,
    cache_creation_input_tokens: Option<i64>,
    cache_creation_1h_input_tokens: Option<i64>,
    thinking_tokens: Option<i64>,
    web_search_requests: Option<i64>,
}

impl From<&Usage> for AnthropicUsage {
    fn from(usage: &Usage) -> Self {
        Self {
            input_tokens: Some(usage.input_tokens),
            output_tokens: Some(usage.output_tokens),
            cache_read_input_tokens: usage.cache_read_input_tokens,
            cache_creation_input_tokens: usage.cache_creation_input_tokens,
            cache_creation_1h_input_tokens: usage
                .cache_creation
                .as_ref()
                .map(|cache_creation| cache_creation.ephemeral_1_h_input_tokens),
            thinking_tokens: usage
                .output_tokens_details
                .as_ref()
                .map(|details| details.thinking_tokens),
            web_search_requests: usage
                .server_tool_use
                .as_ref()
                .map(|server_tool_use| server_tool_use.web_search_requests),
        }
    }
}

impl AnthropicUsage {
    fn merge_message_delta(&mut self, usage: &MessageDeltaUsage) {
        if let Some(value) = usage.input_tokens {
            self.input_tokens = Some(value);
        }
        self.output_tokens = Some(usage.output_tokens);
        if let Some(value) = usage.cache_read_input_tokens {
            self.cache_read_input_tokens = Some(value);
        }
        if let Some(value) = usage.cache_creation_input_tokens {
            self.cache_creation_input_tokens = Some(value);
        }
        if let Some(details) = &usage.output_tokens_details {
            self.thinking_tokens = Some(details.thinking_tokens);
        }
        if let Some(server_tool_use) = &usage.server_tool_use {
            self.web_search_requests = Some(server_tool_use.web_search_requests);
        }
    }
}

/// Anthropic reports `input_tokens` without the cached and cache-write tokens.
/// The numbers are kept as reported; the cost calculation accounts for it.
fn map_anthropic_usage(usage: &AnthropicUsage) -> ModelUsage {
    let count = |value: i64| u32::try_from(value).unwrap_or(0);

    let mut input_tokens_details = ModelTokensDetails::default();
    if let Some(value) = usage.cache_read_input_tokens {
        input_tokens_details.cached_tokens = Some(count(value));
    }
    if let Some(value) = usage.cache_creation_input_tokens {
        input_tokens_details.cache_write_tokens = Some(count(value));
    }
    if let Some(value) = usage
        .cache_creation_1h_input_tokens
        .filter(|value| *value > 0)
    {
        input_tokens_details.extended_cache_write_tokens = Some(count(value));
    }

    ModelUsage {
        input_tokens: count(usage.input_tokens.unwrap_or(0)),
        output_tokens: count(usage.output_tokens.unwrap_or(0)),
        input_tokens_details: (input_tokens_details != ModelTokensDetails::default())
            .then_some(input_tokens_details),
        output_tokens_details: usage
            .thinking_tokens
            .map(|thinking_tokens| ModelTokensDetails {
                reasoning_tokens: Some(count(thinking_tokens)),
                ..Default::default()
            }),
        server_tool_use: usage
            .web_search_requests
            .filter(|requests| *requests > 0)
            .map(|requests| ModelServerToolUsage {
                web_search_requests: Some(count(requests)),
            }),
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
                text: delta.thinking,
                signature: None,
                id: None,
            })
        }
        ContentBlockDeltaEventDelta::SignatureDelta(delta) => {
            PartDelta::Reasoning(ReasoningPartDelta {
                text: String::new(),
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

fn map_anthropic_image_media_type(mime_type: &str) -> Base64ImageSourceMediaType {
    match mime_type {
        "image/jpeg" => Base64ImageSourceMediaType::ImageJpeg,
        "image/png" => Base64ImageSourceMediaType::ImagePng,
        "image/gif" => Base64ImageSourceMediaType::ImageGif,
        "image/webp" => Base64ImageSourceMediaType::ImageWebp,
        // Serialized as a missing media_type, which Anthropic rejects.
        _ => Base64ImageSourceMediaType::Unknown,
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
