use crate::{openai::responses_api::ReasoningEffort, LanguageModelError};

pub const OPENAI_REASONING_EFFORT_MINIMAL: u32 = 1000;
pub const OPENAI_REASONING_EFFORT_LOW: u32 = 2000;
pub const OPENAI_REASONING_EFFORT_MEDIUM: u32 = 3000;
pub const OPENAI_REASONING_EFFORT_HIGH: u32 = 4000;

impl TryFrom<u32> for ReasoningEffort {
    type Error = LanguageModelError;

    fn try_from(value: u32) -> Result<Self, Self::Error> {
        match value {
            OPENAI_REASONING_EFFORT_MINIMAL => Ok(ReasoningEffort::Minimal),
            OPENAI_REASONING_EFFORT_LOW => Ok(ReasoningEffort::Low),
            OPENAI_REASONING_EFFORT_MEDIUM => Ok(ReasoningEffort::Medium),
            OPENAI_REASONING_EFFORT_HIGH => Ok(ReasoningEffort::High),
            _ => Err(LanguageModelError::Unsupported(
                "openai",
                "Budget tokens property is not supported for OpenAI reasoning. You may use \
                 OPENAI_REASONING_EFFORT_* constants to map it to OpenAI reasoning effort levels."
                    .to_string(),
            )),
        }
    }
}
