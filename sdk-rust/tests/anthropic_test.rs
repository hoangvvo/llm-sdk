mod common;
use crate::common::cases::RunTestCaseOptions;
use llm_sdk::{anthropic::*, *};
use serde_json::Value;
use std::{env, error::Error, sync::OnceLock};
use tokio::test;

fn anthropic_api_key() -> &'static String {
    static KEY: OnceLock<String> = OnceLock::new();

    KEY.get_or_init(|| {
        dotenvy::dotenv().ok();
        env::var("ANTHROPIC_API_KEY").expect("ANTHROPIC_API_KEY must be set")
    })
}

fn anthropic_model() -> AnthropicModel {
    AnthropicModel::new(
        "claude-sonnet-4-5".to_string(),
        AnthropicModelOptions {
            api_key: anthropic_api_key().clone(),
            ..Default::default()
        },
    )
}

fn patch_anthropic_strict_tool_schemas(input: &mut LanguageModelInput) {
    if let Some(tools) = &mut input.tools {
        for tool in tools {
            tool.parameters = patch_anthropic_tool_schema(&tool.name, tool.parameters.clone());
        }
    }
}

fn patch_anthropic_tool_schema(name: &str, value: Value) -> Value {
    if name != "get_weather" {
        return value;
    }

    let mut parameters = match value {
        Value::Object(map) => map,
        _ => return value,
    };

    let Some(Value::Object(properties)) = parameters.get("properties") else {
        return Value::Object(parameters);
    };

    let Some(Value::Object(preferred_unit)) = properties.get("preferred_unit") else {
        return Value::Object(parameters);
    };

    // Temporary Anthropic test workaround: strict tools currently reject the
    // shared nullable-enum shape on get_weather.preferred_unit in practice.
    let mut patched_properties = properties.clone();
    let mut patched_preferred_unit = preferred_unit.clone();
    patched_preferred_unit.insert("type".to_string(), Value::String("string".to_string()));
    patched_properties.insert(
        "preferred_unit".to_string(),
        Value::Object(patched_preferred_unit),
    );
    parameters.insert("properties".to_string(), Value::Object(patched_properties));
    Value::Object(parameters)
}

fn anthropic_compat_options() -> Option<RunTestCaseOptions> {
    Some(RunTestCaseOptions {
        additional_input: Some(patch_anthropic_strict_tool_schemas),
        ..Default::default()
    })
}

test_set!(anthropic_model(), generate_text, anthropic_compat_options());

test_set!(anthropic_model(), stream_text, anthropic_compat_options());

test_set!(
    anthropic_model(),
    generate_with_system_prompt,
    anthropic_compat_options()
);

test_set!(
    anthropic_model(),
    generate_tool_call,
    anthropic_compat_options()
);

test_set!(
    anthropic_model(),
    stream_tool_call,
    anthropic_compat_options()
);

test_set!(
    anthropic_model(),
    generate_text_from_tool_result,
    anthropic_compat_options()
);

test_set!(
    anthropic_model(),
    stream_text_from_tool_result,
    anthropic_compat_options()
);

test_set!(
    anthropic_model(),
    generate_text_from_image_tool_result,
    anthropic_compat_options()
);

test_set!(
    anthropic_model(),
    generate_parallel_tool_calls,
    anthropic_compat_options()
);

test_set!(
    anthropic_model(),
    stream_parallel_tool_calls,
    anthropic_compat_options()
);

test_set!(
    anthropic_model(),
    stream_parallel_tool_calls_same_name,
    anthropic_compat_options()
);

test_set!(
    anthropic_model(),
    structured_response_format,
    anthropic_compat_options()
);

test_set!(
    anthropic_model(),
    source_part_input,
    anthropic_compat_options()
);

test_set!(
    ignore = "model does not support image generation",
    anthropic_model(),
    generate_image
);

test_set!(
    ignore = "model does not support image generation",
    anthropic_model(),
    stream_image
);

test_set!(
    anthropic_model(),
    generate_image_input,
    anthropic_compat_options()
);

test_set!(
    anthropic_model(),
    stream_image_input,
    anthropic_compat_options()
);

test_set!(
    ignore = "model does not support audio generation",
    anthropic_model(),
    generate_audio
);

test_set!(
    ignore = "model does not support audio generation",
    anthropic_model(),
    stream_audio
);

test_set!(
    anthropic_model(),
    generate_reasoning,
    Some(RunTestCaseOptions {
        additional_input: Some(|input| {
            patch_anthropic_strict_tool_schemas(input);
            input.reasoning = Some(ReasoningOptions {
                enabled: true,
                budget_tokens: Some(3000),
            });
        }),
        ..Default::default()
    })
);

test_set!(
    anthropic_model(),
    stream_reasoning,
    Some(RunTestCaseOptions {
        additional_input: Some(|input| {
            patch_anthropic_strict_tool_schemas(input);
            input.reasoning = Some(ReasoningOptions {
                enabled: true,
                budget_tokens: Some(3000),
            });
        }),
        ..Default::default()
    })
);
