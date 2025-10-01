use crate::{
    AssistantMessage, AudioPart, AudioPartDelta, CitationDelta, ImagePart, ImagePartDelta, Message,
    Part, ReasoningPart, ReasoningPartDelta, SourcePart, TextPart, TextPartDelta, ToolCallPart,
    ToolCallPartDelta, ToolMessage, ToolResultPart, UserMessage,
};

impl TextPart {
    pub fn new(text: impl Into<String>) -> Self {
        Self {
            text: text.into(),
            citations: None,
        }
    }

    #[must_use]
    pub fn with_citation(mut self, citation: Vec<crate::Citation>) -> Self {
        self.citations = Some(citation);
        self
    }
}

impl From<&str> for TextPart {
    fn from(value: &str) -> Self {
        Self {
            text: value.to_string(),
            citations: None,
        }
    }
}

impl From<String> for TextPart {
    fn from(value: String) -> Self {
        Self {
            text: value,
            citations: None,
        }
    }
}

impl ImagePart {
    pub fn new(image_data: impl Into<String>, mime_type: impl Into<String>) -> Self {
        Self {
            mime_type: mime_type.into(),
            image_data: image_data.into(),
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
    pub fn new(audio_data: impl Into<String>, format: crate::AudioFormat) -> Self {
        Self {
            audio_data: audio_data.into(),
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
            id: None,
        }
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
            is_error: None,
        }
    }

    #[must_use]
    pub fn with_is_error(mut self, is_error: bool) -> Self {
        self.is_error = Some(is_error);
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

impl Part {
    pub fn text(text: impl Into<String>) -> Self {
        Self::Text(TextPart::new(text))
    }

    pub fn image(image_data: impl Into<String>, mime_type: impl Into<String>) -> Self {
        Self::Image(ImagePart::new(image_data, mime_type))
    }

    pub fn audio(audio_data: impl Into<String>, format: crate::AudioFormat) -> Self {
        Self::Audio(AudioPart::new(audio_data, format))
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
        }
    }

    #[must_use]
    pub fn with_citation_delta(mut self, citation: CitationDelta) -> Self {
        self.citation = Some(citation);
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
    pub fn with_data(mut self, image_data: impl Into<String>) -> Self {
        self.image_data = Some(image_data.into());
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
    pub fn with_data(mut self, audio_data: impl Into<String>) -> Self {
        self.audio_data = Some(audio_data.into());
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
