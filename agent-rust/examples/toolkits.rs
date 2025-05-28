use std::{
    env,
    sync::{Arc, Mutex},
    time::Duration,
};

use async_trait::async_trait;
use dotenvy::dotenv;
use llm_agent::{
    Agent, AgentItem, AgentParams, AgentResponse, AgentTool, AgentToolResult, RunSessionRequest,
    Toolkit, ToolkitSession,
};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    Message, Part,
};
use serde::Deserialize;
use tokio::time::sleep;

type VisitorId = &'static str;

type BoxError = Box<dyn std::error::Error + Send + Sync>;

#[derive(Clone, Copy)]
struct RiftContext {
    visitor_id: VisitorId,
}

#[derive(Clone)]
struct RiftManifest {
    visitor_name: &'static str,
    origin_reality: &'static str,
    arrival_signature: &'static str,
    contraband_risk: &'static str,
    sentimental_inventory: &'static [&'static str],
    outstanding_anomalies: &'static [&'static str],
    turbulence_level: &'static str,
    courtesy_note: &'static str,
}

// Mock manifest store to show Toolkit::create_session performing async I/O
// before the session starts.
async fn fetch_rift_manifest(visitor_id: VisitorId) -> Result<Arc<RiftManifest>, BoxError> {
    let manifest = match visitor_id {
        "aurora-shift" => RiftManifest {
            visitor_name: "Captain Lyra Moreno",
            origin_reality: "Aurora-9 Spiral",
            arrival_signature: "slipped in trailing aurora dust and a three-second echo",
            contraband_risk: "elevated",
            sentimental_inventory: &[
                "Chrono Locket (Timeline 12)",
                "Folded star chart annotated in ultraviolet",
            ],
            outstanding_anomalies: &[
                "Glitter fog refuses to obey gravity",
                "Field report cites duplicate footfalls arriving 4s late",
            ],
            turbulence_level: "moderate",
            courtesy_note: "Prefers dry humor, allergic to paradox puns.",
        },
        "ember-paradox" => RiftManifest {
            visitor_name: "Archivist Rune Tal",
            origin_reality: "Ember Paradox Belt",
            arrival_signature: "emerged in a plume of cooled obsidian and smoke",
            contraband_risk: "critical",
            sentimental_inventory: &[
                "Glass bead containing their brother's timeline",
                "A singed manifesto titled 'Do Not Fold'",
            ],
            outstanding_anomalies: &[
                "Customs still waiting on clearance form 88-A",
                "Phoenix feather repeats ignition loop every two minutes",
            ],
            turbulence_level: "volatile",
            courtesy_note: "Responds well to calm checklists and precise handoffs.",
        },
        other => return Err(format!("unknown visitor {other}").into()),
    };

    sleep(Duration::from_millis(60)).await;
    Ok(Arc::new(manifest))
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum Phase {
    Intake,
    Recovery,
    Handoff,
    Closed,
}

struct LostAndFoundState {
    manifest: Arc<RiftManifest>,
    phase: Phase,
    pass_verified: bool,
    tagged_items: Vec<String>,
    prophecy_count: u8,
    drone_deployed: bool,
}

impl LostAndFoundState {
    fn new(manifest: Arc<RiftManifest>) -> Self {
        Self {
            manifest,
            phase: Phase::Intake,
            pass_verified: false,
            tagged_items: Vec::new(),
            prophecy_count: 0,
            drone_deployed: false,
        }
    }
}

// Toolkit session keeps manifest snapshot and mutable workflow flags so each
// turn can surface new prompt/tool sets.
struct LostAndFoundToolkitSession {
    state: Arc<Mutex<LostAndFoundState>>,
}

#[async_trait]
impl ToolkitSession<RiftContext> for LostAndFoundToolkitSession {
    fn system_prompt(&self) -> Option<String> {
        let state = self.state.lock().expect("state poisoned");
        Some(build_prompt(&state))
    }

    fn tools(&self) -> Vec<Arc<dyn AgentTool<RiftContext>>> {
        let snapshot = {
            let state = self.state.lock().expect("state poisoned");
            (
                state.phase,
                state.pass_verified,
                state.tagged_items.len(),
                state.prophecy_count,
            )
        };

        let (phase, pass_verified, tagged_len, prophecy_count) = snapshot;
        if phase == Phase::Closed {
            println!("[Toolkit] Tools for phase {}: <none>", phase_label(phase));
            return Vec::new();
        }

        let mut tools: Vec<Arc<dyn AgentTool<RiftContext>>> = vec![
            Arc::new(StabilizeRiftTool {
                state: Arc::clone(&self.state),
            }),
            Arc::new(LogItemTool {
                state: Arc::clone(&self.state),
            }),
        ];

        if !pass_verified {
            tools.push(Arc::new(VerifyPassTool {
                state: Arc::clone(&self.state),
            }));
        }

        if phase == Phase::Recovery && pass_verified {
            tools.push(Arc::new(SummonRetrievalDroneTool {
                state: Arc::clone(&self.state),
            }));

            if prophecy_count == 0 {
                tools.push(Arc::new(ConsultProphetTool {
                    state: Arc::clone(&self.state),
                }));
            }

            if tagged_len > 0 {
                tools.push(Arc::new(IssueQuantumReceiptTool {
                    state: Arc::clone(&self.state),
                }));
            }
        }

        if phase == Phase::Handoff {
            tools.push(Arc::new(CloseManifestTool {
                state: Arc::clone(&self.state),
            }));
        }

        let names = if tools.is_empty() {
            "<none>".to_string()
        } else {
            tools
                .iter()
                .map(|tool| tool.name())
                .collect::<Vec<_>>()
                .join(", ")
        };
        println!(
            "[Toolkit] Tools for phase {}: {}",
            phase_label(phase),
            names
        );

        tools
    }

    async fn close(self: Box<Self>) -> Result<(), BoxError> {
        Ok(())
    }
}

struct LostAndFoundToolkit;

#[async_trait]
impl Toolkit<RiftContext> for LostAndFoundToolkit {
    async fn create_session(
        &self,
        context: &RiftContext,
    ) -> Result<Box<dyn ToolkitSession<RiftContext> + Send + Sync>, BoxError> {
        let manifest = fetch_rift_manifest(context.visitor_id).await?;
        let state = LostAndFoundState::new(manifest);
        Ok(Box::new(LostAndFoundToolkitSession {
            state: Arc::new(Mutex::new(state)),
        }))
    }
}

fn build_prompt(state: &LostAndFoundState) -> String {
    let manifest = &state.manifest;
    let mut lines = vec![
        "You are the Archivist manning Interdimensional Waypoint Seven's Lost & Found counter."
            .to_string(),
        format!(
            "Visitor: {} from {} ({}).",
            manifest.visitor_name, manifest.origin_reality, manifest.arrival_signature
        ),
        format!(
            "Contraband risk: {}. Turbulence: {}.",
            manifest.contraband_risk, manifest.turbulence_level
        ),
    ];

    if manifest.sentimental_inventory.is_empty() {
        lines.push("Sentimental inventory on file: none".into());
    } else {
        lines.push(format!(
            "Sentimental inventory on file: {}",
            manifest.sentimental_inventory.join("; ")
        ));
    }

    if manifest.outstanding_anomalies.is_empty() {
        lines.push("Outstanding anomalies: none".into());
    } else {
        lines.push(format!(
            "Outstanding anomalies: {}",
            manifest.outstanding_anomalies.join("; ")
        ));
    }

    if state.tagged_items.is_empty() {
        lines.push("No traveler-reported items logged yet; invite concise descriptions.".into());
    } else {
        lines.push(format!(
            "Traveler has logged: {}",
            state.tagged_items.join("; ")
        ));
    }

    if state.drone_deployed {
        lines.push("Retrieval drone currently deployed; acknowledge its status.".into());
    }

    lines.push(format!("Current phase: {}.", phase_label(state.phase)));

    match state.phase {
        Phase::Intake => {
            if !state.pass_verified {
                lines.push(
                    "Stabilise the arrival and prioritise verify_pass before promising retrieval."
                        .into(),
                );
            }
        }
        Phase::Recovery => lines.push(
            "Phase focus: coordinate retrieval. Summon the drone or consult the prophet before \
             issuing a quantum receipt."
                .into(),
        ),
        Phase::Handoff => lines.push(
            "Phase focus: wrap neatly. Close the manifest once receipt status is settled.".into(),
        ),
        Phase::Closed => lines.push(
            "Manifest is archived. No toolkit tools remain; offer a tidy summary and dismiss \
             politely."
                .into(),
        ),
    }

    lines.push("Tone: dry, organised, lightly amused. Reference protocol, not headcanon.".into());
    lines.push(manifest.courtesy_note.into());
    lines.push(
        "When tools are available, invoke exactly one relevant tool before concluding. If none \
         remain, summarise the closure instead."
            .into(),
    );

    lines.join("\n")
}

fn phase_label(phase: Phase) -> &'static str {
    match phase {
        Phase::Intake => "INTAKE",
        Phase::Recovery => "RECOVERY",
        Phase::Handoff => "HANDOFF",
        Phase::Closed => "CLOSED",
    }
}

