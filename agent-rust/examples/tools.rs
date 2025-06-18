use dotenvy::dotenv;
use futures::future::BoxFuture;
use llm_agent::{Agent, AgentRequest, AgentTool, AgentToolResult};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    Message, Part,
};
use serde::Deserialize;
use serde_json::Value;
use std::{
    collections::{HashMap, HashSet},
    env,
    error::Error,
    sync::{Arc, Mutex},
};

/// Context shared across tool invocations. Tools mutate this state directly so
/// we can showcase how agents can maintain memory without involving toolkits.
#[derive(Default, Clone)]
struct LostAndFoundContext {
    manifest_id: String,
    archivist: String,
    intake_ledger: Arc<Mutex<HashMap<String, ItemRecord>>>,
    flagged_contraband: Arc<Mutex<HashSet<String>>>,
    receipt_notes: Arc<Mutex<Vec<String>>>,
}

#[derive(Clone)]
struct ItemRecord {
    description: String,
    priority: String,
}

fn create_context() -> LostAndFoundContext {
    LostAndFoundContext {
        manifest_id: "aurora-shift".into(),
        archivist: "Quill".into(),
        intake_ledger: Arc::new(Mutex::new(HashMap::new())),
        flagged_contraband: Arc::new(Mutex::new(HashSet::new())),
        receipt_notes: Arc::new(Mutex::new(Vec::new())),
    }
}

/// `intake_item` mirrors the TypeScript/Go examples: validates input and
/// updates the ledger.
struct IntakeItemTool;

#[derive(Deserialize)]
struct IntakeItemParams {
    item_id: String,
    description: String,
    priority: Option<String>,
}

impl AgentTool<LostAndFoundContext> for IntakeItemTool {
    fn name(&self) -> String {
        "intake_item".into()
    }
    fn description(&self) -> String {
        "Register an item reported by the traveller.".into()
    }
    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "item_id": { "type": "string", "description": "Identifier used on the manifest ledger." },
                "description": { "type": "string", "description": "What the traveller says it looks like." },
                "priority": { "type": "string", "enum": ["standard", "rush"] }
            },
            "required": ["item_id", "description"],
            "additionalProperties": false
        })
    }
    fn execute<'a>(
        &'a self,
        args: Value,
        context: &'a LostAndFoundContext,
        _state: &llm_agent::RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn Error + Send + Sync>>> {
        Box::pin(async move {
            let params: IntakeItemParams = serde_json::from_value(args)?;
            let key = params.item_id.trim().to_lowercase();
            if key.is_empty() {
                return Err("item_id cannot be empty".into());
            }
            let mut ledger = context
                .intake_ledger
                .lock()
                .expect("intake ledger mutex poisoned");
            if ledger.contains_key(&key) {
                return Ok(AgentToolResult {
                    content: vec![Part::text(format!(
                        "Item {} is already on the ledger—confirm the manifest number before \
                         adding duplicates.",
                        params.item_id
                    ))],
                    is_error: true,
                });
            }

            let priority = params.priority.unwrap_or_else(|| "standard".into());
            ledger.insert(
                key,
                ItemRecord {
                    description: params.description.clone(),
                    priority: priority.clone(),
                },
            );

            context
                .receipt_notes
                .lock()
                .expect("receipt notes mutex poisoned")
                .push(format!(
                    "{}: {}{}",
                    params.item_id,
                    params.description,
                    if priority == "rush" {
                        " (rush intake)"
                    } else {
                        ""
                    }
                ));

            Ok(AgentToolResult {
                content: vec![Part::text(format!(
                    "Logged {} as {}. Intake queue now holds {} item(s).",
                    params.description,
                    params.item_id,
                    ledger.len()
                ))],
                is_error: false,
            })
        })
    }
}

/// `flag_contraband` highlights additional validation and shared-state updates.
struct FlagContrabandTool;

#[derive(Deserialize)]
struct FlagContrabandParams {
    item_id: String,
    reason: String,
}

impl AgentTool<LostAndFoundContext> for FlagContrabandTool {
    fn name(&self) -> String {
        "flag_contraband".into()
    }
    fn description(&self) -> String {
        "Escalate a manifest item for contraband review.".into()
    }
    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "item_id": { "type": "string" },
                "reason": { "type": "string" }
            },
            "required": ["item_id", "reason"],
            "additionalProperties": false
        })
    }
    fn execute<'a>(
        &'a self,
        args: Value,
        context: &'a LostAndFoundContext,
        _state: &llm_agent::RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn Error + Send + Sync>>> {
        Box::pin(async move {
            let params: FlagContrabandParams = serde_json::from_value(args)?;
            let key = params.item_id.trim().to_lowercase();

            let ledger = context
                .intake_ledger
                .lock()
                .expect("intake ledger mutex poisoned");
            if !ledger.contains_key(&key) {
                return Ok(AgentToolResult {
                    content: vec![Part::text(format!(
                        "Cannot flag {}; it has not been logged yet. Intake the item first.",
                        params.item_id
                    ))],
                    is_error: true,
                });
            }
            drop(ledger);

            context
                .flagged_contraband
                .lock()
                .expect("flagged contraband mutex poisoned")
                .insert(key);
            context
                .receipt_notes
                .lock()
                .expect("receipt notes mutex poisoned")
                .push(format!(
                    "⚠️ {} held for review: {}",
                    params.item_id, params.reason
                ));

            Ok(AgentToolResult {
                content: vec![Part::text(format!(
                    "{} marked for contraband inspection. Inform security before release.",
                    params.item_id
                ))],
                is_error: false,
            })
        })
    }
}

