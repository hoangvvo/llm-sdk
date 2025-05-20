use std::{
    io::Write,
    sync::{Arc, Mutex},
};

use dotenvy::dotenv;
use llm_agent::{Agent, AgentItem, AgentRequest, AgentTool, AgentToolResult};
use llm_sdk::{JSONSchema, Message, Part};
use serde::{Deserialize, Serialize};
use serde_json::json;

#[derive(Clone, Serialize, Deserialize)]
struct PlanItem {
    status: String,
    step: String,
}

#[derive(Default, Clone)]
struct Store {
    m: Arc<Mutex<Vec<PlanItem>>>,
    explanation: Arc<Mutex<String>>,
}
impl Store {
    fn list(&self) -> Vec<PlanItem> {
        self.m.lock().unwrap().clone()
    }
    fn set(&self, next: Vec<PlanItem>, explanation: String) {
        *self.m.lock().unwrap() = next;
        *self.explanation.lock().unwrap() = explanation;
    }
    fn explanation(&self) -> String {
        self.explanation.lock().unwrap().clone()
    }
}

fn format_todos(s: &Store) -> String {
    let list = s.list();
    let mut out = String::new();
    out.push_str(&format!("\n─ PLAN (internal) · {} items\n", list.len()));
    let expl = s.explanation();
    if !expl.is_empty() {
        out.push_str(&format!("Explanation: {}\n", expl));
    }
    if list.is_empty() {
        out.push_str("(empty)\n");
        return out;
    }
    for t in list {
        let sym = match t.status.trim() {
            "in_progress" => "▸",
            "complete" => "✓",
            _ => "○",
        };
        out.push_str(&format!("{} {}\n", sym, t.step));
    }
    out
}

fn clear_and_render(messages: &[String], s: &Store) {
    // Clear screen and position cursor at (1,1)
    print!("\x1B[2J\x1B[1;1H");
    let _ = std::io::stdout().flush();
    if !messages.is_empty() {
        println!("{}\n", messages.join("\n\n"));
    }
    print!("{}", format_todos(s));
    let _ = std::io::stdout().flush();
}

type Ctx = ();

struct UpdatePlan {
    s: Store,
}
#[async_trait::async_trait]
impl AgentTool<Ctx> for UpdatePlan {
    fn name(&self) -> String {
        "update_plan".into()
    }
    fn description(&self) -> String {
        "Replace internal plan with explanation and steps".into()
    }
    fn parameters(&self) -> JSONSchema {
        json!({
            "type":"object",
            "properties":{
                "explanation":{"type":"string"},
                "plan":{
                    "type":"array",
                    "items":{
                        "type":"object",
                        "properties":{
                            "status":{"type":"string","enum":["pending","in_progress","complete"]},
                            "step":{"type":"string"}
                        },
                        "required":["status","step"],
                        "additionalProperties":false
                    }
                }
            },
            "required":["explanation","plan"],
            "additionalProperties":false
        })
    }
    async fn execute(
        &self,
        args: serde_json::Value,
        _ctx: &Ctx,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>> {
        #[derive(Deserialize)]
        struct In {
            explanation: String,
            plan: Vec<PlanItem>,
        }
        let p: In = serde_json::from_value(args)?;
        self.s.set(p.plan.clone(), p.explanation.clone());
        Ok(AgentToolResult {
            content: vec![Part::text(
                json!({"ok": true, "explanation": p.explanation, "plan": p.plan}).to_string(),
            )],
            is_error: false,
        })
    }
}

#[tokio::main]
async fn main() {
    dotenv().ok();
    let model = Arc::new(llm_sdk::openai::OpenAIModel::new(
        "gpt-4o",
        llm_sdk::openai::OpenAIModelOptions {
            api_key: std::env::var("OPENAI_API_KEY").expect("OPENAI_API_KEY must be set"),
            ..Default::default()
        },
    ));

    let store = Store::default();
    let overview = "You are a planner–executor assistant.\nBreak the user's goal into clear, \
                    actionable steps using the tool update_plan (explanation, plan: [{status, \
                    step}]).\nUse the plan strictly as your internal plan: NEVER reveal or \
                    enumerate plan items to the user. Do not mention the words TODO, task list, \
                    or the names of tools.\nKeep user-visible replies concise and focused on \
                    results and next-step confirmations.\nWork iteratively: plan an initial set \
                    of high-level steps, then refine/execute one major step per turn, marking \
                    completed items along the way via tools.\nWhen the work is complete, respond \
                    with the final deliverable and a brief one-paragraph summary of what you did.";

    let agent = Agent::new(
        llm_agent::AgentParams::new("planner-executor", model)
            .add_instruction(overview)
            .add_tool(UpdatePlan { s: store.clone() })
            .max_turns(20),
    );

    let mut items: Vec<AgentItem> = vec![AgentItem::Message(Message::user(vec![Part::text(
        "You are hired to produce a concise PRD (Product Requirements Document) for a travel \
         booking app. Do high-level planning and execution across turns: outline the PRD \
         structure, then draft sections (Overview, Target Users, Core Features, MVP Scope, \
         Non-Goals, Success Metrics, Risks), and finally produce the final PRD in markdown. Keep \
         replies brief and focused on progress/results only.",
    )]))];

    let mut messages: Vec<String> = vec![];
    clear_and_render(&messages, &store);

    loop {
        let res = agent
            .run(AgentRequest {
                context: (),
                input: items.clone(),
            })
            .await
            .expect("run failed");
        let mut visible: Vec<String> = vec![];
        for p in &res.content {
            if let Part::Text(t) = p {
                visible.push(t.text.clone())
            }
        }
        if !visible.is_empty() {
            messages.push(visible.join("\n").trim().to_string());
        }
        clear_and_render(&messages, &store);

        items.extend(res.output);
        let list = store.list();
        let all_done = !list.is_empty() && list.iter().all(|t| t.status.trim() == "complete");
        if all_done {
            break;
        }
        items.push(AgentItem::Message(Message::user(vec![Part::text("NEXT")])))
    }

    clear_and_render(&messages, &store);
}
