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

        let assistant_content = match stage.method.as_str() {
            "generate" => model.generate(input.clone()).await?.content,
            "stream" => {
                let mut stream = model.stream(input.clone()).await?;
                let mut accumulator = StreamAccumulator::new();
                while let Some(partial_response) = stream.next().await {
                    accumulator.add_partial(partial_response?)?;
                }
                accumulator.compute_response()?.content
            }
            method => {
                return Err(std::io::Error::other(format!(
                    "unsupported shared test method {method:?}"
                ))
                .into());
            }
        };

        let _: Value = call_protocol(&json!({
            "command": "validate_output",
            "test_case": test_case_name,
            "stage": stage_index,
            "content": &assistant_content,
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
