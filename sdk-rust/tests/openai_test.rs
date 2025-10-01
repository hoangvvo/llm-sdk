mod common;
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

fn openai_model() -> OpenAIModel {
    OpenAIModel::new(
        "gpt-4o".to_string(),
        OpenAIModelOptions {
            api_key: openai_api_key().clone(),
            ..Default::default()
        },
    )
}

fn openai_reasoning_model() -> OpenAIModel {
    OpenAIModel::new(
        "o1".to_string(),
        OpenAIModelOptions {
            api_key: openai_api_key().clone(),
            ..Default::default()
        },
    )
}

test_set!(openai_model(), generate_text);

test_set!(openai_model(), stream_text);

test_set!(openai_model(), generate_with_system_prompt);

test_set!(openai_model(), generate_tool_call);

test_set!(openai_model(), stream_tool_call);

test_set!(openai_model(), generate_text_from_tool_result);

test_set!(openai_model(), stream_text_from_tool_result);

test_set!(openai_model(), generate_parallel_tool_calls);

test_set!(openai_model(), stream_parallel_tool_calls);

test_set!(openai_model(), stream_parallel_tool_calls_same_name);

test_set!(openai_model(), structured_response_format);

test_set!(openai_model(), source_part_input);

test_set!(openai_model(), generate_image);

test_set!(openai_model(), stream_image);

test_set!(openai_model(), generate_image_input);

test_set!(openai_model(), stream_image_input);

test_set!(
    ignore = "audio is not supported in responses api",
    openai_model(),
    generate_audio
);

test_set!(
    ignore = "audio is not supported in responses api",
    openai_model(),
    stream_audio
);

test_set!(openai_reasoning_model(), generate_reasoning);

test_set!(openai_reasoning_model(), stream_reasoning);
