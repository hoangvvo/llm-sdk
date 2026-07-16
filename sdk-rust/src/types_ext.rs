use crate::{
    AssistantMessage, AudioOptions, AudioPart, AudioPartDelta, CitationDelta, FunctionTool,
    ImagePart, ImagePartDelta, LanguageModelInput, Message, Modality, Part, ReasoningOptions,
    ReasoningPart, ReasoningPartDelta, ResponseFormatOption, SourcePart, TextPart, TextPartDelta,
    Tool, ToolCallPart, ToolCallPartDelta, ToolChoiceOption, ToolMessage, ToolResultPart,
    ToolResultStatus, UserMessage, WebSearchTool, WebSearchUserLocation,
};

impl TextPart {
    pub fn new(text: impl Into<String>) -> Self {
        Self {
            text: text.into(),
            citations: None,
            signature: None,
        }
    }

    #[must_use]
    pub fn with_citation(mut self, citation: Vec<crate::Citation>) -> Self {
        self.citations = Some(citation);
        self
    }

    #[must_use]
    pub fn with_signature(mut self, signature: impl Into<String>) -> Self {
        self.signature = Some(signature.into());
        self
    }
}

impl From<&str> for TextPart {
    fn from(value: &str) -> Self {
        Self {
            text: value.to_string(),
            citations: None,
            signature: None,
        }
    }
}

impl From<String> for TextPart {
    fn from(value: String) -> Self {
        Self {
            text: value,
            citations: None,
            signature: None,
        }
    }
}

impl ImagePart {
    pub fn new(data: impl Into<String>, mime_type: impl Into<String>) -> Self {
        Self {
            mime_type: mime_type.into(),
            data: data.into(),
            width: None,
            height: None,
            id: None,
        }
    }

    #[must_use]
    pub fn with_width(mut self, w: u32) -> Self {
        self.width = Some(w);
        self
    }

    #[must_use]
    pub fn with_height(mut self, h: u32) -> Self {
        self.height = Some(h);
        self
    }

    #[must_use]
    pub fn with_id(mut self, id: impl Into<String>) -> Self {
        self.id = Some(id.into());
        self
    }
}

impl AudioPart {
    pub fn new(data: impl Into<String>, format: crate::AudioFormat) -> Self {
        Self {
            data: data.into(),
            format,
            sample_rate: None,
            channels: None,
            transcript: None,
            id: None,
        }
    }

    #[must_use]
    pub fn with_sample_rate(mut self, rate: u32) -> Self {
        self.sample_rate = Some(rate);
        self
    }

    #[must_use]
    pub fn with_channels(mut self, channels: u32) -> Self {
        self.channels = Some(channels);
        self
    }

    #[must_use]
    pub fn with_transcript(mut self, transcript: impl Into<String>) -> Self {
        self.transcript = Some(transcript.into());
        self
    }

    #[must_use]
    pub fn with_id(mut self, id: impl Into<String>) -> Self {
        self.id = Some(id.into());
        self
    }
}

impl SourcePart {
    pub fn new(source: impl Into<String>, title: impl Into<String>, content: Vec<Part>) -> Self {
        Self {
            source: source.into(),
            title: title.into(),
            content,
        }
    }
}

impl ReasoningPart {
    pub fn new(text: impl Into<String>) -> Self {
        Self {
            text: text.into(),
            id: None,
            signature: None,
        }
    }

    #[must_use]
    pub fn with_signature(mut self, signature: impl Into<String>) -> Self {
        self.signature = Some(signature.into());
        self
    }

    #[must_use]
    pub fn with_id(mut self, id: impl Into<String>) -> Self {
        self.id = Some(id.into());
        self
    }
}

impl ToolCallPart {
    pub fn new(
        tool_call_id: impl Into<String>,
        tool_name: impl Into<String>,
        args: serde_json::Value,
    ) -> Self {
        Self {
            tool_call_id: tool_call_id.into(),
            tool_name: tool_name.into(),
            args,
            signature: None,
            id: None,
        }
    }

    #[must_use]
    pub fn with_signature(mut self, signature: impl Into<String>) -> Self {
        self.signature = Some(signature.into());
        self
    }

    #[must_use]
    pub fn with_id(mut self, id: impl Into<String>) -> Self {
        self.id = Some(id.into());
        self
    }
}

impl ToolResultPart {
    pub fn new(
        tool_call_id: impl Into<String>,
        tool_name: impl Into<String>,
        content: Vec<Part>,
    ) -> Self {
        Self {
            tool_call_id: tool_call_id.into(),
            tool_name: tool_name.into(),
            content,
            status: ToolResultStatus::Completed,
        }
    }

    #[must_use]
    pub fn with_status(mut self, status: ToolResultStatus) -> Self {
        self.status = status;
        self
    }
}

