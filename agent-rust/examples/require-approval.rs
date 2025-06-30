use dotenvy::dotenv;
use futures::{future::BoxFuture, StreamExt};
use llm_agent::{
    Agent, AgentError, AgentItem, AgentRequest, AgentStreamEvent, AgentTool, AgentToolResult,
};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    Message, Part,
};
use serde::Deserialize;
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    env,
    error::Error,
    fmt,
    io::{self, Write},
    sync::{Arc, Mutex},
};

// Human-in-the-loop outline with agent primitives:
// 1. Seed the run with a user `AgentItem` and call `Agent::run_stream` so we
//    capture every emitted `AgentStreamEvent` (model messages, tool results,
//    etc.).
// 2. When the tool throws our user-land `RequireApprovalError`, collect the
//    human decision and persist it on the shared RunSession context.
// 3. Repeat step (1) with the accumulated items and mutated context until the
//    tool succeeds or returns an error result that reflects the denial.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum ApprovalStatus {
    Approved,
    Denied,
}

#[derive(Clone, Default)]
struct VaultContext {
    approvals: Arc<Mutex<HashMap<String, ApprovalStatus>>>,
}

impl VaultContext {
    fn status(&self, key: &str) -> Option<ApprovalStatus> {
        self.approvals
            .lock()
            .expect("approvals mutex poisoned")
            .get(key)
            .copied()
    }

    fn set_status(&self, artifact: &str, status: ApprovalStatus) {
        self.approvals
            .lock()
            .expect("approvals mutex poisoned")
            .insert(artifact.to_lowercase(), status);
    }
}

#[derive(Debug)]
struct RequireApprovalError {
    message: String,
    artifact: String,
}

impl RequireApprovalError {
    fn new(artifact: String) -> Self {
        let message =
            format!("Release of {artifact} requires human approval before it can proceed.");
        Self { message, artifact }
    }
}

impl fmt::Display for RequireApprovalError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl Error for RequireApprovalError {}

// Single AgentTool that inspects the context map and interrupts the run without
// touching the Agent implementation. Thrown errors become
// AgentToolExecutionError.
struct UnlockArtifactTool;

#[derive(Deserialize)]
struct UnlockArtifactArgs {
    artifact: String,
}

impl AgentTool<VaultContext> for UnlockArtifactTool {
    fn name(&self) -> String {
        "unlock_artifact".into()
    }

    fn description(&self) -> String {
        "Unlock an artifact for release once a human supervisor has recorded their approval.".into()
    }

    fn parameters(&self) -> llm_sdk::JSONSchema {
        json!({
            "type": "object",
            "properties": {
                "artifact": { "type": "string", "description": "Name of the artifact to release.", "minLength": 1 }
            },
            "required": ["artifact"],
            "additionalProperties": false
        })
    }

    fn execute<'a>(
        &'a self,
        args: Value,
        context: &'a VaultContext,
        _state: &llm_agent::RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn Error + Send + Sync>>> {
        Box::pin(async move {
            let params: UnlockArtifactArgs = serde_json::from_value(args)
                .map_err(|err| Box::new(err) as Box<dyn Error + Send + Sync>)?;
            let artifact = params.artifact.trim().to_string();
            let artifact_key = artifact.to_lowercase();
            match context.status(&artifact_key) {
                None => {
                    Err(Box::new(RequireApprovalError::new(artifact))
                        as Box<dyn Error + Send + Sync>)
                }
                Some(ApprovalStatus::Denied) => Ok(AgentToolResult {
                    content: vec![Part::text(format!(
                        "Release of {artifact} remains blocked until a supervisor approves it."
                    ))],
                    is_error: true,
                }),
                Some(ApprovalStatus::Approved) => Ok(AgentToolResult {
                    content: vec![Part::text(format!(
                        "{artifact} unlocked. Proceed with standard vault handling protocols."
                    ))],
                    is_error: false,
                }),
            }
        })
    }
}

fn build_agent(model: Arc<dyn llm_sdk::LanguageModel + Send + Sync>) -> Agent<VaultContext> {
    Agent::builder("VaultSentinel", model)
        .add_instruction(
            "You supervise the Eon Vault, safeguarding experimental expedition technology.",
        )
        .add_tool(UnlockArtifactTool)
        .build()
}

const INITIAL_PROMPT: &str = "We have an emergency launch window in four hours. Please unlock the \
                              Starlight Compass for the Horizon survey team.";

