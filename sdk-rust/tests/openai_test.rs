mod common;
use llm_sdk::{openai::*, *};
use std::{env, error::Error, sync::LazyLock};
use tokio::test;

static OPENAI_MODEL: LazyLock<OpenAIModel> = LazyLock::new(|| {
    dotenvy::dotenv().ok();

    OpenAIModel::new(OpenAIModelOptions {
        model_id: "o1".to_string(),
        api_key: env::var("OPENAI_API_KEY")
            .expect("OPENAI_API_KEY must be set")
            .to_string(),
        ..Default::default()
    })
    .with_metadata(LanguageModelMetadata {
        capabilities: Some(vec![
            LanguageModelCapability::FunctionCalling,
            LanguageModelCapability::ImageInput,
            LanguageModelCapability::StructuredOutput,
        ]),
        ..Default::default()
    })
});

test_set!(OPENAI_MODEL, generate_text);

test_set!(OPENAI_MODEL, stream_text);

test_set!(OPENAI_MODEL, generate_with_system_prompt);

test_set!(OPENAI_MODEL, generate_tool_call);

test_set!(OPENAI_MODEL, stream_tool_call);

test_set!(OPENAI_MODEL, generate_text_from_tool_result);

test_set!(OPENAI_MODEL, stream_text_from_tool_result);

test_set!(OPENAI_MODEL, generate_parallel_tool_calls);

test_set!(OPENAI_MODEL, stream_parallel_tool_calls);

test_set!(OPENAI_MODEL, stream_parallel_tool_calls_same_name);

test_set!(OPENAI_MODEL, structured_response_format);

test_set!(OPENAI_MODEL, source_part_input);

test_set!(OPENAI_MODEL, generate_reasoning);

test_set!(OPENAI_MODEL, stream_reasoning);

test_set!(OPENAI_MODEL, input_reasoning);
