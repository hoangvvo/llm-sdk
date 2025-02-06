use crate::{
    AssistantMessage, AudioPart, ImagePart, Message, Part, TextPart, ToolCallPart, ToolMessage,
    ToolResultPart, UserMessage,
};

impl TextPart {
    pub fn new(text: impl Into<String>) -> Self {
        Self { text: text.into() }
    }
}

impl From<&str> for TextPart {
    fn from(value: &str) -> Self {
        Self {
            text: value.to_string(),
        }
    }
}

impl From<String> for TextPart {
    fn from(value: String) -> Self {
        Self { text: value }
    }
}

impl ImagePart {
    pub fn new(image_data: impl Into<String>, mime_type: impl Into<String>) -> Self {
        Self {
            mime_type: mime_type.into(),
            image_data: image_data.into(),
            width: None,
            height: None,
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
}

impl AudioPart {
    pub fn new(audio_data: impl Into<String>, format: crate::AudioFormat) -> Self {
        Self {
            audio_data: audio_data.into(),
            format,
            sample_rate: None,
            channels: None,
            transcript: None,
            audio_id: None,
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
    pub fn with_audio_id(mut self, audio_id: impl Into<String>) -> Self {
        self.audio_id = Some(audio_id.into());
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
        }
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
