mod common;
use llm_sdk::{openai::*, *};
use std::{env, error::Error, sync::LazyLock};
use tokio::test;

static MODEL: LazyLock<OpenAIModel> = LazyLock::new(|| {
    dotenvy::dotenv().ok();

    OpenAIModel::new(
        "gpt-4o".to_string(),
        OpenAIModelOptions {
            api_key: env::var("OPENAI_API_KEY")
                .expect("OPENAI_API_KEY must be set")
                .to_string(),
            ..Default::default()
        },
    )
});

static REASONING_MODEL: LazyLock<OpenAIModel> = LazyLock::new(|| {
    dotenvy::dotenv().ok();

    OpenAIModel::new(
        "o1".to_string(),
        OpenAIModelOptions {
            api_key: env::var("OPENAI_API_KEY")
                .expect("OPENAI_API_KEY must be set")
                .to_string(),
            ..Default::default()
        },
    )
});

test_set!(MODEL, generate_text);

test_set!(MODEL, stream_text);

test_set!(MODEL, generate_with_system_prompt);

test_set!(MODEL, generate_tool_call);

test_set!(MODEL, stream_tool_call);

test_set!(MODEL, generate_text_from_tool_result);

test_set!(MODEL, stream_text_from_tool_result);

test_set!(MODEL, generate_parallel_tool_calls);

test_set!(MODEL, stream_parallel_tool_calls);

test_set!(MODEL, stream_parallel_tool_calls_same_name);

test_set!(MODEL, structured_response_format);

test_set!(MODEL, source_part_input);

test_set!(
    ignore = "audio is not supported in responses api",
    MODEL,
    generate_audio
);

test_set!(
    ignore = "audio is not supported in responses api",
    MODEL,
    stream_audio
);

test_set!(REASONING_MODEL, generate_reasoning);

test_set!(REASONING_MODEL, stream_reasoning);
