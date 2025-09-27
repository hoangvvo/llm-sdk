use crate::{
    anthropic::api::{
        self, Base64ImageSource, ContentBlock, ContentBlockDelta, ContentBlockDeltaEvent,
        ContentBlockStartEvent, CreateMessageParams, ImageSource, InputContentBlock, InputMessage,
        InputMessageContent, Message as AnthropicMessage, MessageDeltaEvent, MessageDeltaUsage,
        MessageStartEvent, MessageStreamEvent, RequestCitationsConfig, RequestImageBlock,
        RequestSearchResultBlock, RequestTextBlock, RequestThinkingBlock, RequestToolResultBlock,
        RequestToolUseBlock, SystemPrompt, ThinkingConfigDisabled, ThinkingConfigEnabled,
        ThinkingConfigParam, Tool, ToolResultContent, ToolResultContentBlock, Usage,
    },
    client_utils, stream_utils, Citation, CitationDelta, ContentDelta, ImagePart, LanguageModel,
    LanguageModelError, LanguageModelInput, LanguageModelMetadata, LanguageModelResult,
    LanguageModelStream, Message, ModelResponse, ModelUsage, Part, PartDelta, PartialModelResponse,
    ReasoningOptions, ReasoningPart, ReasoningPartDelta, TextPart, TextPartDelta, Tool as SdkTool,
    ToolCallPart, ToolCallPartDelta, ToolChoiceOption, ToolResultPart,
};
use async_stream::try_stream;
use futures::{future::BoxFuture, StreamExt};
use reqwest::{
    header::{HeaderMap, HeaderName, HeaderValue},
    Client,
};
use serde_json::{Map, Value};
use std::{collections::HashMap, sync::Arc};

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
                    let payload = convert_to_anthropic_create_params(input, &self.model_id, false)?;

                    let headers = self.request_headers()?;

                    let response: AnthropicMessage = client_utils::send_json(
                        &self.client,
                        &format!("{}/v1/messages", self.base_url),
                        &payload,
                        headers,
                    )
                    .await?;

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
                    let payload = convert_to_anthropic_create_params(input, &self.model_id, true)?;

                    let headers = self.request_headers()?;
                    let mut chunk_stream = client_utils::send_sse_stream::<_, MessageStreamEvent>(
                        &self.client,
                        &format!("{}/v1/messages", self.base_url),
                        &payload,
                        headers,
                        self.provider(),
                    )
                    .await?;

                    let metadata = self.metadata.clone();

                    let stream = try_stream! {
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
                                }
                                MessageStreamEvent::MessageDelta(MessageDeltaEvent { usage, .. }) => {
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
                                }
                                MessageStreamEvent::ContentBlockStart(ContentBlockStartEvent { content_block, index }) => {
                                    let deltas = map_anthropic_content_block_start_event(content_block, index)?;
                                    for delta in deltas {
                                        yield PartialModelResponse {
                                            delta: Some(delta),
                                            ..Default::default()
                                        };
                                    }
                                }
                                MessageStreamEvent::ContentBlockDelta(ContentBlockDeltaEvent { delta, index }) => {
                                    if let Some(delta) = map_anthropic_content_block_delta_event(delta, index) {
                                        yield PartialModelResponse {
                                            delta: Some(delta),
                                            ..Default::default()
                                        };
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

fn convert_to_anthropic_create_params(
    input: LanguageModelInput,
    model_id: &str,
    stream: bool,
) -> LanguageModelResult<Value> {
    let LanguageModelInput {
        system_prompt,
        messages,
        tools,
        tool_choice,
        response_format: _,
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
        extra,
    } = input;

    let max_tokens = max_tokens.unwrap_or(4096);

    let message_params = convert_to_anthropic_messages(messages)?;

    let params = CreateMessageParams {
        max_tokens,
        messages: message_params,
        metadata: None,
        model: api::Model::String(model_id.to_string()),
        service_tier: None,
        stop_sequences: None,
        stream: Some(stream),
        system: system_prompt.map(SystemPrompt::String),
        temperature,
        thinking: reasoning
            .map(|options| convert_to_anthropic_thinking_config(&options, max_tokens)),
        tool_choice: tool_choice.map(convert_to_anthropic_tool_choice),
        tools: tools.map(|tool_list| {
            tool_list
                .into_iter()
                .map(convert_tool)
                .map(api::ToolUnion::Tool)
                .collect()
        }),
        top_k: top_k
            .map(|value| {
                u32::try_from(value).map_err(|_| {
                    LanguageModelError::InvalidInput(
                        "Anthropic top_k must be a non-negative integer".to_string(),
                    )
                })
            })
            .transpose()?,
        top_p,
    };

    let mut value = serde_json::to_value(&params).map_err(|error| {
        LanguageModelError::Invariant(
            PROVIDER,
            format!("Failed to serialize Anthropic request: {error}"),
        )
    })?;

    if let Value::Object(ref mut map) = value {
        if let Some(extra) = extra {
            let Value::Object(extra_object) = extra else {
                return Err(LanguageModelError::InvalidInput(
                    "Anthropic extra field must be a JSON object".to_string(),
                ));
            };

            for (key, val) in extra_object {
                map.insert(key, val);
            }
        }
    } else {
        return Err(LanguageModelError::Invariant(
            PROVIDER,
            "Anthropic request serialization did not produce an object".to_string(),
        ));
    }

    Ok(value)
}

fn convert_tool(tool: SdkTool) -> Tool {
    Tool {
        name: tool.name,
        description: Some(tool.description),
        input_schema: tool.parameters,
        cache_control: None,
        type_field: None,
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
        content: InputMessageContent::Blocks(content_blocks),
        role: role.to_string(),
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
            text_part.text,
        ))),
        Part::Image(image_part) => Ok(InputContentBlock::Image(create_request_image_block(
            image_part,
        ))),
        Part::Source(source_part) => Ok(InputContentBlock::SearchResult(convert_source_part(
            source_part,
        )?)),
        Part::ToolCall(tool_call) => Ok(InputContentBlock::ToolUse(RequestToolUseBlock {
            cache_control: None,
            id: tool_call.tool_call_id,
            input: normalize_tool_args(tool_call.args)?,
            name: tool_call.tool_name,
        })),
        Part::ToolResult(tool_result) => Ok(InputContentBlock::ToolResult(
            convert_tool_result_part(tool_result)?,
        )),
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
    for part in tool_result.content {
        let block = convert_part_to_tool_result_content_block(part)?;
        content_blocks.push(block);
    }

    let content = if content_blocks.is_empty() {
        None
    } else {
        Some(ToolResultContent::Blocks(content_blocks))
    };

    Ok(RequestToolResultBlock {
        cache_control: None,
        content,
        is_error: tool_result.is_error,
        tool_use_id: tool_result.tool_call_id,
    })
}

