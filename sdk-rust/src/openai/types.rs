use crate::{
    openai::responses_api::{ReasoningEffort, ReasoningEffortValue},
    LanguageModelError,
};

pub const OPENAI_REASONING_EFFORT_MINIMAL: u32 = 1000;
pub const OPENAI_REASONING_EFFORT_LOW: u32 = 2000;
pub const OPENAI_REASONING_EFFORT_MEDIUM: u32 = 3000;
pub const OPENAI_REASONING_EFFORT_HIGH: u32 = 4000;

pub fn reasoning_effort_from_budget(value: u32) -> Result<ReasoningEffort, LanguageModelError> {
    let effort = match value {
        OPENAI_REASONING_EFFORT_MINIMAL => ReasoningEffortValue::Minimal,
        OPENAI_REASONING_EFFORT_LOW => ReasoningEffortValue::Low,
        OPENAI_REASONING_EFFORT_MEDIUM => ReasoningEffortValue::Medium,
        OPENAI_REASONING_EFFORT_HIGH => ReasoningEffortValue::High,
        _ => {
            return Err(LanguageModelError::Unsupported(
                "openai",
                "Budget tokens property is not supported for OpenAI reasoning. You may use \
                 OPENAI_REASONING_EFFORT_* constants to map it to OpenAI reasoning effort levels."
                    .to_string(),
            ))
        }
    };

    Ok(Some(Some(effort)))
}