/// `issue_receipt` summarises everything, returning a final message and
/// clearing state.
struct IssueReceiptTool;

#[derive(Deserialize)]
struct IssueReceiptParams {
    traveller: String,
}

impl AgentTool<LostAndFoundContext> for IssueReceiptTool {
    fn name(&self) -> String {
        "issue_receipt".into()
    }
    fn description(&self) -> String {
        "Publish a receipt for the traveller and clear the manifest ledger.".into()
    }
    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "traveller": { "type": "string" }
            },
            "required": ["traveller"],
            "additionalProperties": false
        })
    }
    fn execute<'a>(
        &'a self,
        args: Value,
        context: &'a LostAndFoundContext,
        _state: &llm_agent::RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn Error + Send + Sync>>> {
        Box::pin(async move {
            let params: IssueReceiptParams = serde_json::from_value(args)?;

            let mut ledger = context
                .intake_ledger
                .lock()
                .expect("intake ledger mutex poisoned");
            if ledger.is_empty() {
                return Ok(AgentToolResult {
                    content: vec![Part::text(format!(
                        "No items pending on manifest {}. Intake something before issuing a \
                         receipt.",
                        context.manifest_id
                    ))],
                    is_error: true,
                });
            }

            let mut cleared = Vec::new();
            {
                let flagged = context
                    .flagged_contraband
                    .lock()
                    .expect("flagged contraband mutex poisoned");
                for (id, record) in ledger.iter() {
                    if !flagged.contains(id) {
                        cleared.push(format!("{} ({})", id, record.description));
                    }
                }
            }

            let mut summary = vec![format!(
                "Receipt for {} on manifest {}:",
                params.traveller, context.manifest_id
            )];
            if cleared.is_empty() {
                summary.push("No items cleared—everything is held for review.".into());
            } else {
                summary.push(format!("Cleared items: {}", cleared.join(", ")));
            }
            {
                let notes = context
                    .receipt_notes
                    .lock()
                    .expect("receipt notes mutex poisoned");
                if !notes.is_empty() {
                    summary.push("Notes:".into());
                    summary.extend(notes.iter().cloned());
                }
            }
            let contraband_count = context
                .flagged_contraband
                .lock()
                .expect("flagged contraband mutex poisoned")
                .len();
            summary.push(format!(
                "{contraband_count} item(s) require contraband follow-up."
            ));

            // Clear state for the next manifest.
            ledger.clear();
            context
                .flagged_contraband
                .lock()
                .expect("flagged contraband mutex poisoned")
                .clear();
            context
                .receipt_notes
                .lock()
                .expect("receipt notes mutex poisoned")
                .clear();

            Ok(AgentToolResult {
                content: vec![Part::text(summary.join("\n"))],
                is_error: false,
            })
        })
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    dotenv().ok();

    let api_key = env::var("OPENAI_API_KEY")?;
    let model = Arc::new(OpenAIModel::new(
        "gpt-4o",
        OpenAIModelOptions {
            api_key,
            ..Default::default()
        },
    ));

    let agent = Agent::builder("WaypointClerk", model)
        .add_instruction(
            "You are the archivist completing intake for Waypoint Seven's Interdimensional Lost & \
             Found desk.",
        )
        .add_instruction(
            "When travellers report belongings, call the available tools to mutate the manifest \
             and then summarise your actions.",
        )
        .add_instruction(
            "If a tool reports an error, acknowledge the issue and guide the traveller \
             appropriately.",
        )
        .add_tool(IntakeItemTool)
        .add_tool(FlagContrabandTool)
        .add_tool(IssueReceiptTool)
        .build();

    // Success path: multiple tools fired in one turn.
    let success_context = create_context();
    let success_response = agent
        .run(AgentRequest {
            context: success_context.clone(),
            input: vec![llm_agent::AgentItem::Message(Message::user(vec![
                Part::text(
                    "Log the Chrono Locket as rush, flag the Folded star chart for contraband, \
                     then issue a receipt for Captain Lyra Moreno.",
                ),
            ]))],
        })
        .await?;

    println!("\n=== SUCCESS RUN ===");
    println!("{success_response:#?}");
    println!("{}", success_response.text());

    // Failure path: illustrate tool error handling.
    let failure_context = create_context();
    let failure_response = agent
        .run(AgentRequest {
            context: failure_context,
            input: vec![llm_agent::AgentItem::Message(Message::user(vec![
                Part::text("Issue a receipt immediately without logging anything."),
            ]))],
        })
        .await?;

    println!("\n=== FAILURE RUN ===");
    println!("{failure_response:#?}");
    println!("{}", failure_response.text());

    Ok(())
}