struct StabilizeRiftTool {
    state: Arc<Mutex<LostAndFoundState>>,
}

#[derive(Deserialize)]
struct StabilizeArgs {
    technique: Option<String>,
}

#[async_trait]
impl AgentTool<RiftContext> for StabilizeRiftTool {
    fn name(&self) -> String {
        "stabilize_rift".into()
    }

    fn description(&self) -> String {
        "Describe how you calm the rift turbulence and reassure the traveler.".into()
    }

    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "technique": {
                    "type": "string",
                    "description": "Optional note about the stabilisation technique used."
                }
            },
            "required": ["technique"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: serde_json::Value,
        _context: &RiftContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, BoxError> {
        let args: StabilizeArgs = serde_json::from_value(args)?;
        let (turbulence, technique_raw) = {
            let state = self.state.lock().expect("state poisoned");
            (
                state.manifest.turbulence_level,
                args.technique.unwrap_or_default(),
            )
        };

        let technique = technique_raw.trim().to_string();

        let mut sentence = format!("I cycle the containment field to damp {turbulence} turbulence");
        if !technique.is_empty() {
            sentence.push_str(&format!(" using {technique}"));
        }
        sentence.push('.');

        println!(
            "[tool] stabilize_rift invoked with technique={}",
            if technique.is_empty() {
                "<none>".to_string()
            } else {
                technique.clone()
            }
        );

        Ok(AgentToolResult {
            content: vec![Part::text(sentence)],
            is_error: false,
        })
    }
}

