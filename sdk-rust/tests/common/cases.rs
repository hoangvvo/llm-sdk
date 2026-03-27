use crate::common::assert::{
    compile_pattern, AudioPartAssertion, ImagePartAssertion, OutputAssertion, PartAssertion,
    ReasoningPartAssertion, TextPartAssertion, ToolCallPartAssertion,
};
use futures::stream::StreamExt;
use json_dotpath::DotPaths;
use llm_sdk::*;
use serde::Deserialize;
use serde_json::{json, Value};
use std::{collections::HashMap, error::Error, fs, path::PathBuf, sync::LazyLock};

#[derive(Debug, Clone)]
pub enum TestMethod {
    Generate,
    Stream,
}

#[derive(Debug, Clone)]
pub struct TestCase {
    pub stages: Vec<TestStage>,
}

#[derive(Debug, Clone)]
pub struct TestStage {
    pub input_template: Value,
    pub method: TestMethod,
    pub output: OutputAssertion,
}

#[derive(Debug, Deserialize)]
struct TestDataJSON {
    tools: Vec<Tool>,
    test_cases: Vec<TestCaseJSON>,
}

#[derive(Debug, Deserialize)]
struct TestCaseJSON {
    name: String,
    stages: Vec<TestStageJSON>,
}

#[derive(Debug, Deserialize)]
struct TestStageJSON {
    #[serde(rename = "type")]
    test_type: String,
    input: Value,
    #[serde(default)]
    input_tools: Vec<String>,
    expect: Value,
}

static TEST_DATA: LazyLock<TestDataJSON> = LazyLock::new(|| {
    let mut path = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    path.push("../sdk-tests/tests.json");

    let data = fs::read_to_string(path).expect("Failed to read test data");
    serde_json::from_str(&data).expect("Failed to parse test data")
});

static TOOLS_MAP: LazyLock<HashMap<String, Tool>> = LazyLock::new(|| {
    let mut map = HashMap::new();
    for tool in &TEST_DATA.tools {
        map.insert(tool.name.clone(), tool.clone());
    }
    map
});

static TEST_CASES: LazyLock<HashMap<String, TestCase>> = LazyLock::new(|| {
    let mut map = HashMap::new();
    for tc in &TEST_DATA.test_cases {
        map.insert(tc.name.clone(), convert_json_to_test_case(tc));
    }
    map
});

fn convert_json_to_test_case(tc: &TestCaseJSON) -> TestCase {
    let stages = tc
        .stages
        .iter()
        .map(|stage| TestStage {
            input_template: build_stage_input_template(&stage.input, &stage.input_tools),
            method: parse_test_method(&stage.test_type),
            output: convert_output(&stage.expect),
        })
        .collect();

    TestCase { stages }
}

fn build_stage_input_template(input: &Value, input_tools: &[String]) -> Value {
    if input_tools.is_empty() {
        return input.clone();
    }

    let mut input_value = input.clone();
    let Value::Object(ref mut object) = input_value else {
        panic!("Stage input must be a JSON object");
    };
    object.insert(
        "tools".to_string(),
        Value::Array(resolve_tools(input_tools)),
    );
    input_value
}

fn parse_test_method(value: &str) -> TestMethod {
    if value == "stream" {
        TestMethod::Stream
    } else {
        TestMethod::Generate
    }
}

fn resolve_tools(tool_names: &[String]) -> Vec<Value> {
    tool_names
        .iter()
        .map(|tool_name| {
            let tool = TOOLS_MAP
                .get(tool_name)
                .unwrap_or_else(|| panic!("Tool {tool_name} not found in test data"));
            serde_json::to_value(tool).expect("Failed to encode tool")
        })
        .collect()
}

fn convert_output(output: &Value) -> OutputAssertion {
    let mut converted = OutputAssertion {
        content: Vec::new(),
    };
    if let Some(content) = output.get("content").and_then(Value::as_array) {
        converted.content = convert_output_assertions(content);
    }
    converted
}

