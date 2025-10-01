use super::api::{
    Content, FunctionCall, FunctionCallingConfig, FunctionCallingConfigMode, FunctionDeclaration,
    FunctionResponse, GenerateContentConfig, GenerateContentParameters, GenerateContentResponse,
    MediaModality, ModalityTokenCount, Part as GooglePart, PrebuiltVoiceConfig, SpeechConfig,
    ThinkingConfig, Tool, ToolConfig, VoiceConfig,
};
use crate::{
    audio_part_utils, client_utils, id_utils, source_part_utils, stream_utils, AudioPart,
    ContentDelta, ImagePart, LanguageModel, LanguageModelError, LanguageModelInput,
    LanguageModelMetadata, LanguageModelResult, LanguageModelStream, Message, ModelResponse,
    ModelTokensDetails, ModelUsage, Part, PartialModelResponse, ReasoningPart,
    ResponseFormatOption, ToolChoiceOption,
};
use async_stream::try_stream;
use futures::{future::BoxFuture, StreamExt};
use reqwest::{
    header::{HeaderMap, HeaderName, HeaderValue},
    Client,
};
use serde_json::json;
use std::{collections::HashMap, sync::Arc};

const PROVIDER: &str = "google";

pub struct GoogleModel {
    model_id: String,
    api_key: String,
    base_url: String,
    client: Client,
    metadata: Option<Arc<LanguageModelMetadata>>,
    headers: HashMap<String, String>,
}

#[derive(Clone, Default)]
pub struct GoogleModelOptions {
    pub api_key: String,
    pub base_url: Option<String>,
    pub headers: Option<HashMap<String, String>>,
    pub client: Option<Client>,
}

impl GoogleModel {
    #[must_use]
    pub fn new(model_id: impl Into<String>, options: GoogleModelOptions) -> Self {
        let GoogleModelOptions {
            api_key,
            base_url,
            headers,
            client,
        } = options;

        let base_url = base_url
            .unwrap_or_else(|| "https://generativelanguage.googleapis.com/v1beta".to_string())
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

        for (key, value) in &self.headers {
            let header_name = HeaderName::from_bytes(key.as_bytes()).map_err(|error| {
                LanguageModelError::InvalidInput(format!(
                    "Invalid Google header name '{key}': {error}"
                ))
            })?;
            let header_value = HeaderValue::from_str(value).map_err(|error| {
                LanguageModelError::InvalidInput(format!(
                    "Invalid Google header value for '{key}': {error}"
                ))
            })?;
            headers.insert(header_name, header_value);
        }

        Ok(headers)
    }
}

impl LanguageModel for GoogleModel {
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
                    let params = convert_to_generate_content_parameters(input, &self.model_id)?;

                    let url = format!(
                        "{}/models/{}:generateContent?key={}",
                        self.base_url, self.model_id, self.api_key
                    );

                    let headers = self.request_headers()?;
                    let response: GenerateContentResponse =
                        client_utils::send_json(&self.client, &url, &params, headers).await?;

                    let candidate = response
                        .candidates
                        .and_then(|c| c.into_iter().next())
                        .ok_or_else(|| {
                            LanguageModelError::Invariant(
                                PROVIDER,
                                "No candidate in response".to_string(),
                            )
                        })?;

                    let content = map_google_content(
                        candidate.content.and_then(|c| c.parts).unwrap_or_default(),
                    )?;

                    let usage = response
                        .usage_metadata
                        .map(|u| map_google_usage_metadata(&u));

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
                    let params = convert_to_generate_content_parameters(input, &self.model_id)?;
                    let metadata = self.metadata.clone();

                    let url = format!(
                        "{}/models/{}:streamGenerateContent?key={}&alt=sse",
                        self.base_url, self.model_id, self.api_key
                    );

                    let headers = self.request_headers()?;
                    let mut chunk_stream = client_utils::send_sse_stream::<
                        _,
                        GenerateContentResponse,
                    >(
                        &self.client, &url, &params, headers, self.provider()
                    )
                    .await?;