struct LogItemTool {
    state: Arc<Mutex<LostAndFoundState>>,
}

#[derive(Deserialize)]
struct LogItemArgs {
    item: String,
    #[serde(default)]
    timeline: Option<String>,
}

#[async_trait]
impl AgentTool<RiftContext> for LogItemTool {
    fn name(&self) -> String {
        "log_item".into()
    }

    fn description(&self) -> String {
        "Record a traveler-reported possession so recovery tools know what to fetch.".into()
    }

    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "item": { "type": "string", "description": "Name of the missing item." },
                "timeline": { "type": "string", "description": "Optional timeline or reality tag for the item." }
            },
            "required": ["item", "timeline"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: serde_json::Value,
        _context: &RiftContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, BoxError> {
        let args: LogItemArgs = serde_json::from_value(args)?;
        let mut state = self.state.lock().expect("state poisoned");

        let mut label = args.item;
        if let Some(timeline) = args.timeline {
            let trimmed = timeline.trim();
            if !trimmed.is_empty() {
                label = format!("{label} ({trimmed})");
            }
        }
        state.tagged_items.push(label.clone());
        let ledger = state.tagged_items.join("; ");

        println!("[tool] log_item recorded {label}");

        Ok(AgentToolResult {
            content: vec![Part::text(format!(
                "Logged {label} for retrieval queue. Current ledger: {ledger}."
            ))],
            is_error: false,
        })
    }
}

struct VerifyPassTool {
    state: Arc<Mutex<LostAndFoundState>>,
}

#[derive(Deserialize)]
struct VerifyPassArgs {
    clearance_code: String,
}

#[async_trait]
impl AgentTool<RiftContext> for VerifyPassTool {
    fn name(&self) -> String {
        "verify_pass".into()
    }

    fn description(&self) -> String {
        "Validate the traveler's interdimensional pass to unlock recovery tools.".into()
    }

    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "clearance_code": {
                    "type": "string",
                    "description": "Code supplied by the traveler for verification."
                }
            },
            "required": ["clearance_code"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: serde_json::Value,
        _context: &RiftContext,
        _run_state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, BoxError> {
        let args: VerifyPassArgs = serde_json::from_value(args)?;
        let mut state = self.state.lock().expect("state poisoned");
        state.pass_verified = true;
        state.phase = Phase::Recovery;

        println!(
            "[tool] verify_pass authenticated clearance_code={}",
            args.clearance_code
        );

        Ok(AgentToolResult {
            content: vec![Part::text(format!(
                "Pass authenticated with code {}. Recovery protocols online.",
                args.clearance_code
            ))],
            is_error: false,
        })
    }
}

