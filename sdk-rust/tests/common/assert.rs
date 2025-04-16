use crate::Part;
use regex::Regex;

#[derive(Debug, Clone)]
pub struct TextPartAssertion {
    pub text: Regex,
}

impl TextPartAssertion {
    pub fn assert(&self, content: &[Part]) -> Result<(), String> {
        let found_part = content.iter().find(|part| {
            if let Part::Text(text_part) = part {
                self.text.is_match(&text_part.text)
            } else {
                false
            }
        });

        if found_part.is_none() {
            return Err(format!(
                "Expected matching text part: {}\nReceived:\n{}",
                self.text,
                serde_json::to_string_pretty(content).unwrap()
            ));
        }

        Ok(())
    }
}

#[derive(Debug, Clone)]
pub enum ToolCallpartAssertionArgPropValue {
    Value(Regex),
}

impl ToolCallpartAssertionArgPropValue {
    pub fn is_matched(&self, actual: &serde_json::Value) -> bool {
        match self {
            Self::Value(regex) => {
                let actual_str = actual.to_string();
                regex.is_match(&actual_str)
            }
        }
    }
}

#[derive(Debug, Clone)]
pub struct ToolCallPartAssertion {
    pub tool_name: String,
    pub args: Vec<(String, ToolCallpartAssertionArgPropValue)>,
}

impl ToolCallPartAssertion {
    pub fn assert(&self, content: &[Part]) -> Result<(), String> {
        let found_part = content.iter().find(|part| {
            if let Part::ToolCall(tool_call_part) = part {
                tool_call_part.tool_name == self.tool_name
                    && self.args.iter().all(|(key, value)| {
                        tool_call_part
                            .args
                            .get(key)
                            .is_some_and(|arg| value.is_matched(arg))
                    })
            } else {
                false
            }
        });

        if found_part.is_none() {
            return Err(format!(
                "Expected matching tool call part:\nExpected tool {} with args \
                 {:#?}\nReceived:\n{}",
                self.tool_name,
                self.args,
                serde_json::to_string_pretty(content).unwrap()
            ));
        }

        Ok(())
    }
}

#[derive(Debug, Clone)]
pub struct AudioPartAssertion {
    pub audio_id: bool,            // Whether the audio ID is present
    pub transcript: Option<Regex>, // Optional transcript to match
}

impl AudioPartAssertion {
    pub fn assert(&self, content: &[Part]) -> Result<(), String> {
        let found_part = content.iter().find(|part| {
            if let Part::Audio(audio_part) = part {
                if audio_part.audio_data.is_empty() {
                    return false; // Audio data must be present
                }
                if self.audio_id && audio_part.id.is_none() {
                    return false; // Audio ID must be present if required
                }
                if let Some(transcript) = &self.transcript {
                    if let Some(audio_transcript) = &audio_part.transcript {
                        if !transcript.is_match(audio_transcript) {
                            return false;
                        }
                    } else {
                        return false; // Transcript must be present if required
                    }
                }
                true
            } else {
                false
            }
        });
        if found_part.is_none() {
            return Err(format!(
                "Expected matching audio part:\nExpected transcript: {:#?}, audio_id present: \
                 {}\nReceived:\n{}",
                self.transcript,
                self.audio_id,
                serde_json::to_string_pretty(content).unwrap()
            ));
        }
        Ok(())
    }
}

#[derive(Debug, Clone)]
pub struct ReasoningPartAssertion {
    pub text: Regex,
}

impl ReasoningPartAssertion {
    pub fn assert(&self, content: &[Part]) -> Result<(), String> {
        let found_part = content.iter().find(|part| {
            if let Part::Reasoning(reasoning_part) = part {
                self.text.is_match(&reasoning_part.text)
            } else {
                false
            }
        });

        if found_part.is_none() {
            return Err(format!(
                "Expected matching reasoning part:\nExpected text: {:#?}\nReceived:\n{}",
                self.text,
                serde_json::to_string_pretty(content).unwrap()
            ));
        }

        Ok(())
    }
}

#[derive(Debug, Clone)]
pub enum PartAssertion {
    Text(TextPartAssertion),
    ToolCall(ToolCallPartAssertion),
    Audio(AudioPartAssertion),
    Reasoning(ReasoningPartAssertion),
}

impl PartAssertion {
    pub fn assert(&self, content: &[Part]) -> Result<(), String> {
        match self {
            Self::Text(text_assertion) => text_assertion.assert(content),
            Self::ToolCall(tool_call_assertion) => tool_call_assertion.assert(content),
            Self::Audio(audio_assertion) => audio_assertion.assert(content),
            Self::Reasoning(reasoning_assertion) => reasoning_assertion.assert(content),
        }
    }
}

#[derive(Debug, Clone)]
pub struct OutputAssertion {
    pub content: Vec<PartAssertion>,
}