impl From<TextPart> for Part {
    fn from(value: TextPart) -> Self {
        Self::Text(value)
    }
}

impl From<ImagePart> for Part {
    fn from(value: ImagePart) -> Self {
        Self::Image(value)
    }
}

impl From<AudioPart> for Part {
    fn from(value: AudioPart) -> Self {
        Self::Audio(value)
    }
}

impl From<ToolCallPart> for Part {
    fn from(value: ToolCallPart) -> Self {
        Self::ToolCall(value)
    }
}

impl From<ToolResultPart> for Part {
    fn from(value: ToolResultPart) -> Self {
        Self::ToolResult(value)
    }
}

impl From<SourcePart> for Part {
    fn from(value: SourcePart) -> Self {
        Self::Source(value)
    }
}

impl From<ReasoningPart> for Part {
    fn from(value: ReasoningPart) -> Self {
        Self::Reasoning(value)
    }
}

impl FunctionTool {
    pub fn new(
        name: impl Into<String>,
        description: impl Into<String>,
        parameters: crate::JSONSchema,
    ) -> Self {
        Self {
            name: name.into(),
            description: description.into(),
            parameters,
        }
    }
}

impl WebSearchTool {
    #[must_use]
    pub fn new() -> Self {
        Self::default()
    }

    #[must_use]
    pub fn with_allowed_domains(mut self, allowed_domains: Vec<String>) -> Self {
        self.allowed_domains = Some(allowed_domains);
        self
    }

    #[must_use]
    pub fn with_user_location(mut self, user_location: WebSearchUserLocation) -> Self {
        self.user_location = Some(user_location);
        self
    }
}

impl WebSearchUserLocation {
    #[must_use]
    pub fn new() -> Self {
        Self::default()
    }

    #[must_use]
    pub fn with_city(mut self, city: impl Into<String>) -> Self {
        self.city = Some(city.into());
        self
    }

    #[must_use]
    pub fn with_region(mut self, region: impl Into<String>) -> Self {
        self.region = Some(region.into());
        self
    }

    #[must_use]
    pub fn with_country(mut self, country: impl Into<String>) -> Self {
        self.country = Some(country.into());
        self
    }

    #[must_use]
    pub fn with_timezone(mut self, timezone: impl Into<String>) -> Self {
        self.timezone = Some(timezone.into());
        self
    }
}

impl From<FunctionTool> for Tool {
    fn from(value: FunctionTool) -> Self {
        Self::Function(value)
    }
}

impl From<WebSearchTool> for Tool {
    fn from(value: WebSearchTool) -> Self {
        Self::WebSearch(value)
    }
}

impl Part {
    pub fn text(text: impl Into<String>) -> Self {
        Self::Text(TextPart::new(text))
    }

    pub fn image(data: impl Into<String>, mime_type: impl Into<String>) -> Self {
        Self::Image(ImagePart::new(data, mime_type))
    }

    pub fn audio(data: impl Into<String>, format: crate::AudioFormat) -> Self {
        Self::Audio(AudioPart::new(data, format))
    }

    pub fn source(source: impl Into<String>, title: impl Into<String>, content: Vec<Self>) -> Self {
        Self::Source(SourcePart::new(source, title, content))
    }

    pub fn reasoning(text: impl Into<String>) -> Self {
        Self::Reasoning(ReasoningPart::new(text))
    }

    pub fn tool_call(
        tool_call_id: impl Into<String>,
        tool_name: impl Into<String>,
        args: serde_json::Value,
    ) -> Self {
        Self::ToolCall(ToolCallPart::new(tool_call_id, tool_name, args))
    }

    pub fn tool_result(
        tool_call_id: impl Into<String>,
        tool_name: impl Into<String>,
        content: Vec<Self>,
    ) -> Self {
        Self::ToolResult(ToolResultPart::new(tool_call_id, tool_name, content))
    }
}

impl UserMessage {
    pub fn new<I, P>(parts: I) -> Self
    where
        I: IntoIterator<Item = P>,
        P: Into<Part>,
    {
        Self {
            content: parts.into_iter().map(Into::into).collect(),
        }
    }
}

impl AssistantMessage {
    pub fn new<I, P>(parts: I) -> Self
    where
        I: IntoIterator<Item = P>,
        P: Into<Part>,
    {
        Self {
            content: parts.into_iter().map(Into::into).collect(),
        }
    }
}

impl ToolMessage {
    pub fn new<I, P>(parts: I) -> Self
    where
        I: IntoIterator<Item = P>,
        P: Into<Part>,
    {
        Self {
            content: parts.into_iter().map(Into::into).collect(),
        }
    }
}