struct SummonRetrievalDroneTool {
    state: Arc<Mutex<LostAndFoundState>>,
}

#[derive(Deserialize)]
struct SummonDroneArgs {
    #[serde(default)]
    designation: Option<String>,
    #[serde(default)]
    target: Option<String>,
}

#[async_trait]
impl AgentTool<RiftContext> for SummonRetrievalDroneTool {
    fn name(&self) -> String {
        "summon_retrieval_drone".into()
    }

    fn description(&self) -> String {
        "Dispatch a retrieval drone to recover a logged item from the rift queue.".into()
    }

    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "designation": {
                    "type": "string",
                    "description": "Optional drone designation to flavour the dispatch."
                },
                "target": {
                    "type": "string",
                    "description": "Specific item to prioritise; defaults to the first logged item."
                }
            },
            "required": ["designation", "target"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: serde_json::Value,
        _context: &RiftContext,
        _run_state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, BoxError> {
        let args: SummonDroneArgs = serde_json::from_value(args)?;
        let mut state = self.state.lock().expect("state poisoned");
        state.drone_deployed = true;

        let designation = args
            .designation
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
            .unwrap_or_else(|| "Drone Theta".to_string());

        let target = args
            .target
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
            .unwrap_or_else(|| {
                state
                    .tagged_items
                    .first()
                    .cloned()
                    .unwrap_or_else(|| "the most recently logged item".to_string())
            });

        println!(
            "[tool] summon_retrieval_drone dispatched designation={} target={}",
            designation, target
        );

        Ok(AgentToolResult {
            content: vec![Part::text(format!(
                "Dispatched {designation} to retrieve {target}."
            ))],
            is_error: false,
        })
    }
}

struct ConsultProphetTool {
    state: Arc<Mutex<LostAndFoundState>>,
}

#[derive(Deserialize)]
struct ConsultProphetArgs {
    #[serde(default)]
    topic: Option<String>,
}

#[async_trait]
impl AgentTool<RiftContext> for ConsultProphetTool {
    fn name(&self) -> String {
        "consult_prophet_agent".into()
    }

    fn description(&self) -> String {
        "Ping Prophet Sigma for probability guidance when the queue misbehaves.".into()
    }

    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "topic": {
                    "type": "string",
                    "description": "Optional focus question for the prophet agent."
                }
            },
            "required": ["topic"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: serde_json::Value,
        _context: &RiftContext,
        _run_state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, BoxError> {
        let args: ConsultProphetArgs = serde_json::from_value(args)?;
        let mut state = self.state.lock().expect("state poisoned");
        state.prophecy_count = state.prophecy_count.saturating_add(1);

        let anomaly = state
            .manifest
            .outstanding_anomalies
            .first()
            .copied()
            .unwrap_or("no immediate hazards");

        let mut sentence = format!("Prophet Sigma notes anomaly priority: {anomaly}");
        if let Some(topic) = args
            .topic
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
        {
            println!("[tool] consult_prophet_agent requested topic={topic}");
            sentence.push_str(&format!(" while considering {topic}."));
        } else {
            println!("[tool] consult_prophet_agent requested topic=<none>");
            sentence.push('.');
        }

        Ok(AgentToolResult {
            content: vec![Part::text(sentence)],
            is_error: false,
        })
    }
}

struct IssueQuantumReceiptTool {
    state: Arc<Mutex<LostAndFoundState>>,
}

#[derive(Deserialize)]
struct IssueReceiptArgs {
    #[serde(default)]
    recipient: Option<String>,
}

#[async_trait]
impl AgentTool<RiftContext> for IssueQuantumReceiptTool {
    fn name(&self) -> String {
        "issue_quantum_receipt".into()
    }

    fn description(&self) -> String {
        "Generate a quantum receipt confirming which items are cleared for handoff.".into()
    }

    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "recipient": {
                    "type": "string",
                    "description": "Optional recipient line for the receipt header."
                }
            },
            "required": ["recipient"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: serde_json::Value,
        _context: &RiftContext,
        _run_state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, BoxError> {
        let args: IssueReceiptArgs = serde_json::from_value(args)?;
        let mut state = self.state.lock().expect("state poisoned");

        let recipient = args
            .recipient
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
            .unwrap_or_else(|| state.manifest.visitor_name.to_string());

        let items = state.tagged_items.join("; ");
        state.phase = Phase::Handoff;

        println!(
            "[tool] issue_quantum_receipt issued to {} for items={}",
            recipient,
            if items.is_empty() {
                "<none>".to_string()
            } else {
                items.clone()
            }
        );

        Ok(AgentToolResult {
            content: vec![Part::text(format!(
                "Issued quantum receipt to {recipient} for {items}. Handoff phase engaged."
            ))],
            is_error: false,
        })
    }
}

