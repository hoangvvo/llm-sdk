use crate::{AssistantMessage, Message, Part, TextPart, ToolMessage, UserMessage};

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

impl From<TextPart> for Part {
    fn from(value: TextPart) -> Self {
        Self::Text(value)
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

impl From<UserMessage> for Message {
    fn from(value: UserMessage) -> Self {
        Self::User(value)
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

impl From<AssistantMessage> for Message {
    fn from(value: AssistantMessage) -> Self {
        Self::Assistant(value)
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

impl From<ToolMessage> for Message {
    fn from(value: ToolMessage) -> Self {
        Self::Tool(value)
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