impl Message {
    pub fn user<I, P>(parts: I) -> Self
    where
        I: IntoIterator<Item = P>,
        P: Into<Part>,
    {
        Self::User(UserMessage::new(parts))
    }
    pub fn assistant<I, P>(parts: I) -> Self
    where
        I: IntoIterator<Item = P>,
        P: Into<Part>,
    {
        Self::Assistant(AssistantMessage::new(parts))
    }

    pub fn tool<I, P>(parts: I) -> Self
    where
        I: IntoIterator<Item = P>,
        P: Into<Part>,
    {
        Self::Tool(ToolMessage::new(parts))
    }
}

impl From<UserMessage> for Message {
    fn from(value: UserMessage) -> Self {
        Self::User(value)
    }
}

impl LanguageModelInput {
    pub fn new<I, M>(messages: I) -> Self
    where
        I: IntoIterator<Item = M>,
        M: Into<Message>,
    {
        Self {
            messages: messages.into_iter().map(Into::into).collect(),
            ..Default::default()
        }
    }

    #[must_use]
    pub fn with_system_prompt(mut self, system_prompt: impl Into<String>) -> Self {
        self.system_prompt = Some(system_prompt.into());
        self
    }

    #[must_use]
    pub fn with_messages<I, M>(mut self, messages: I) -> Self
    where
        I: IntoIterator<Item = M>,
        M: Into<Message>,
    {
        self.messages = messages.into_iter().map(Into::into).collect();
        self
    }

    #[must_use]
    pub fn add_message<M>(mut self, message: M) -> Self
    where
        M: Into<Message>,
    {
        self.messages.push(message.into());
        self
    }

    #[must_use]
    pub fn with_tools<I, T>(mut self, tools: I) -> Self
    where
        I: IntoIterator<Item = T>,
        T: Into<Tool>,
    {
        self.tools = Some(tools.into_iter().map(Into::into).collect());
        self
    }

    #[must_use]
    pub fn add_tool<T>(mut self, tool: T) -> Self
    where
        T: Into<Tool>,
    {
        self.tools.get_or_insert_with(Vec::new).push(tool.into());
        self
    }

    #[must_use]
    pub fn with_tool_choice<T>(mut self, tool_choice: T) -> Self
    where
        T: Into<ToolChoiceOption>,
    {
        self.tool_choice = Some(tool_choice.into());
        self
    }

    #[must_use]
    pub fn with_response_format<T>(mut self, response_format: T) -> Self
    where
        T: Into<ResponseFormatOption>,
    {
        self.response_format = Some(response_format.into());
        self
    }

    #[must_use]
    pub fn with_max_tokens(mut self, max_tokens: u32) -> Self {
        self.max_tokens = Some(max_tokens);
        self
    }

    #[must_use]
    pub fn with_temperature(mut self, temperature: f64) -> Self {
        self.temperature = Some(temperature);
        self
    }

    #[must_use]
    pub fn with_top_p(mut self, top_p: f64) -> Self {
        self.top_p = Some(top_p);
        self
    }

    #[must_use]
    pub fn with_top_k(mut self, top_k: i32) -> Self {
        self.top_k = Some(top_k);
        self
    }

    #[must_use]
    pub fn with_presence_penalty(mut self, presence_penalty: f64) -> Self {
        self.presence_penalty = Some(presence_penalty);
        self
    }

    #[must_use]
    pub fn with_frequency_penalty(mut self, frequency_penalty: f64) -> Self {
        self.frequency_penalty = Some(frequency_penalty);
        self
    }

    #[must_use]
    pub fn with_seed(mut self, seed: i64) -> Self {
        self.seed = Some(seed);
        self
    }

    #[must_use]
    pub fn with_modalities<I>(mut self, modalities: I) -> Self
    where
        I: IntoIterator<Item = Modality>,
    {
        self.modalities = Some(modalities.into_iter().collect());
        self
    }

    #[must_use]
    pub fn add_modality(mut self, modality: Modality) -> Self {
        self.modalities.get_or_insert_with(Vec::new).push(modality);
        self
    }

    #[must_use]
    pub fn with_audio<T>(mut self, audio: T) -> Self
    where
        T: Into<AudioOptions>,
    {
        self.audio = Some(audio.into());
        self
    }

    #[must_use]
    pub fn with_reasoning<T>(mut self, reasoning: T) -> Self
    where
        T: Into<ReasoningOptions>,
    {
        self.reasoning = Some(reasoning.into());
        self
    }

    #[must_use]
    pub fn with_metadata<I, K, V>(mut self, metadata: I) -> Self
    where
        I: IntoIterator<Item = (K, V)>,
        K: Into<String>,
        V: Into<String>,
    {
        self.metadata = Some(
            metadata
                .into_iter()
                .map(|(k, v)| (k.into(), v.into()))
                .collect(),
        );
        self
    }
}