struct CloseManifestTool {
    state: Arc<Mutex<LostAndFoundState>>,
}

#[async_trait]
impl AgentTool<RiftContext> for CloseManifestTool {
    fn name(&self) -> String {
        "close_manifest".into()
    }

    fn description(&self) -> String {
        "Archive the case once items are delivered and note any lingering anomalies.".into()
    }

    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {},
            "required": [],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        _args: serde_json::Value,
        _context: &RiftContext,
        _run_state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, BoxError> {
        let mut state = self.state.lock().expect("state poisoned");
        state.phase = Phase::Closed;

        let anomaly_count = state.manifest.outstanding_anomalies.len();

        println!("[tool] close_manifest archived manifest with anomaly_reminders={anomaly_count}");

        Ok(AgentToolResult {
            content: vec![Part::text(format!(
                "Archived manifest with {anomaly_count} anomaly reminder(s) for facilities."
            ))],
            is_error: false,
        })
    }
}

// Static tool configured directly on the agent to contrast toolkit-provided
// tools.
struct PageSecurityTool;

#[derive(Deserialize)]
struct PageSecurityArgs {
    reason: String,
}

#[async_trait]
impl AgentTool<RiftContext> for PageSecurityTool {
    fn name(&self) -> String {
        "page_security".into()
    }

    fn description(&self) -> String {
        "Escalate to security if contraband risk becomes unmanageable.".into()
    }

    fn parameters(&self) -> llm_sdk::JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "reason": { "type": "string", "description": "Why security needs to step in." }
            },
            "required": ["reason"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: serde_json::Value,
        context: &RiftContext,
        _run_state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, BoxError> {
        let args: PageSecurityArgs = serde_json::from_value(args)?;
        Ok(AgentToolResult {
            content: vec![Part::text(format!(
                "Security paged for {}: {}.",
                context.visitor_id, args.reason
            ))],
            is_error: false,
        })
    }
}

#[tokio::main]
async fn main() -> Result<(), BoxError> {
    dotenv().ok();
    let api_key = env::var("OPENAI_API_KEY")?;
    let model = Arc::new(OpenAIModel::new(
        "gpt-4o-mini",
        OpenAIModelOptions {
            api_key,
            ..Default::default()
        },
    ));

    let agent = Agent::new(
        AgentParams::new("WaypointArchivist", model)
            .add_instruction(
                "You are the archivist at Waypoint Seven's Interdimensional Lost & Found desk."
                    .to_string(),
            )
            .add_instruction(
                "Keep responses under 120 words when possible and stay bone-dry with humour."
                    .to_string(),
            )
            .add_instruction(|ctx: &RiftContext| {
                Ok(format!(
                    "Reference the visitor's manifest supplied by the toolkit for {}. Do not \
                     invent new lore.",
                    ctx.visitor_id
                ))
            })
            .add_instruction(
                "When tools remain, call exactly one per turn before concluding. If tools run \
                 out, summarise the closure instead."
                    .to_string(),
            )
            .add_tool(PageSecurityTool)
            .add_toolkit(LostAndFoundToolkit),
    );

    // Create a RunSession explicitly so the ToolkitSession persists across multiple
    // turns.
    let session = agent
        .create_session(RiftContext {
            visitor_id: "aurora-shift",
        })
        .await?;

    let mut transcript: Vec<AgentItem> = Vec::new();
    let prompts = vec![
        "I just slipped through the rift and my belongings are glittering in the wrong timeline. \
         What now?",
        "The Chrono Locket from Timeline 12 is missing, and the echo lag is getting worse.",
        "The locket links to my sister's echo—anything else before I depart?",
    ];

    for (index, prompt) in prompts.iter().enumerate() {
        println!("\n=== TURN {} ===", index + 1);

        transcript.push(AgentItem::Message(Message::user(vec![Part::text(*prompt)])));

        let mut response: AgentResponse = session
            .run(RunSessionRequest {
                input: transcript.clone(),
            })
            .await?;

        println!("{}", response.text());
        transcript.extend(response.output.drain(..));
    }

    session.close().await?;

    Ok(())
}
