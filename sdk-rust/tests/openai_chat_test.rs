mod common;
use crate::common::cases::RunTestCaseOptions;
use llm_sdk::{openai::*, *};
use std::{env, error::Error, sync::OnceLock};
use tokio::test;

fn openai_api_key() -> &'static String {
    static KEY: OnceLock<String> = OnceLock::new();

    KEY.get_or_init(|| {
        dotenvy::dotenv().ok();
        env::var("OPENAI_API_KEY").expect("OPENAI_API_KEY must be set")
    })
}

fn openai_model() -> OpenAIChatModel {
    OpenAIChatModel::new(
        "gpt-5.6-terra".to_string(),
        OpenAIChatModelOptions {
            api_key: openai_api_key().clone(),
            ..Default::default()
        },
    )
}

fn openai_audio_model() -> OpenAIChatModel {
    OpenAIChatModel::new(
        "gpt-audio-1.5".to_string(),
        OpenAIChatModelOptions {
            api_key: openai_api_key().clone(),
            ..Default::default()
        },
    )
}

fn disable_reasoning(input: &mut LanguageModelInput) {
    input.reasoning = Some(ReasoningOptions {
        enabled: false,
        budget_tokens: None,
    });
}

fn no_reasoning_options() -> Option<RunTestCaseOptions> {
    Some(RunTestCaseOptions {
        additional_input: Some(disable_reasoning),
        ..Default::default()
    })
}

test_set!(openai_model(), generate_text);

test_set!(openai_model(), stream_text);

test_set!(openai_model(), generate_with_system_prompt);

test_set!(openai_model(), generate_tool_call, no_reasoning_options());

test_set!(openai_model(), stream_tool_call, no_reasoning_options());

test_set!(
    openai_model(),
    generate_text_from_tool_result,
    no_reasoning_options()
);

test_set!(
    openai_model(),
    stream_text_from_tool_result,
    no_reasoning_options()
);

test_set!(
    openai_model(),
    generate_parallel_tool_calls,
    no_reasoning_options()
);

test_set!(
    openai_model(),
    stream_parallel_tool_calls,
    no_reasoning_options()
);

test_set!(
    openai_model(),
    stream_parallel_tool_calls_same_name,
    no_reasoning_options()
);

test_set!(openai_model(), structured_response_format);

test_set!(openai_model(), source_part_input, no_reasoning_options());

test_set!(
    ignore = "chat completion api does not support hosted web search",
    openai_model(),
    generate_web_search
);

test_set!(
    ignore = "chat completion api does not support hosted web search",
    openai_model(),
    stream_web_search
);

test_set!(
    ignore = "chat completion api does not support image generation",
    openai_model(),
    generate_image
);

test_set!(
    ignore = "chat completion api does not support image generation",
    openai_model(),
    stream_image
);

test_set!(openai_model(), generate_image_input);

test_set!(openai_model(), stream_image_input);

test_set!(
    openai_audio_model(),
    generate_audio,
    Some(RunTestCaseOptions {
        additional_input: Some(|input| {
            input.audio = Some(AudioOptions {
                format: Some(AudioFormat::Mp3),
                voice: Some("alloy".to_string()),
                ..Default::default()
            });
        }),
        ..Default::default()
    })
);

test_set!(
    openai_audio_model(),
    stream_audio,
    Some(RunTestCaseOptions {
        additional_input: Some(|input| {
            input.audio = Some(AudioOptions {
                format: Some(AudioFormat::Linear16),
                voice: Some("alloy".to_string()),
                ..Default::default()
            });
        }),
        ..Default::default()
    })
);

test_set!(
    ignore = "reasoning is not supported in chat completion api",
    openai_model(),
    generate_reasoning
);

test_set!(
    ignore = "reasoning is not supported in chat completion api",
    openai_model(),
    stream_reasoning
);