impl From<AssistantMessage> for Message {
    fn from(value: AssistantMessage) -> Self {
        Self::Assistant(value)
    }
}

impl From<ToolMessage> for Message {
    fn from(value: ToolMessage) -> Self {
        Self::Tool(value)
    }
}

impl TextPartDelta {
    pub fn new(text: impl Into<String>) -> Self {
        Self {
            text: text.into(),
            citation: None,
            signature: None,
        }
    }

    #[must_use]
    pub fn with_citation_delta(mut self, citation: CitationDelta) -> Self {
        self.citation = Some(citation);
        self
    }

    #[must_use]
    pub fn with_signature(mut self, signature: impl Into<String>) -> Self {
        self.signature = Some(signature.into());
        self
    }
}

impl CitationDelta {
    #[must_use]
    pub fn with_source(mut self, source: impl Into<String>) -> Self {
        self.source = Some(source.into());
        self
    }

    #[must_use]
    pub fn with_title(mut self, title: impl Into<String>) -> Self {
        self.title = Some(title.into());
        self
    }

    #[must_use]
    pub fn with_cited_text(mut self, cited_text: impl Into<String>) -> Self {
        self.cited_text = Some(cited_text.into());
        self
    }

    #[must_use]
    pub fn with_start_index(mut self, start_index: usize) -> Self {
        self.start_index = Some(start_index);
        self
    }

    #[must_use]
    pub fn with_end_index(mut self, end_index: usize) -> Self {
        self.end_index = Some(end_index);
        self
    }

    #[must_use]
    pub fn with_signature(mut self, signature: impl Into<String>) -> Self {
        self.signature = Some(signature.into());
        self
    }
}

impl ToolCallPartDelta {
    #[must_use]
    pub fn with_tool_call_id(mut self, tool_call_id: impl Into<String>) -> Self {
        self.tool_call_id = Some(tool_call_id.into());
        self
    }

    #[must_use]
    pub fn with_tool_name(mut self, tool_name: impl Into<String>) -> Self {
        self.tool_name = Some(tool_name.into());
        self
    }

    #[must_use]
    pub fn with_args(mut self, args: impl Into<String>) -> Self {
        self.args = Some(args.into());
        self
    }

    #[must_use]
    pub fn with_signature(mut self, signature: impl Into<String>) -> Self {
        self.signature = Some(signature.into());
        self
    }

    #[must_use]
    pub fn with_id(mut self, id: impl Into<String>) -> Self {
        self.id = Some(id.into());
        self
    }
}

impl ImagePartDelta {
    #[must_use]
    pub fn with_mime_type(mut self, mime_type: impl Into<String>) -> Self {
        self.mime_type = Some(mime_type.into());
        self
    }

    #[must_use]
    pub fn with_data(mut self, data: impl Into<String>) -> Self {
        self.data = Some(data.into());
        self
    }

    #[must_use]
    pub fn with_width(mut self, width: u32) -> Self {
        self.width = Some(width);
        self
    }

    #[must_use]
    pub fn with_height(mut self, height: u32) -> Self {
        self.height = Some(height);
        self
    }

    #[must_use]
    pub fn with_id(mut self, id: impl Into<String>) -> Self {
        self.id = Some(id.into());
        self
    }
}

impl AudioPartDelta {
    #[must_use]
    pub fn with_format(mut self, format: crate::AudioFormat) -> Self {
        self.format = Some(format);
        self
    }

    #[must_use]
    pub fn with_data(mut self, data: impl Into<String>) -> Self {
        self.data = Some(data.into());
        self
    }

    #[must_use]
    pub fn with_sample_rate(mut self, sample_rate: u32) -> Self {
        self.sample_rate = Some(sample_rate);
        self
    }

    #[must_use]
    pub fn with_channels(mut self, channels: u32) -> Self {
        self.channels = Some(channels);
        self
    }

    #[must_use]
    pub fn with_transcript(mut self, transcript: impl Into<String>) -> Self {
        self.transcript = Some(transcript.into());
        self
    }

    #[must_use]
    pub fn with_id(mut self, id: impl Into<String>) -> Self {
        self.id = Some(id.into());
        self
    }
}

impl ReasoningPartDelta {
    #[must_use]
    pub fn with_text(mut self, text: impl Into<String>) -> Self {
        self.text = Some(text.into());
        self
    }

    #[must_use]
    pub fn with_id(mut self, id: impl Into<String>) -> Self {
        self.id = Some(id.into());
        self
    }

    #[must_use]
    pub fn with_signature(mut self, signature: impl Into<String>) -> Self {
        self.signature = Some(signature.into());
        self
    }
}
