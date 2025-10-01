use async_stream::stream;
use axum::{
    body::Body,
    extract::Json,
    http::{HeaderValue, Response, StatusCode},
    response::{sse::Event, IntoResponse, Sse},
    routing::{get, post},
    Router,
};
use base64::{engine::general_purpose::STANDARD as BASE64_STANDARD, Engine};
use dotenvy::dotenv;
use futures::{future::BoxFuture, StreamExt};
use llm_agent::{Agent, AgentItem, AgentRequest, AgentTool, AgentToolResult, BoxedError, RunState};
use llm_sdk::{
    AudioFormat, LanguageModelMetadata, Message, Part, PartDelta, PartialModelResponse,
    ToolResultPart,
};
use serde::{de, Deserialize, Serialize};
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    convert::Infallible,
    error::Error,
    mem,
    time::{Duration, SystemTime, UNIX_EPOCH},
};
use tokio::net::TcpListener;

mod common;

// ==== Vercel AI SDK types ====

#[derive(Debug, Clone, Copy, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
enum UIMessageRole {
    System,
    User,
    Assistant,
    #[serde(other)]
    Unknown,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct UIMessage {
    id: Option<String>,
    role: UIMessageRole,
    #[serde(default)]
    parts: Vec<UIMessagePart>,
    #[serde(default)]
    metadata: Option<Value>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ChatRequestBody {
    id: Option<String>,
    trigger: Option<String>,
    message_id: Option<String>,
    #[serde(default)]
    messages: Vec<UIMessage>,
    provider: Option<String>,
    model_id: Option<String>,
    metadata: Option<LanguageModelMetadata>,
}

#[derive(Debug)]
enum UIMessagePart {
    Text(TextUIPart),
    Reasoning(ReasoningUIPart),
    DynamicTool(DynamicToolUIPart),
    Tool(ToolUIPart),
    File(FileUIPart),
    Unknown,
}

#[derive(Debug, Deserialize)]
struct TextUIPart {
    text: String,
}

#[derive(Debug, Deserialize)]
struct ReasoningUIPart {
    text: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct DynamicToolUIPart {
    tool_name: String,
    tool_call_id: String,
    input: Option<Value>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ToolUIPart {
    #[serde(skip)]
    type_tag: String,
    state: String,
    tool_call_id: String,
    tool_name: Option<String>,
    input: Option<Value>,
    output: Option<Value>,
    error_text: Option<String>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct FileUIPart {
    url: String,
    media_type: String,
    #[serde(default)]
    filename: Option<String>,
}

impl<'de> Deserialize<'de> for UIMessagePart {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: de::Deserializer<'de>,
    {
        let value = Value::deserialize(deserializer)?;
        let type_str = value
            .get("type")
            .and_then(Value::as_str)
            .ok_or_else(|| de::Error::missing_field("type"))?;
        match type_str {
            "text" => {
                let part: TextUIPart = serde_json::from_value(value).map_err(de::Error::custom)?;
                Ok(Self::Text(part))
            }
            "reasoning" => {
                let part: ReasoningUIPart =
                    serde_json::from_value(value).map_err(de::Error::custom)?;
                Ok(Self::Reasoning(part))
            }
            "dynamic-tool" => {
                let part: DynamicToolUIPart =
                    serde_json::from_value(value).map_err(de::Error::custom)?;
                Ok(Self::DynamicTool(part))
            }
            "file" => {
                let part: FileUIPart = serde_json::from_value(value).map_err(de::Error::custom)?;
                Ok(Self::File(part))
            }
            _ if type_str.starts_with("tool-") => {
                let mut part: ToolUIPart =
                    serde_json::from_value(value.clone()).map_err(de::Error::custom)?;
                part.type_tag = type_str.to_string();
                if part.tool_name.is_none() {
                    let candidate = type_str.trim_start_matches("tool-");
                    if !candidate.is_empty() {
                        part.tool_name = Some(candidate.to_string());
                    }
                }
                Ok(Self::Tool(part))
            }
            _ => Ok(Self::Unknown),
        }
    }
}

impl ToolUIPart {
    fn resolved_tool_name(&self) -> Option<&str> {
        if let Some(name) = &self.tool_name {
            if !name.is_empty() {
                return Some(name);
            }
        }
        let derived = self.type_tag.trim_start_matches("tool-");
        if derived.is_empty() {
            None
        } else {
            Some(derived)
        }
    }
}

// ==== Agent setup ====

#[derive(Clone, Default)]
struct ChatContext;

struct TimeTool;

impl AgentTool<ChatContext> for TimeTool {
    fn name(&self) -> String {
        "get_current_time".to_string()
    }

    fn description(&self) -> String {
        "Get the current server time in ISO 8601 format.".to_string()
    }

    fn parameters(&self) -> Value {
        json!({
            "type": "object",
            "properties": {},
            "additionalProperties": false
        })
    }

    fn execute<'a>(
        &'a self,
        _args: Value,
        _context: &'a ChatContext,
        _state: &'a RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn Error + Send + Sync>>> {
        Box::pin(async move {
            let now = chrono::Utc::now().to_rfc3339();
            Ok(AgentToolResult {
                content: vec![Part::text(now)],
                is_error: false,
            })
        })
    }
}

#[derive(Debug, Deserialize)]
struct WeatherParams {
    location: String,
}

struct WeatherTool;

impl AgentTool<ChatContext> for WeatherTool {
    fn name(&self) -> String {
        "get_local_weather".to_string()
    }

    fn description(&self) -> String {
        "Return a lightweight weather forecast for a given city using mock data.".to_string()
    }

    fn parameters(&self) -> Value {
        json!({
            "type": "object",
            "properties": {
                "location": {
                    "type": "string",
                    "description": "City name to look up weather for."
                }
            },
            "required": ["location"],
            "additionalProperties": false
        })
    }

    fn execute<'a>(
        &'a self,
        args: Value,
        _context: &'a ChatContext,
        _state: &'a RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn Error + Send + Sync>>> {
        Box::pin(async move {
            let params: WeatherParams = serde_json::from_value(args)?;
            let trimmed = params.location.trim();
            let conditions = ["sunny", "cloudy", "rainy", "breezy"];
            let condition = conditions[trimmed.len() % conditions.len()];
            let payload = json!({
                "location": trimmed,
                "condition": condition,
                "temperatureC": 18 + (trimmed.len() % 10),
            });
            Ok(AgentToolResult {
                content: vec![Part::text(payload.to_string())],
                is_error: false,
            })
        })
    }
}

fn create_agent(
    provider: &str,
    model_id: &str,
    metadata: LanguageModelMetadata,
) -> Result<Agent<ChatContext>, BoxedError> {
    let model = common::get_model(provider, model_id, metadata, None)?;
    Ok(Agent::<ChatContext>::builder("UIExampleAgent", model)
        .add_instruction("You are an assistant orchestrated by the llm-agent SDK.")
        .add_instruction("Use the available tools when they can provide better answers.")
        .add_tool(TimeTool)
        .add_tool(WeatherTool)
        .build())
}

// ==== Streaming adapter ====

#[derive(Debug, Serialize)]
#[serde(tag = "type", rename_all = "kebab-case")]
enum UIMessageChunk {
    #[serde(rename_all = "camelCase")]
    Start {
        #[serde(skip_serializing_if = "Option::is_none")]
        message_id: Option<String>,
        #[serde(skip_serializing_if = "Option::is_none")]
        message_metadata: Option<Value>,
    },
    #[serde(rename_all = "camelCase")]
    Finish {
        #[serde(skip_serializing_if = "Option::is_none")]
        message_metadata: Option<Value>,
    },
    StartStep,
    FinishStep,
    #[serde(rename_all = "camelCase")]
    TextStart {
        id: String,
    },
    #[serde(rename_all = "camelCase")]
    TextDelta {
        id: String,
        delta: String,
    },
    #[serde(rename_all = "camelCase")]
    TextEnd {
        id: String,
    },
    #[serde(rename_all = "camelCase")]
    ReasoningStart {
        id: String,
    },
    #[serde(rename_all = "camelCase")]
    ReasoningDelta {
        id: String,
        delta: String,
    },
    #[serde(rename_all = "camelCase")]
    ReasoningEnd {
        id: String,
    },
    #[serde(rename_all = "camelCase")]
    ToolInputStart {
        tool_call_id: String,
        tool_name: String,
    },
    #[serde(rename_all = "camelCase")]
    ToolInputDelta {
        tool_call_id: String,
        input_text_delta: String,
    },
    #[serde(rename_all = "camelCase")]
    ToolInputAvailable {
        tool_call_id: String,
        tool_name: String,
        input: Value,
    },
    #[serde(rename_all = "camelCase")]
    ToolOutputAvailable {
        tool_call_id: String,
        output: Value,
    },
    #[serde(rename_all = "camelCase")]
    Error {
        error_text: String,
    },
}

impl UIMessageChunk {
    fn new_start(message_id: Option<String>, message_metadata: Option<Value>) -> Self {
        Self::Start {
            message_id,
            message_metadata,
        }
    }

    fn new_finish(message_metadata: Option<Value>) -> Self {
        Self::Finish { message_metadata }
    }

    fn to_json_string(&self) -> String {
        serde_json::to_string(self).unwrap_or_else(|_| "{}".to_string())
    }
}

struct ToolCallStreamState {
    tool_call_id: Option<String>,
    tool_name: Option<String>,
    args_buffer: String,
    did_emit_start: bool,
}

impl ToolCallStreamState {
    fn new() -> Self {
        Self {
            tool_call_id: None,
            tool_name: None,
            args_buffer: String::new(),
            did_emit_start: false,
        }
    }
}

/// Bridges `AgentStreamEvent`s back into the Vercel AI SDK data stream
/// protocol. Feed every event from `Agent::run_stream` into `handle_event`
/// so the UI receives the expected chunks.
struct DataStreamProtocolAdapter {
    message_id: String,
    text_state: HashMap<usize, String>,
    reasoning_state: HashMap<usize, String>,
    tool_call_state: HashMap<usize, ToolCallStreamState>,
    text_counter: usize,
    reasoning_counter: usize,
    step_started: bool,
    closed: bool,
}

impl DataStreamProtocolAdapter {
    fn new() -> (Self, UIMessageChunk) {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_nanos();
        let adapter = Self {
            message_id: format!("msg_{nanos}"),
            text_state: HashMap::new(),
            reasoning_state: HashMap::new(),
            tool_call_state: HashMap::new(),
            text_counter: 0,
            reasoning_counter: 0,
            step_started: false,
            closed: false,
        };
        let start = UIMessageChunk::new_start(Some(adapter.message_id.clone()), None);
        (adapter, start)
    }

    fn allocate_text_id(&mut self) -> String {
        self.text_counter += 1;
        format!("text_{}", self.text_counter)
    }

    fn allocate_reasoning_id(&mut self, provided: Option<&str>) -> String {
        if let Some(id) = provided {
            if !id.is_empty() {
                return format!("reasoning_{id}");
            }
        }
        self.reasoning_counter += 1;
        format!("reasoning_{}", self.reasoning_counter)
    }

    fn ensure_step_started(&mut self) -> Option<UIMessageChunk> {
        if self.step_started {
            return None;
        }
        self.step_started = true;
        Some(UIMessageChunk::StartStep)
    }

    fn finish_step(&mut self) -> Vec<UIMessageChunk> {
        if !self.step_started {
            return Vec::new();
        }
        let mut events = self.flush_states();
        events.push(UIMessageChunk::FinishStep);
        self.step_started = false;
        events
    }

    fn flush_states(&mut self) -> Vec<UIMessageChunk> {
        let mut events = Vec::new();

        for state_id in mem::take(&mut self.text_state).into_values() {
            events.push(UIMessageChunk::TextEnd { id: state_id });
        }

        for state_id in mem::take(&mut self.reasoning_state).into_values() {
            events.push(UIMessageChunk::ReasoningEnd { id: state_id });
        }

        for state in mem::take(&mut self.tool_call_state).into_values() {
            if let (Some(tool_call_id), Some(tool_name)) = (state.tool_call_id, state.tool_name) {
                if !state.args_buffer.is_empty() {
                    let input = safe_json_parse(&state.args_buffer);
                    events.push(UIMessageChunk::ToolInputAvailable {
                        tool_call_id,
                        tool_name,
                        input,
                    });
                }
            }
        }

        events
    }

    fn handle_event(&mut self, event: &llm_agent::AgentStreamEvent) -> Vec<UIMessageChunk> {
        match event {
            llm_agent::AgentStreamEvent::Partial(PartialModelResponse {
                delta: Some(delta),
                ..
            }) => {
                let mut events = Vec::new();
                if let Some(start) = self.ensure_step_started() {
                    events.push(start);
                }
                events.extend(self.write_delta(delta));
                events
            }
            llm_agent::AgentStreamEvent::Partial(_) => Vec::new(),
            llm_agent::AgentStreamEvent::Item(item_event) => {
                let mut events = self.finish_step();
                if let AgentItem::Tool(tool) = &item_event.item {
                    if let Some(start) = self.ensure_step_started() {
                        events.push(start);
                    }
                    events.extend(self.write_for_tool_item(tool));
                    events.extend(self.finish_step());
                }
                events
            }
            llm_agent::AgentStreamEvent::Response(_) => Vec::new(),
        }
    }

    fn write_delta(&mut self, delta: &llm_sdk::ContentDelta) -> Vec<UIMessageChunk> {
        match &delta.part {
            PartDelta::Text(text_delta) => {
                self.write_for_text_part(delta.index, text_delta.text.clone())
            }
            PartDelta::Reasoning(reasoning_delta) => self.write_for_reasoning_part(
                delta.index,
                reasoning_delta.text.clone().unwrap_or_default(),
                reasoning_delta.id.clone(),
            ),
            PartDelta::ToolCall(tool_delta) => {
                self.write_for_tool_call_part(delta.index, tool_delta)
            }
            PartDelta::Audio(_) | PartDelta::Image(_) => self.flush_states(),
        }
    }

    fn write_for_text_part(&mut self, index: usize, text: String) -> Vec<UIMessageChunk> {
        let mut events = Vec::new();
        let id = if let Some(existing) = self.text_state.get(&index) {
            existing.clone()
        } else {
            let identifier = self.allocate_text_id();
            self.text_state.insert(index, identifier.clone());
            events.push(UIMessageChunk::TextStart {
                id: identifier.clone(),
            });
            identifier
        };

        events.push(UIMessageChunk::TextDelta { id, delta: text });
        events
    }

    fn write_for_reasoning_part(
        &mut self,
        index: usize,
        text: String,
        id: Option<String>,
    ) -> Vec<UIMessageChunk> {
        let mut events = Vec::new();
        let identifier = if let Some(existing) = self.reasoning_state.get(&index) {
            existing.clone()
        } else {
            let identifier = self.allocate_reasoning_id(id.as_deref());
            self.reasoning_state.insert(index, identifier.clone());
            events.push(UIMessageChunk::ReasoningStart {
                id: identifier.clone(),
            });
            identifier
        };

        events.push(UIMessageChunk::ReasoningDelta {
            id: identifier,
            delta: text,
        });
        events
    }

    fn write_for_tool_call_part(
        &mut self,
        index: usize,
        part: &llm_sdk::ToolCallPartDelta,
    ) -> Vec<UIMessageChunk> {
        let mut events = Vec::new();
        if !self.tool_call_state.contains_key(&index) {
            events.extend(self.flush_states());
            self.tool_call_state
                .insert(index, ToolCallStreamState::new());
        }
        let state = self.tool_call_state.get_mut(&index).expect("tool state");

        if let Some(tool_call_id) = &part.tool_call_id {
            state.tool_call_id = Some(tool_call_id.clone());
        }
        if let Some(tool_name) = &part.tool_name {
            state.tool_name = Some(tool_name.clone());
        }

        if !state.did_emit_start {
            if let (Some(tool_call_id), Some(tool_name)) = (&state.tool_call_id, &state.tool_name) {
                state.did_emit_start = true;
                events.push(UIMessageChunk::ToolInputStart {
                    tool_call_id: tool_call_id.clone(),
                    tool_name: tool_name.clone(),
                });
            }
        }

        if let Some(args_chunk) = &part.args {
            state.args_buffer.push_str(args_chunk);
            if let Some(tool_call_id) = &state.tool_call_id {
                events.push(UIMessageChunk::ToolInputDelta {
                    tool_call_id: tool_call_id.clone(),
                    input_text_delta: args_chunk.clone(),
                });
            }
        }

        events
    }

    fn write_for_tool_item(&mut self, item: &llm_agent::AgentItemTool) -> Vec<UIMessageChunk> {
        let mut events = self.flush_states();

        let mut text_buffer = String::new();
        for part in &item.output {
            if let Part::Text(text_part) = part {
                text_buffer.push_str(&text_part.text);
            }
        }

        let output = if text_buffer.is_empty() {
            serde_json::to_value(&item.output).unwrap_or(Value::Null)
        } else {
            safe_json_parse(&text_buffer)
        };

        events.push(UIMessageChunk::ToolOutputAvailable {
            tool_call_id: item.tool_call_id.clone(),
            output,
        });

        events
    }

    fn emit_error(&self, error_text: &str) -> UIMessageChunk {
        UIMessageChunk::Error {
            error_text: error_text.to_string(),
        }
    }

    fn finish(&mut self) -> Vec<UIMessageChunk> {
        if self.closed {
            return Vec::new();
        }
        let mut events = self.finish_step();
        events.push(UIMessageChunk::new_finish(None));
        self.closed = true;
        events
    }
}

// ==== Adapter helpers ====

fn convert_file_part(part: &FileUIPart) -> Result<Vec<Part>, String> {
    let data = extract_data_payload(&part.url);
    if part.media_type.starts_with("image/") {
        Ok(vec![Part::image(data, &part.media_type)])
    } else if part.media_type.starts_with("audio/") {
        if let Some(format) = map_mime_type_to_audio_format(&part.media_type) {
            Ok(vec![Part::audio(data, format)])
        } else {
            Ok(Vec::new())
        }
    } else if part.media_type.starts_with("text/") {
        let decoded = BASE64_STANDARD
            .decode(data.as_bytes())
            .map_err(|err| format!("Failed to decode text data: {err}"))?;
        let text = String::from_utf8(decoded)
            .map_err(|err| format!("Invalid UTF-8 text payload: {err}"))?;
        Ok(vec![Part::text(text)])
    } else {
        Ok(Vec::new())
    }
}

fn convert_tool_part(part: &ToolUIPart) -> Result<Vec<Part>, String> {
    let tool_name = part
        .resolved_tool_name()
        .ok_or_else(|| "Missing tool name".to_string())?
        .to_string();
    match part.state.as_str() {
        "input-available" => Ok(vec![Part::tool_call(
            &part.tool_call_id,
            &tool_name,
            part.input.clone().unwrap_or(Value::Null),
        )]),
        "output-available" => {
            let call = Part::tool_call(
                &part.tool_call_id,
                &tool_name,
                part.input.clone().unwrap_or(Value::Null),
            );
            let output_text =
                serde_json::to_string(&part.output).unwrap_or_else(|_| "null".to_string());
            let result: Part = ToolResultPart::new(
                &part.tool_call_id,
                &tool_name,
                vec![Part::text(output_text)],
            )
            .into();
            Ok(vec![call, result])
        }
        "output-error" => {
            let call = Part::tool_call(
                &part.tool_call_id,
                &tool_name,
                part.input.clone().unwrap_or(Value::Null),
            );
            let result: Part = ToolResultPart::new(
                &part.tool_call_id,
                &tool_name,
                vec![Part::text(part.error_text.clone().unwrap_or_default())],
            )
            .with_is_error(true)
            .into();
            Ok(vec![call, result])
        }
        _ => Ok(Vec::new()),
    }
}

fn ui_message_part_to_parts(part: &UIMessagePart) -> Result<Vec<Part>, String> {
    match part {
        UIMessagePart::Text(part) => Ok(vec![Part::text(&part.text)]),
        UIMessagePart::Reasoning(part) => Ok(vec![Part::reasoning(part.text.clone())]),
        UIMessagePart::DynamicTool(part) => Ok(vec![Part::tool_call(
            &part.tool_call_id,
            &part.tool_name,
            part.input.clone().unwrap_or(Value::Null),
        )]),
        UIMessagePart::File(part) => convert_file_part(part),
        UIMessagePart::Tool(part) => convert_tool_part(part),
        UIMessagePart::Unknown => Ok(Vec::new()),
    }
}

fn ui_messages_to_messages(messages: &[UIMessage]) -> Result<Vec<Message>, String> {
    let mut history = Vec::new();

    for message in messages {
        match message.role {
            UIMessageRole::User => {
                let mut parts = Vec::new();
                for part in &message.parts {
                    parts.extend(ui_message_part_to_parts(part)?);
                }
                if !parts.is_empty() {
                    history.push(Message::user(parts));
                }
            }
            UIMessageRole::Assistant => {
                for part in &message.parts {
                    for converted in ui_message_part_to_parts(part)? {
                        match converted {
                            Part::Text(_)
                            | Part::Reasoning(_)
                            | Part::Audio(_)
                            | Part::Image(_)
                            | Part::ToolCall(_) => {
                                append_assistant_message(&mut history, converted)
                            }
                            Part::ToolResult(_) => append_tool_message(&mut history, converted),
                            Part::Source(_) => {}
                        }
                    }
                }
            }
            UIMessageRole::System | UIMessageRole::Unknown => {}
        }
    }

    Ok(history)
}

fn append_assistant_message(history: &mut Vec<Message>, part: Part) {
    if let Some(Message::Assistant(assistant)) = history.last_mut() {
        assistant.content.push(part);
        return;
    }

    if history.len() >= 2 {
        let last_index = history.len() - 1;
        let last_is_tool = matches!(history[last_index], Message::Tool(_));
        if last_is_tool {
            if let Some(Message::Assistant(assistant)) = history.get_mut(last_index - 1) {
                assistant.content.push(part);
                return;
            }
        }
    }

    history.push(Message::assistant(vec![part]));
}

fn append_tool_message(history: &mut Vec<Message>, part: Part) {
    if let Some(Message::Tool(tool)) = history.last_mut() {
        tool.content.push(part);
        return;
    }

    history.push(Message::tool(vec![part]));
}

/// Attempts to parse tool arguments or results as JSON. When decoding fails we
/// fall back to the original string so the UI can still render the payload.
fn safe_json_parse(raw: &str) -> Value {
    serde_json::from_str(raw).unwrap_or_else(|_| Value::String(raw.to_string()))
}

fn map_mime_type_to_audio_format(mime_type: &str) -> Option<AudioFormat> {
    let normalized = mime_type.split(';').next().unwrap_or(mime_type).trim();
    match normalized {
        "audio/wav" => Some(AudioFormat::Wav),
        "audio/L16" | "audio/l16" => Some(AudioFormat::Linear16),
        "audio/flac" => Some(AudioFormat::Flac),
        "audio/basic" => Some(AudioFormat::Mulaw),
        "audio/mpeg" => Some(AudioFormat::Mp3),
        "audio/ogg" | "audio/ogg;codecs=\"opus\"" | "audio/ogg; codecs=\"opus\"" => {
            Some(AudioFormat::Opus)
        }
        "audio/aac" => Some(AudioFormat::Aac),
        _ => None,
    }
}

fn extract_data_payload(url: &str) -> String {
    url.split_once(',')
        .map(|(_, data)| data.to_string())
        .unwrap_or_else(|| url.to_string())
}

// ==== HTTP handlers ====

async fn chat_handler(
    Json(body): Json<ChatRequestBody>,
) -> Result<Response<Body>, (StatusCode, String)> {
    let provider = body.provider.unwrap_or_else(|| "openai".to_string());
    let model_id = body.model_id.unwrap_or_else(|| "gpt-4o-mini".to_string());
    let metadata = body.metadata.unwrap_or_default();

    let agent = create_agent(&provider, &model_id, metadata)
        .map_err(|err| (StatusCode::INTERNAL_SERVER_ERROR, err.to_string()))?;

    let history =
        ui_messages_to_messages(&body.messages).map_err(|err| (StatusCode::BAD_REQUEST, err))?;
    let mut items = Vec::with_capacity(history.len());
    for message in history {
        items.push(AgentItem::Message(message));
    }

    let mut stream = agent
        .run_stream(AgentRequest {
            input: items,
            context: ChatContext,
        })
        .await
        .map_err(|err| (StatusCode::INTERNAL_SERVER_ERROR, err.to_string()))?;

    let event_stream = stream! {
        let (mut adapter, start_event) = DataStreamProtocolAdapter::new();
        yield Ok::<Event, Infallible>(Event::default().data(start_event.to_json_string()));

        while let Some(event_result) = stream.next().await {
            match event_result {
                Ok(event) => {
                    for payload in adapter.handle_event(&event) {
                        yield Ok::<Event, Infallible>(
                            Event::default().data(payload.to_json_string()),
                        );
                    }
                }
                Err(err) => {
                    let error_event = adapter.emit_error(&err.to_string());
                    yield Ok::<Event, Infallible>(
                        Event::default().data(error_event.to_json_string()),
                    );
                    break;
                }
            }
        }

        for payload in adapter.finish() {
            yield Ok::<Event, Infallible>(Event::default().data(payload.to_json_string()));
        }

        yield Ok::<Event, Infallible>(Event::default().data("[DONE]"));
    };

    let sse = Sse::new(event_stream)
        .keep_alive(axum::response::sse::KeepAlive::new().interval(Duration::from_secs(15)));
    let mut response = sse.into_response();
    let headers = response.headers_mut();
    headers.insert(
        "x-vercel-ai-ui-message-stream",
        HeaderValue::from_static("v1"),
    );
    headers.insert(
        "cache-control",
        HeaderValue::from_static("no-cache, no-transform"),
    );
    headers.insert("access-control-allow-origin", HeaderValue::from_static("*"));
    headers.insert("connection", HeaderValue::from_static("keep-alive"));

    Ok(response)
}

async fn options_handler() -> impl IntoResponse {
    Response::builder()
        .status(StatusCode::NO_CONTENT)
        .header("access-control-allow-origin", "*")
        .header("access-control-allow-headers", "content-type")
        .header("access-control-allow-methods", "POST, OPTIONS")
        .body(Body::empty())
        .unwrap()
}

async fn not_found() -> impl IntoResponse {
    let body = json!({"error": "Not found"});
    Response::builder()
        .status(StatusCode::NOT_FOUND)
        .header("content-type", "application/json")
        .body(Body::from(body.to_string()))
        .unwrap()
}

// ==== Server bootstrap ====

#[tokio::main]
async fn main() -> Result<(), BoxedError> {
    dotenv().ok();

    let app = Router::new()
        .route("/api/chat", post(chat_handler).options(options_handler))
        .route("/", get(not_found));

    let listener = TcpListener::bind(("0.0.0.0", 8000)).await?;
    println!("AI SDK UI example server listening on http://localhost:8000");
    axum::serve(listener, app.into_make_service()).await?;
    Ok(())
}
