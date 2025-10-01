use crate::common::assert::{
    AudioPartAssertion, ImagePartAssertion, OutputAssertion, PartAssertion, ReasoningPartAssertion,
    TextPartAssertion, ToolCallPartAssertion, ToolCallpartAssertionArgPropValue,
};
use futures::stream::StreamExt;
use llm_sdk::*;
use regex::Regex;
use serde::Deserialize;
use serde_json::Value;
use std::{collections::HashMap, error::Error, fs, path::PathBuf, sync::LazyLock};

#[derive(Debug, Clone)]
pub enum TestMethod {
    Generate,
    Stream,
}

#[derive(Debug, Clone)]
pub struct TestCase {
    pub input: LanguageModelInput,
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
    #[serde(rename = "type")]
    test_type: String,
    input: LanguageModelInput,
    #[serde(default)]
    input_tools: Vec<String>,
    output: Value,
}

// Load test data from JSON at compile time
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
    // Input is already a LanguageModelInput from JSON deserialization
    let mut input = tc.input.clone();

    // Handle tools
    if !tc.input_tools.is_empty() {
        let mut resolved_tools = Vec::new();
        for tool_name in &tc.input_tools {
            if let Some(tool) = TOOLS_MAP.get(tool_name) {
                resolved_tools.push(tool.clone());
            }
        }
        if !resolved_tools.is_empty() {
            input.tools = Some(resolved_tools);
        }
    }

    // Convert output
    let mut output = OutputAssertion {
        content: Vec::new(),
    };
    if let Some(content) = tc.output.get("content").and_then(|v| v.as_array()) {
        output.content = convert_output_assertions(content);
    }

    // Determine method
    let method = if tc.test_type == "stream" {
        TestMethod::Stream
    } else {
        TestMethod::Generate
    };

    TestCase {
        input,
        method,
        output,
    }
}

fn convert_output_assertions(content: &[Value]) -> Vec<PartAssertion> {
    let mut assertions = Vec::new();

    for part in content {
        if let Some(part_obj) = part.as_object() {
            let part_type = part_obj.get("type").and_then(|v| v.as_str()).unwrap_or("");

            match part_type {
                "text" => {
                    if let Some(text) = part_obj.get("text").and_then(|v| v.as_str()) {
                        // Always treat as regex
                        assertions.push(PartAssertion::Text(TextPartAssertion {
                            text: Regex::new(text).unwrap(),
                        }));
                    }
                }
                "tool_call" => {
                    let tool_name = part_obj
                        .get("tool_name")
                        .and_then(|v| v.as_str())
                        .unwrap_or("")
                        .to_string();
                    let mut args = Vec::new();
                    if let Some(args_obj) = part_obj.get("args").and_then(|v| v.as_object()) {
                        for (key, value) in args_obj {
                            if let Some(val_str) = value.as_str() {
                                // Always treat as regex
                                args.push((
                                    key.clone(),
                                    ToolCallpartAssertionArgPropValue::Value(
                                        Regex::new(val_str).unwrap(),
                                    ),
                                ));
                            }
                        }
                    }
                    assertions.push(PartAssertion::ToolCall(ToolCallPartAssertion {
                        tool_name,
                        args,
                    }));
                }
                "audio" => {
                    let id = part_obj
                        .get("id")
                        .and_then(serde_json::Value::as_bool)
                        .unwrap_or(false);
                    let transcript = part_obj
                        .get("transcript")
                        .and_then(|v| v.as_str())
                        .map(|t| {
                            // Always treat as regex
                            Regex::new(t).unwrap()
                        });
                    assertions.push(PartAssertion::Audio(AudioPartAssertion { id, transcript }));
                }
                "image" => {
                    let id = part_obj
                        .get("id")
                        .and_then(serde_json::Value::as_bool)
                        .unwrap_or(false);
                    assertions.push(PartAssertion::Image(ImagePartAssertion { id }));
                }
                "reasoning" => {
                    if let Some(text) = part_obj.get("text").and_then(|v| v.as_str()) {
                        assertions.push(PartAssertion::Reasoning(ReasoningPartAssertion {
                            text: Regex::new(text).unwrap(),
                        }));
                    }
                }
                _ => {}
            }
        }
    }

    assertions
}

pub async fn run_test_case(
    model: &dyn LanguageModel,
    test_case_name: &str,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    let test_case = TEST_CASES
        .get(test_case_name)
        .ok_or_else(|| format!("Test case '{test_case_name}' not found"))?
        .clone();

    let mut input = test_case.input.clone();
    let mut output = test_case.output.clone();
    if let Some(opts) = options {
        if let Some(f) = opts.additional_input {
            f(&mut input);
        }
        if let Some(f) = opts.custom_output_content {
            output.content = f(&mut output.content);
        }
    }

    match test_case.method {
        TestMethod::Generate => {
            let result = model.generate(input).await?;
            for part_assertion in output.content {
                part_assertion.assert(&result.content)?;
            }
        }
        TestMethod::Stream => {
            let mut stream = model.stream(input).await?;

            let mut accumulator = StreamAccumulator::new();

            while let Some(partial_response) = stream.next().await {
                let partial_response = partial_response?;
                accumulator.add_partial(partial_response)?;
            }

            let result = accumulator.compute_response()?;
            for part_assertion in output.content {
                part_assertion.assert(&result.content)?;
            }
        }
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