                    let stream = try_stream! {
                        let mut all_content_deltas: Vec<ContentDelta> = Vec::new();

                        while let Some(chunk) = chunk_stream.next().await {
                            let response = chunk?;

                            let candidate = response
                                .candidates
                                .and_then(|c| c.into_iter().next());

                            if let Some(candidate) = candidate {
                                if let Some(content) = candidate.content {
                                    if let Some(parts) = content.parts {
                                        let incoming_deltas = map_google_content_to_delta(
                                            parts,
                                            &all_content_deltas,
                                        )?;

                                        all_content_deltas.extend(incoming_deltas.clone());

                                        for delta in incoming_deltas {
                                            yield PartialModelResponse {
                                                delta: Some(delta),
                                                usage: None,
                                                cost: None,
                                            };
                                        }
                                    }
                                }
                            }

                            if let Some(usage_metadata) = response.usage_metadata {
                                let usage = map_google_usage_metadata(&usage_metadata);
                                yield PartialModelResponse {
                                    delta: None,
                                    cost: metadata
                                        .as_ref()
                                        .and_then(|m| m.pricing.as_ref())
                                        .map(|pricing| usage.calculate_cost(pricing)),
                                    usage: Some(usage),
                                };
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

fn convert_to_generate_content_parameters(
    input: LanguageModelInput,
    model_id: &str,
) -> LanguageModelResult<GenerateContentParameters> {
    let messages = convert_to_google_contents(input.messages)?;

    let mut params = GenerateContentParameters {
        contents: messages,
        model: model_id.to_string(),
        ..Default::default()
    };
    let mut config = GenerateContentConfig::default();

    if let Some(system_prompt) = input.system_prompt {
        params.system_instruction = Some(Content {
            role: Some("system".to_string()),
            parts: Some(vec![GooglePart {
                text: Some(system_prompt),
                ..Default::default()
            }]),
        });
    }

    if let Some(temp) = input.temperature {
        config.temperature = Some(temp);
    }
    if let Some(top_p) = input.top_p {
        config.top_p = Some(top_p);
    }
    if let Some(top_k) = input.top_k {
        config.top_k = Some(top_k);
    }
    if let Some(presence_penalty) = input.presence_penalty {
        config.presence_penalty = Some(presence_penalty);
    }
    if let Some(frequency_penalty) = input.frequency_penalty {
        config.frequency_penalty = Some(frequency_penalty);
    }
    if let Some(seed) = input.seed {
        config.seed = Some(seed);
    }
    if let Some(max_tokens) = input.max_tokens {
        config.max_output_tokens = Some(max_tokens);
    }

    if let Some(tools) = input.tools {
        let function_declarations = tools
            .into_iter()
            .map(|tool| FunctionDeclaration {
                name: Some(tool.name),
                description: Some(tool.description),
                parameters_json_schema: Some(tool.parameters),
                ..Default::default()
            })
            .collect();

        params.tools = Some(vec![Tool {
            function_declarations: Some(function_declarations),
        }]);
    }

    if let Some(tool_choice) = input.tool_choice {
        params.tool_config = Some(ToolConfig {
            function_calling_config: Some(convert_to_google_function_calling_config(tool_choice)),
        });
    }

    if let Some(response_format) = input.response_format {
        let (response_mime_type, response_json_schema) =
            convert_to_google_response_schema(response_format);
        config.response_mime_type = Some(response_mime_type);
        config.response_json_schema = response_json_schema;
    }

    if let Some(modalities) = input.modalities {
        config.response_modalities = Some(
            modalities
                .into_iter()
                .map(|m| match m {
                    crate::Modality::Text => "TEXT".to_string(),
                    crate::Modality::Image => "IMAGE".to_string(),
                    crate::Modality::Audio => "AUDIO".to_string(),
                })
                .collect(),
        );
    }

    if let Some(audio) = input.audio {
        if let Some(voice) = audio.voice {
            config.speech_config = Some(SpeechConfig {
                voice_config: Some(VoiceConfig {
                    prebuilt_voice_config: Some(PrebuiltVoiceConfig {
                        voice_name: Some(voice),
                    }),
                }),
                language_code: audio.language,
                multi_speaker_voice_config: None,
            });
        }
    }

    if let Some(reasoning) = input.reasoning {
        config.thinking_config = Some(ThinkingConfig {
            include_thoughts: Some(reasoning.enabled),
            thinking_budget: reasoning
                .budget_tokens
                .map(|t| i32::try_from(t).unwrap_or(0)),
        });
    }

    params.generation_config = Some(config);

    params.extra = input.extra;

    Ok(params)
}

fn convert_to_google_contents(messages: Vec<Message>) -> LanguageModelResult<Vec<Content>> {
    messages
        .into_iter()
        .map(|message| match message {
            Message::User(user_message) => Ok(Content {
                role: Some("user".to_string()),
                parts: Some(
                    user_message
                        .content
                        .into_iter()
                        .flat_map(convert_to_google_parts)
                        .collect(),
                ),
            }),
            Message::Assistant(assistant_message) => Ok(Content {
                role: Some("model".to_string()),
                parts: Some(
                    assistant_message
                        .content
                        .into_iter()
                        .flat_map(convert_to_google_parts)
                        .collect(),
                ),
            }),
            Message::Tool(tool_message) => Ok(Content {
                role: Some("user".to_string()),
                parts: Some(
                    tool_message
                        .content
                        .into_iter()
                        .flat_map(convert_to_google_parts)
                        .collect(),
                ),
            }),
        })
        .collect()
}

fn convert_to_google_parts(part: Part) -> Vec<GooglePart> {
    match part {
        Part::Text(text_part) => vec![GooglePart {
            text: Some(text_part.text),
            ..Default::default()
        }],
        Part::Image(image_part) => vec![GooglePart {
            inline_data: Some(super::api::Blob2 {
                data: Some(image_part.data),
                mime_type: Some(image_part.mime_type),
                display_name: None,
            }),
            ..Default::default()
        }],
        Part::Audio(audio_part) => vec![GooglePart {
            inline_data: Some(super::api::Blob2 {
                data: Some(audio_part.data),
                mime_type: Some(audio_part_utils::map_audio_format_to_mime_type(
                    &audio_part.format,
                )),
                display_name: None,
            }),
            ..Default::default()
        }],
        Part::Reasoning(reasoning_part) => vec![GooglePart {
            text: Some(reasoning_part.text),
            thought: Some(true),
            thought_signature: reasoning_part.signature,
            ..Default::default()
        }],
        Part::Source(source_part) => source_part
            .content
            .into_iter()
            .flat_map(convert_to_google_parts)
            .collect(),
        Part::ToolCall(tool_call_part) => vec![GooglePart {
            function_call: Some(FunctionCall {
                name: Some(tool_call_part.tool_name),
                args: Some(tool_call_part.args),
                id: Some(tool_call_part.tool_call_id),
            }),
            ..Default::default()
        }],
        Part::ToolResult(tool_result_part) => vec![GooglePart {
            function_response: Some(FunctionResponse {
                id: Some(tool_result_part.tool_call_id),
                name: Some(tool_result_part.tool_name),
                response: Some(convert_to_google_function_response(
                    tool_result_part.content,
                    tool_result_part.is_error.unwrap_or(false),
                )),
            }),
            ..Default::default()
        }],
    }
}

fn convert_to_google_function_response(
    parts: Vec<Part>,
    is_error: bool,
) -> HashMap<String, serde_json::Value> {
    let compatible_parts = source_part_utils::get_compatible_parts_without_source_parts(parts);
    let text_parts: Vec<String> = compatible_parts
        .into_iter()
        .filter_map(|part| {
            if let Part::Text(text_part) = part {
                Some(text_part.text)
            } else {
                None
            }
        })
        .collect();

    let responses: Vec<serde_json::Value> = text_parts
        .into_iter()
        .map(|text| serde_json::from_str(&text).unwrap_or_else(|_| json!({ "data": text })))
        .collect();

    // Use "output" key to specify function output and "error" key to specify error
    // details, as per Google API specification
    let mut result = HashMap::new();
    let key = if is_error { "error" } else { "output" };
    let value = if responses.len() == 1 {
        responses.into_iter().next().unwrap_or(json!({}))
    } else {
        json!(responses)
    };
    result.insert(key.to_string(), value);
    result
}

fn convert_to_google_function_calling_config(
    tool_choice: ToolChoiceOption,
) -> FunctionCallingConfig {
    match tool_choice {
        ToolChoiceOption::Auto => FunctionCallingConfig {
            mode: Some(FunctionCallingConfigMode::Auto),
            allowed_function_names: None,
        },
        ToolChoiceOption::None => FunctionCallingConfig {
            mode: Some(FunctionCallingConfigMode::None),
            allowed_function_names: None,
        },
        ToolChoiceOption::Required => FunctionCallingConfig {
            mode: Some(FunctionCallingConfigMode::Any),
            allowed_function_names: None,
        },
        ToolChoiceOption::Tool(tool) => FunctionCallingConfig {
            mode: Some(FunctionCallingConfigMode::Any),
            allowed_function_names: Some(vec![tool.tool_name]),
        },
    }
}

fn convert_to_google_response_schema(
    response_format: ResponseFormatOption,
) -> (String, Option<serde_json::Value>) {
    match response_format {
        ResponseFormatOption::Text => ("text/plain".to_string(), None),
        ResponseFormatOption::Json(json_format) => {
            ("application/json".to_string(), json_format.schema)
        }
    }
}

fn map_google_content(parts: Vec<GooglePart>) -> LanguageModelResult<Vec<Part>> {
    parts
        .into_iter()
        .filter_map(|part| {
            if let Some(text) = part.text {
                if part.thought.unwrap_or(false) {
                    let mut reasoning_part = ReasoningPart::new(text);
                    if let Some(signature) = part.thought_signature {
                        reasoning_part = reasoning_part.with_signature(signature);
                    }
                    Some(Ok(reasoning_part.into()))
                } else {
                    Some(Ok(Part::text(text)))
                }
            } else if let Some(inline_data) = part.inline_data {
                if let (Some(data), Some(mime_type)) = (inline_data.data, inline_data.mime_type) {
                    if mime_type.starts_with("image/") {
                        Some(Ok(Part::Image(ImagePart {
                            data: data,
                            mime_type,
                            width: None,
                            height: None,
                            id: None,
                        })))
                    } else if mime_type.starts_with("audio/") {
                        if let Ok(format) =
                            audio_part_utils::map_mime_type_to_audio_format(&mime_type)
                        {
                            Some(Ok(Part::Audio(AudioPart {
                                data: data,
                                format,
                                sample_rate: None,
                                channels: None,
                                id: None,
                                transcript: None,
                            })))
                        } else {
                            Some(Err(LanguageModelError::Invariant(
                                PROVIDER,
                                format!("Unsupported audio mime type: {mime_type}"),
                            )))
                        }
                    } else {
                        None
                    }
                } else {
                    Some(Err(LanguageModelError::Invariant(
                        PROVIDER,
                        "Inline data missing data or mime type".to_string(),
                    )))
                }
            } else if let Some(function_call) = part.function_call {
                if let Some(name) = function_call.name {
                    Some(Ok(Part::ToolCall(crate::ToolCallPart {
                        tool_call_id: function_call
                            .id
                            // Google does not always return id, generate one if missing
                            .unwrap_or_else(|| id_utils::generate_string(10)),
                        tool_name: name,
                        args: json!(function_call.args.unwrap_or_default()),
                        id: None,
                    })))
                } else {
                    Some(Err(LanguageModelError::Invariant(
                        PROVIDER,
                        "Function call missing name".to_string(),
                    )))
                }
            } else {
                None
            }
        })
        .collect()
}

fn map_google_content_to_delta(
    parts: Vec<GooglePart>,
    existing_deltas: &[ContentDelta],
) -> LanguageModelResult<Vec<ContentDelta>> {
    let mut deltas = Vec::new();

    let parts = map_google_content(parts)?;

    for part in parts {
        let all_content_deltas = existing_deltas
            .iter()
            .chain(deltas.iter())
            .collect::<Vec<_>>();
        let part_delta = stream_utils::loosely_convert_part_to_part_delta(part)?;
        let guessed_index = stream_utils::guess_delta_index(&part_delta, &all_content_deltas, None);
        deltas.push(ContentDelta {
            index: guessed_index,
            part: part_delta,
        });
    }

    Ok(deltas)
}

fn map_google_usage_metadata(
    usage: &super::api::GenerateContentResponseUsageMetadata,
) -> ModelUsage {
    let input_tokens = usage.prompt_token_count.unwrap_or(0);
    let output_tokens = usage.candidates_token_count.unwrap_or(0);

    let input_tokens_details = map_modality_token_counts(
        usage.prompt_tokens_details.as_ref(),
        usage.cache_tokens_details.as_ref(),
    );

    let output_tokens_details =
        map_modality_token_counts(usage.candidates_tokens_details.as_ref(), None);

    ModelUsage {
        input_tokens,
        output_tokens,
        input_tokens_details,
        output_tokens_details,
    }
}

fn map_modality_token_counts(
    details: Option<&Vec<ModalityTokenCount>>,
    cached_details: Option<&Vec<ModalityTokenCount>>,
) -> Option<ModelTokensDetails> {
    if details.is_none() && cached_details.is_none() {
        return None;
    }

    let mut tokens_details = ModelTokensDetails {
        text_tokens: None,
        cached_text_tokens: None,
        audio_tokens: None,
        cached_audio_tokens: None,
        image_tokens: None,
        cached_image_tokens: None,
    };

    if let Some(details) = details {
        for detail in details {
            if let (Some(modality), Some(count)) = (&detail.modality, detail.token_count) {
                match modality {
                    MediaModality::Text => {
                        *tokens_details.text_tokens.get_or_insert_default() += count;
                    }
                    MediaModality::Audio => {
                        *tokens_details.audio_tokens.get_or_insert_default() += count;
                    }
                    MediaModality::Image => {
                        *tokens_details.image_tokens.get_or_insert_default() += count;
                    }
                    _ => {}
                }
            }
        }
    }

    if let Some(cached) = cached_details {
        for detail in cached {
            if let (Some(modality), Some(count)) = (&detail.modality, detail.token_count) {
                match modality {
                    MediaModality::Text => {
                        *tokens_details.cached_text_tokens.get_or_insert_default() += count;
                    }
                    MediaModality::Audio => {
                        *tokens_details.cached_audio_tokens.get_or_insert_default() += count;
                    }
                    MediaModality::Image => {
                        *tokens_details.cached_image_tokens.get_or_insert_default() += count;
                    }
                    _ => {}
                }
            }
        }
    }

    Some(tokens_details)
}