fn convert_part_to_tool_result_content_block(
    part: Part,
) -> LanguageModelResult<ToolResultContentBlock> {
    match part {
        Part::Text(text_part) => Ok(ToolResultContentBlock::Text(create_request_text_block(
            text_part.text,
        ))),
        Part::Image(image_part) => Ok(ToolResultContentBlock::Image(create_request_image_block(
            image_part,
        ))),
        Part::Source(source_part) => Ok(ToolResultContentBlock::SearchResult(convert_source_part(
            source_part,
        )?)),
        _ => Err(LanguageModelError::Unsupported(
            PROVIDER,
            "Cannot convert tool result part to Anthropic content".to_string(),
        )),
    }
}

fn create_request_text_block(text: String) -> RequestTextBlock {
    RequestTextBlock {
        cache_control: None,
        citations: None,
        text,
        type_field: "text".to_string(),
    }
}

fn create_request_image_block(image_part: ImagePart) -> RequestImageBlock {
    RequestImageBlock {
        cache_control: None,
        source: ImageSource::Base64(Base64ImageSource {
            data: image_part.image_data,
            media_type: image_part.mime_type,
        }),
    }
}

fn convert_source_part(
    source_part: crate::SourcePart,
) -> LanguageModelResult<RequestSearchResultBlock> {
    let mut content = Vec::new();
    for part in source_part.content {
        match part {
            Part::Text(text_part) => content.push(create_request_text_block(text_part.text)),
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

fn convert_to_anthropic_thinking_config(
    reasoning: &ReasoningOptions,
    max_tokens: u32,
) -> ThinkingConfigParam {
    if !reasoning.enabled {
        return ThinkingConfigParam::Disabled(ThinkingConfigDisabled {});
    }

    let fallback = max_tokens.saturating_sub(1).max(1);
    let budget = reasoning
        .budget_tokens
        .map_or(fallback, |value| value.max(1));

    ThinkingConfigParam::Enabled(ThinkingConfigEnabled {
        budget_tokens: budget,
    })
}

fn map_anthropic_message(content: Vec<ContentBlock>) -> Vec<Part> {
    let mut parts = Vec::new();
    for block in content {
        if let Some(part) = map_content_block(block) {
            parts.push(part);
        }
    }
    parts
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
        _ => None,
    }
}

fn map_text_block(block: api::ResponseTextBlock) -> TextPart {
    let citations = map_text_citations(block.citations);
    TextPart {
        text: block.text,
        citations,
    }
}

fn map_text_citations(citations: Option<Vec<api::ResponseCitation>>) -> Option<Vec<Citation>> {
    let citations = citations?;

    let mut results = Vec::new();

    for citation in citations {
        if let api::ResponseCitation::SearchResultLocation(
            api::ResponseSearchResultLocationCitation {
                cited_text,
                end_block_index,
                search_result_index: _,
                source,
                start_block_index,
                title,
            },
        ) = citation
        {
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
                start_index: start_block_index,
                end_index: end_block_index,
            };

            results.push(mapped);
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
        tool_name: block.name,
        args: block.input,
        id: None,
    }
}

fn map_anthropic_usage(usage: &Usage) -> ModelUsage {
    ModelUsage {
        input_tokens: usage.input_tokens,
        output_tokens: usage.output_tokens,
        ..Default::default()
    }
}

fn map_anthropic_message_delta_usage(usage: &MessageDeltaUsage) -> ModelUsage {
    ModelUsage {
        input_tokens: usage.input_tokens.unwrap_or(0),
        output_tokens: usage.output_tokens,
        ..Default::default()
    }
}

fn map_anthropic_content_block_start_event(
    content_block: ContentBlock,
    index: usize,
) -> LanguageModelResult<Vec<ContentDelta>> {
    if let Some(part) = map_content_block(content_block) {
        let mut delta = stream_utils::loosely_convert_part_to_part_delta(part)?;
        if let PartDelta::ToolCall(tool_call_delta) = &mut delta {
            tool_call_delta.args = Some(String::new());
        }
        Ok(vec![ContentDelta { index, part: delta }])
    } else {
        Ok(vec![])
    }
}

fn map_anthropic_content_block_delta_event(
    delta: ContentBlockDelta,
    index: usize,
) -> Option<ContentDelta> {
    let part_delta = match delta {
        ContentBlockDelta::TextDelta(delta) => PartDelta::Text(TextPartDelta {
            text: delta.text,
            citation: None,
        }),
        ContentBlockDelta::InputJsonDelta(delta) => PartDelta::ToolCall(ToolCallPartDelta {
            tool_name: None,
            args: Some(delta.partial_json),
            tool_call_id: None,
            id: None,
        }),
        ContentBlockDelta::ThinkingDelta(delta) => PartDelta::Reasoning(ReasoningPartDelta {
            text: Some(delta.thinking),
            signature: None,
            id: None,
        }),
        ContentBlockDelta::SignatureDelta(delta) => PartDelta::Reasoning(ReasoningPartDelta {
            text: None,
            signature: Some(delta.signature),
            id: None,
        }),
        ContentBlockDelta::CitationsDelta(delta) => {
            if let Some(citation) = map_citation_delta(delta.citation) {
                PartDelta::Text(TextPartDelta {
                    text: String::new(),
                    citation: Some(citation),
                })
            } else {
                return None;
            }
        }
    };

    Some(ContentDelta {
        index,
        part: part_delta,
    })
}

fn map_citation_delta(citation: api::ResponseCitation) -> Option<CitationDelta> {
    let api::ResponseCitation::SearchResultLocation(api::ResponseSearchResultLocationCitation {
        cited_text,
        end_block_index,
        search_result_index: _,
        source,
        start_block_index,
        title,
    }) = citation
    else {
        return None;
    };

    let result = CitationDelta {
        r#type: "citation".to_string(),
        source: Some(source),
        title,
        cited_text: if cited_text.is_empty() {
            None
        } else {
            Some(cited_text)
        },
        start_index: Some(start_block_index),
        end_index: Some(end_block_index),
    };

    Some(result)
}
