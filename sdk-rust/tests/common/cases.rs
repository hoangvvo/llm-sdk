use futures::stream::StreamExt;
use llm_sdk::*;
use serde::{de::DeserializeOwned, Deserialize};
use serde_json::{json, Value};
use std::{
    error::Error,
    io::Write,
    path::PathBuf,
    process::{Command, Stdio},
};

#[derive(Debug, Deserialize)]
struct CaseInfo {
    stage_count: usize,
}

#[derive(Debug, Deserialize)]
struct PreparedStage {
    method: String,
    input: Value,
}

fn test_cases_by_group(group: &str) -> Result<Vec<String>, Box<dyn Error>> {
    call_protocol(&json!({
        "command": "list_cases",
        "group": group,
    }))
}

fn protocol_path() -> PathBuf {
    let mut path = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    path.push("../sdk-tests/cli.mjs");
    path
}

fn call_protocol<T: DeserializeOwned>(request: &Value) -> Result<T, Box<dyn Error>> {
    let mut child = Command::new("node")
        .arg(protocol_path())
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()?;
    child
        .stdin
        .take()
        .ok_or_else(|| std::io::Error::other("failed to open sdk-tests stdin"))?
        .write_all(&serde_json::to_vec(request)?)?;
    let output = child.wait_with_output()?;
    if !output.status.success() {
        let message = String::from_utf8_lossy(&output.stderr).trim().to_string();
        return Err(std::io::Error::other(format!("sdk-tests: {message}")).into());
    }
    Ok(serde_json::from_slice(&output.stdout)?)
}

fn tool_call_parts(content: &[Part]) -> Vec<&Part> {
    content
        .iter()
        .filter(|part| matches!(part, Part::ToolCall(_)))
        .collect()
}

#[derive(Clone, Copy, Default)]
pub struct RunTestCaseOptions {
    pub profile: Option<&'static str>,
}

pub async fn run_test_case(
    model: &dyn LanguageModel,
    test_case_name: &str,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    let options = options.unwrap_or_default();
    let info: CaseInfo = call_protocol(&json!({
        "command": "case_info",
        "test_case": test_case_name,
    }))?;
    let mut history: Vec<Message> = Vec::new();
    let mut context = json!({ "stages": [] });

    for stage_index in 0..info.stage_count {
        let stage: PreparedStage = call_protocol(&json!({
            "command": "prepare_stage",
            "test_case": test_case_name,
            "stage": stage_index,
            "context": context,
            "profile": options.profile,
        }))?;
        let mut input: LanguageModelInput = serde_json::from_value(stage.input)?;
        let stage_messages = input.messages.clone();
        input.messages = history.iter().cloned().chain(stage_messages).collect();

        let execution: LanguageModelResult<(ModelResponse, Option<Value>)> = async {
            match stage.method.as_str() {
                "generate" => Ok((model.generate(input.clone()).await?, None)),
                "stream" => {
                    let mut stream = model.stream(input.clone()).await?;
                    let mut accumulator = StreamAccumulator::new();
                    let mut partials = 0;
                    let mut deltas = 0;
                    let mut usage_updates = 0;
                    while let Some(partial_response) = stream.next().await {
                        let partial_response = partial_response?;
                        partials += 1;
                        if partial_response.delta.is_some() {
                            deltas += 1;
                        }
                        if partial_response.usage.is_some() {
                            usage_updates += 1;
                        }
                        accumulator
                            .add_partial(partial_response)
                            .map_err(|error| LanguageModelError::Invariant("sdk-tests", error))?;
                    }
                    Ok((
                        accumulator.compute_response()?,
                        Some(json!({
                            "partials": partials,
                            "deltas": deltas,
                            "usage_updates": usage_updates,
                        })),
                    ))
                }
                method => Err(LanguageModelError::InvalidInput(format!(
                    "unsupported shared test method {method:?}"
                ))),
            }
        }
        .await;
        let (response, stream_metrics) = match execution {
            Ok(result) => result,
            Err(error) => {
                let kind = match &error {
                    LanguageModelError::InvalidInput(_) => "invalid_input",
                    LanguageModelError::Transport(_) => "transport",
                    LanguageModelError::StatusCode(_, _) => "status_code",
                    LanguageModelError::Unsupported(_, _) => "unsupported",
                    LanguageModelError::NotImplemented(_, _) => "not_implemented",
                    LanguageModelError::Invariant(_, _) => "invariant",
                    LanguageModelError::Refusal(_) => "refusal",
                };
                let _: Value = call_protocol(&json!({
                    "command": "validate_error",
                    "test_case": test_case_name,
                    "stage": stage_index,
                    "error": { "kind": kind, "message": error.to_string() },
                    "profile": options.profile,
                }))?;
                return Ok(());
            }
        };
        let assistant_content = response.content.clone();

        let _: Value = call_protocol(&json!({
            "command": "validate_output",
            "test_case": test_case_name,
            "stage": stage_index,
            "content": &assistant_content,
            "response": &response,
            "stream": stream_metrics,
            "profile": options.profile,
        }))?;

        history = input.messages;
        history.push(Message::assistant(assistant_content.clone()));
        let Some(stages) = context.get_mut("stages").and_then(Value::as_array_mut) else {
            return Err(std::io::Error::other("missing stage execution context").into());
        };
        stages.push(json!({
            "assistant": &assistant_content,
            "tool_calls": tool_call_parts(&assistant_content),
        }));
    }

    Ok(())
}