fn initial_transcript() -> Vec<AgentItem> {
    vec![AgentItem::Message(Message::user(vec![Part::text(
        INITIAL_PROMPT,
    )]))]
}

// Stream one pass of the agent, appending every AgentStreamItemEvent.
async fn run_stream(
    agent: &Agent<VaultContext>,
    transcript: &mut Vec<AgentItem>,
    context: VaultContext,
) -> Result<llm_agent::AgentResponse, AgentError> {
    let mut stream = agent
        .run_stream(AgentRequest {
            context,
            input: transcript.clone(),
        })
        .await?;

    while let Some(event) = stream.next().await {
        match event? {
            AgentStreamEvent::Partial(_) => {}
            AgentStreamEvent::Item(item_event) => {
                // Persist generated items so later iterations operate on the full history.
                transcript.push(item_event.item.clone());
                log_item(&item_event.item);
            }
            AgentStreamEvent::Response(response) => return Ok(response),
        }
    }

    Err(AgentError::Invariant(
        "agent stream completed without emitting a response".into(),
    ))
}

fn log_item(item: &AgentItem) {
    match item {
        AgentItem::Message(message) => {
            let text = render_parts(message_content(message));
            if !text.is_empty() {
                println!("\n[{}] {}", message_role(message), text);
            }
        }
        AgentItem::Model(response) => {
            let text = render_parts(&response.content);
            if !text.is_empty() {
                println!("\n[assistant]\n{}", text);
            }
        }
        AgentItem::Tool(tool) => {
            let input = serde_json::to_string(&tool.input).unwrap_or_else(|_| "{}".into());
            println!("\n[tool:{}]", tool.tool_name);
            println!("  input={input}");
            let output = render_parts(&tool.output);
            if !output.is_empty() {
                println!("  output={}", output);
            }
        }
    }
}

fn prompt_for_approval(artifact: &str) -> ApprovalStatus {
    print!("Grant approval to unlock {artifact}? (y/N) ");
    io::stdout().flush().expect("flush stdout");

    let mut input = String::new();
    if io::stdin().read_line(&mut input).is_err() {
        eprintln!("failed to read input; treating as denial");
        return ApprovalStatus::Denied;
    }

    match input.trim().to_lowercase().as_str() {
        "y" | "yes" => ApprovalStatus::Approved,
        "n" | "no" | "" => ApprovalStatus::Denied,
        _ => {
            println!("Unrecognized response, treating as denied.");
            ApprovalStatus::Denied
        }
    }
}

fn render_parts(parts: &[Part]) -> String {
    parts
        .iter()
        .filter_map(|part| match part {
            Part::Text(text) => {
                let trimmed = text.text.trim();
                if trimmed.is_empty() {
                    None
                } else {
                    Some(trimmed.to_string())
                }
            }
            _ => None,
        })
        .collect::<Vec<_>>()
        .join("\n")
}

fn message_content(message: &Message) -> &Vec<Part> {
    match message {
        Message::User(user) => &user.content,
        Message::Assistant(assistant) => &assistant.content,
        Message::Tool(tool) => &tool.content,
    }
}

fn message_role(message: &Message) -> &'static str {
    match message {
        Message::User(_) => "user",
        Message::Assistant(_) => "assistant",
        Message::Tool(_) => "tool",
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error + Send + Sync>> {
    dotenv().ok();

    let api_key = env::var("OPENAI_API_KEY")?;
    let model = Arc::new(OpenAIModel::new(
        "gpt-4o",
        OpenAIModelOptions {
            api_key,
            ..Default::default()
        },
    ));

    let agent = build_agent(model);
    let mut transcript = initial_transcript();
    println!("[user] {INITIAL_PROMPT}");

    let context = VaultContext::default();

    loop {
        match run_stream(&agent, &mut transcript, context.clone()).await {
            Ok(response) => {
                println!("\nCompleted run.");
                println!("{:#?}", response.content);
                break;
            }
            Err(AgentError::ToolExecution(inner)) => {
                if let Some(approval) = inner.downcast_ref::<RequireApprovalError>() {
                    println!("\n[agent halted] err = {}", approval.message);
                    let decision = prompt_for_approval(&approval.artifact);
                    context.set_status(&approval.artifact, decision);
                    continue;
                }
                return Err(inner);
            }
            Err(err) => return Err(err.into()),
        }
    }

    Ok(())
}