fn convert_output_assertions(content: &[Value]) -> Vec<PartAssertion> {
    let mut assertions = Vec::new();

    for part in content {
        if let Some(part_obj) = part.as_object() {
            let part_type = part_obj.get("type").and_then(|v| v.as_str()).unwrap_or("");

            match part_type {
                "text" => {
                    if let Some(text) = part_obj.get("text").and_then(|v| v.as_str()) {
                        assertions.push(PartAssertion::Text(TextPartAssertion {
                            text: compile_pattern(text),
                        }));
                    }
                }
                "tool_call" => {
                    let tool_name = part_obj
                        .get("tool_name")
                        .and_then(|v| v.as_str())
                        .unwrap_or("")
                        .to_string();
                    let args = part_obj
                        .get("args")
                        .and_then(|v| v.as_str())
                        .map(compile_pattern);
                    assertions.push(PartAssertion::ToolCall(ToolCallPartAssertion {
                        tool_name,
                        args,
                    }));
                }
                "audio" => {
                    let id = part_obj.get("id").and_then(Value::as_bool).unwrap_or(false);
                    let transcript = part_obj
                        .get("transcript")
                        .and_then(|v| v.as_str())
                        .map(compile_pattern);
                    assertions.push(PartAssertion::Audio(AudioPartAssertion { id, transcript }));
                }
                "image" => {
                    let id = part_obj.get("id").and_then(Value::as_bool).unwrap_or(false);
                    assertions.push(PartAssertion::Image(ImagePartAssertion { id }));
                }
                "reasoning" => {
                    if let Some(text) = part_obj.get("text").and_then(|v| v.as_str()) {
                        assertions.push(PartAssertion::Reasoning(ReasoningPartAssertion {
                            text: compile_pattern(text),
                        }));
                    }
                }
                _ => {}
            }
        }
    }

    assertions
}

fn resolve_stage_input(
    input_template: &Value,
    context: &Value,
) -> Result<LanguageModelInput, Box<dyn Error>> {
    let resolved = resolve_stage_refs(input_template, context)?;
    Ok(serde_json::from_value(resolved)?)
}

fn resolve_stage_refs(value: &Value, root: &Value) -> Result<Value, Box<dyn Error>> {
    match value {
        Value::Array(items) => {
            let mut resolved = Vec::with_capacity(items.len());
            for item in items {
                resolved.push(resolve_stage_refs(item, root)?);
            }
            Ok(Value::Array(resolved))
        }
        Value::Object(object) => {
            if let Some(path) = get_stage_ref_path(object) {
                return resolve_ref_path(path, root);
            }

            let mut resolved = serde_json::Map::with_capacity(object.len());
            for (key, child) in object {
                resolved.insert(key.clone(), resolve_stage_refs(child, root)?);
            }
            Ok(Value::Object(resolved))
        }
        _ => Ok(value.clone()),
    }
}

fn get_stage_ref_path(object: &serde_json::Map<String, Value>) -> Option<&str> {
    if object.len() != 1 {
        return None;
    }
    object.get("$ref").and_then(Value::as_str)
}

fn resolve_ref_path(path: &str, root: &Value) -> Result<Value, Box<dyn Error>> {
    root.dot_get::<Value>(path)?
        .ok_or_else(|| std::io::Error::other(format!("Invalid stage ref path '{path}'")).into())
}

fn tool_call_parts_value(content: &[Part]) -> Value {
    Value::Array(
        content
            .iter()
            .filter_map(|part| match part {
                Part::ToolCall(tool_call) => Some(serde_json::to_value(tool_call).ok()),
                _ => None,
            })
            .flatten()
            .map(|value| {
                let Value::Object(mut object) = value else {
                    unreachable!("tool call part must serialize to object");
                };
                object.insert("type".to_string(), Value::String("tool-call".to_string()));
                Value::Object(object)
            })
            .collect(),
    )
}

pub async fn run_test_case(
    model: &dyn LanguageModel,
    test_case_name: &str,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    let test_case = TEST_CASES
        .get(test_case_name)
        .ok_or_else(|| std::io::Error::other(format!("Test case '{test_case_name}' not found")))?
        .clone();

    let mut history: Vec<Message> = Vec::new();
    let mut context = json!({ "stages": [] });

    for stage in test_case.stages {
        let mut input = resolve_stage_input(&stage.input_template, &context)?;
        let stage_messages = input.messages.clone();
        input.messages = history
            .iter()
            .cloned()
            .chain(stage_messages.iter().cloned())
            .collect();

        let mut request_input = input.clone();
        let mut output = stage.output.clone();
        if let Some(opts) = &options {
            if let Some(f) = opts.additional_input {
                f(&mut request_input);
            }
            if let Some(f) = opts.custom_output_content {
                output.content = f(&mut output.content);
            }
        }

        let assistant_content = match stage.method {
            TestMethod::Generate => {
                let result = model.generate(request_input).await?;
                for part_assertion in output.content {
                    part_assertion.assert(&result.content)?;
                }
                result.content
            }
            TestMethod::Stream => {
                let mut stream = model.stream(request_input).await?;
                let mut accumulator = StreamAccumulator::new();

                while let Some(partial_response) = stream.next().await {
                    let partial_response = partial_response?;
                    accumulator.add_partial(partial_response)?;
                }

                let result = accumulator.compute_response()?;
                for part_assertion in output.content {
                    part_assertion.assert(&result.content)?;
                }
                result.content
            }
        };
        let tool_calls = tool_call_parts_value(&assistant_content);

        history = input.messages.clone();
        history.push(Message::assistant(assistant_content.clone()));

        let Some(stages) = context.get_mut("stages").and_then(Value::as_array_mut) else {
            return Err("missing stage execution context".into());
        };
        stages.push(json!({
            "assistant": assistant_content,
            "tool_calls": tool_calls,
        }));
    }

    Ok(())
}