pub async fn run_test_group(
    model: &dyn LanguageModel,
    group: &str,
    options: Option<RunTestCaseOptions>,
) -> Result<(), Box<dyn Error>> {
    let mut failures = Vec::new();
    for test_case_name in test_cases_by_group(group)? {
        if let Err(error) = run_test_case(model, &test_case_name, options).await {
            failures.push(format!(
                "shared test case {test_case_name:?} failed: {error}"
            ));
        }
    }
    if !failures.is_empty() {
        return Err(std::io::Error::other(failures.join("\n")).into());
    }
    Ok(())
}

#[macro_export]
macro_rules! test_set {
    ($model_expr:expr, $test_name:ident) => {
        #[test]
        async fn $test_name() -> Result<(), Box<dyn Error>> {
            let model = $model_expr;
            $crate::common::cases::run_test_case(&model, stringify!($test_name), None).await
        }
    };
    ($model_expr:expr, $test_name:ident, $options:expr) => {
        #[test]
        async fn $test_name() -> Result<(), Box<dyn Error>> {
            let model = $model_expr;
            $crate::common::cases::run_test_case(&model, stringify!($test_name), $options).await
        }
    };
    (ignore = $reason:literal, $model_expr:expr, $test_name:ident) => {
        #[test]
        #[ignore = $reason]
        async fn $test_name() -> Result<(), Box<dyn Error>> {
            let model = $model_expr;
            $crate::common::cases::run_test_case(&model, stringify!($test_name), None).await
        }
    };
    (ignore = $reason:literal, $model_expr:expr, $test_name:ident, $options:expr) => {
        #[test]
        #[ignore = $reason]
        async fn $test_name() -> Result<(), Box<dyn Error>> {
            let model = $model_expr;
            $crate::common::cases::run_test_case(&model, stringify!($test_name), $options).await
        }
    };
}

#[macro_export]
macro_rules! test_group {
    ($model_expr:expr, $group:ident) => {
        #[test]
        async fn $group() -> Result<(), Box<dyn Error>> {
            let model = $model_expr;
            $crate::common::cases::run_test_group(&model, stringify!($group), None).await
        }
    };
    ($model_expr:expr, $group:ident, $options:expr) => {
        #[test]
        async fn $group() -> Result<(), Box<dyn Error>> {
            let model = $model_expr;
            $crate::common::cases::run_test_group(&model, stringify!($group), $options).await
        }
    };
}