#[derive(Clone, Default)]
pub struct RunTestCaseOptions {
    pub additional_input: Option<fn(&mut LanguageModelInput)>,
    pub custom_output_content: Option<fn(&mut Vec<PartAssertion>) -> Vec<PartAssertion>>,
}

pub async fn test_generate_text(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "generate_text", options).await
}

pub async fn test_stream_text(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "stream_text", options).await
}

pub async fn test_generate_with_system_prompt(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "generate_with_system_prompt", options).await
}

pub async fn test_generate_tool_call(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "generate_tool_call", options).await
}

pub async fn test_stream_tool_call(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "stream_tool_call", options).await
}

pub async fn test_generate_text_from_tool_result(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "generate_text_from_tool_result", options).await
}

pub async fn test_stream_text_from_tool_result(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "stream_text_from_tool_result", options).await
}

#[allow(dead_code)]
pub async fn test_generate_text_from_image_tool_result(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "generate_text_from_image_tool_result", options).await
}

pub async fn test_generate_parallel_tool_calls(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "generate_parallel_tool_calls", options).await
}

pub async fn test_stream_parallel_tool_calls(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "stream_parallel_tool_calls", options).await
}

pub async fn test_stream_parallel_tool_calls_same_name(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "stream_parallel_tool_calls_of_same_name", options).await
}

pub async fn test_structured_response_format(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "structured_response_format", options).await
}

pub async fn test_source_part_input(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "source_part_input", options).await
}

pub async fn test_generate_image(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "generate_image", options).await
}

pub async fn test_stream_image(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "stream_image", options).await
}

pub async fn test_generate_image_input(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "generate_image_input", options).await
}

pub async fn test_stream_image_input(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "stream_image_input", options).await
}

pub async fn test_generate_audio(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "generate_audio", options).await
}

pub async fn test_stream_audio(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "stream_audio", options).await
}

pub async fn test_generate_reasoning(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "generate_reasoning", options).await
}

pub async fn test_stream_reasoning(
    model: &dyn LanguageModel,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    run_test_case(model, "stream_reasoning", options).await
}

#[macro_export]
macro_rules! test_set {
    ($model_expr:expr, $test_name:ident) => {
        #[test]
        async fn $test_name() -> Result<(), Box<dyn Error>> {
            let model = $model_expr;
            paste::paste! {
                $crate::common::cases::[<test_$test_name>](&model, None).await
            }
        }
    };
    ($model_expr:expr, $test_name:ident, $options:expr) => {
        #[test]
        async fn $test_name() -> Result<(), Box<dyn Error>> {
            let model = $model_expr;
            paste::paste! {
                $crate::common::cases::[<test_$test_name>](&model, $options).await
            }
        }
    };
    (ignore = $reason:literal, $model_expr:expr, $test_name:ident) => {
        #[test]
        #[ignore = $reason]
        async fn $test_name() -> Result<(), Box<dyn Error>> {
            let model = $model_expr;
            paste::paste! {
                $crate::common::cases::[<test_$test_name>](&model, None).await
            }
        }
    };
    (ignore = $reason:literal, $model_expr:expr, $test_name:ident, $options:expr) => {
        #[test]
        #[ignore = $reason]
        async fn $test_name() -> Result<(), Box<dyn Error>> {
            let model = $model_expr;
            paste::paste! {
                $crate::common::cases::[<test_$test_name>](&model, $options).await
            }
        }
    };
}
