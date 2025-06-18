use dotenvy::dotenv;
use futures::future::BoxFuture;
use llm_agent::{Agent, AgentItem, AgentRequest, AgentTool, AgentToolResult, InstructionParam};
use llm_sdk::{JSONSchema, Message, Part};
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::{
    collections::HashMap,
    sync::{Arc, Mutex},
};
// Memory pattern example: core + archival memory tools and instructions.

#[derive(Clone, Default)]
struct Store {
    core: Arc<Mutex<HashMap<String, String>>>,
    archival: Arc<Mutex<HashMap<String, String>>>,
}

#[derive(Clone, Serialize, Deserialize)]
struct MemoryBlock {
    id: String,
    content: String,
}

impl Store {
    fn fetch_core(&self) -> Vec<MemoryBlock> {
        self.core
            .lock()
            .unwrap()
            .iter()
            .map(|(id, content)| MemoryBlock {
                id: id.clone(),
                content: content.clone(),
            })
            .collect()
    }
    fn update_core(&self, b: MemoryBlock) -> Vec<MemoryBlock> {
        let mut core = self.core.lock().unwrap();
        if b.content.trim().is_empty() {
            core.remove(&b.id);
        } else {
            core.insert(b.id, b.content);
        }
        drop(core);
        self.fetch_core()
    }
    fn search_archival(&self, query: &str) -> Vec<MemoryBlock> {
        // TODO: Replace with semantic vector search using embeddings.
        let q = query.to_lowercase();
        self.archival
            .lock()
            .unwrap()
            .iter()
            .filter(|(id, c)| id.to_lowercase().contains(&q) || c.to_lowercase().contains(&q))
            .map(|(id, content)| MemoryBlock {
                id: id.clone(),
                content: content.clone(),
            })
            .collect()
    }
    fn update_archival(&self, b: MemoryBlock) {
        let mut arch = self.archival.lock().unwrap();
        if b.content.trim().is_empty() {
            arch.remove(&b.id);
        } else {
            arch.insert(b.id, b.content);
        }
    }
}

type Ctx = ();

struct CoreMemoryUpdate {
    store: Store,
}
impl AgentTool<Ctx> for CoreMemoryUpdate {
    fn name(&self) -> String {
        "core_memory_update".into()
    }
    fn description(&self) -> String {
        "Update or add a core memory block. Returns all core memories after the update.".into()
    }
    fn parameters(&self) -> JSONSchema {
        json!({
            "type": "object",
            "properties": {"id": {"type": "string"}, "content": {"type": "string"}},
            "required": ["id", "content"],
            "additionalProperties": false
        })
    }
    fn execute<'a>(
        &'a self,
        args: serde_json::Value,
        _context: &'a Ctx,
        _state: &'a llm_agent::RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>>> {
        Box::pin(async move {
            #[derive(Deserialize)]
            struct In {
                id: String,
                content: String,
            }
            let mut input: In = serde_json::from_value(args)?;
            println!(
                "[memory.core_memory_update] id={} len={}",
                input.id,
                input.content.len()
            );
            if input.id.trim().is_empty() {
                input.id = rand_id();
            }
            let updated = self.store.update_core(MemoryBlock {
                id: input.id,
                content: input.content,
            });
            let body = json!({"core_memories": updated}).to_string();
            Ok(AgentToolResult {
                content: vec![Part::text(body)],
                is_error: false,
            })
        })
    }
}

struct ArchivalSearch {
    store: Store,
}
impl AgentTool<Ctx> for ArchivalSearch {
    fn name(&self) -> String {
        "archival_memory_search".into()
    }
    fn description(&self) -> String {
        "Search for memories in the archival memory".into()
    }
    fn parameters(&self) -> JSONSchema {
        json!({
            "type": "object",
            "properties": {"query": {"type": "string"}},
            "required": ["query"],
            "additionalProperties": false
        })
    }
    fn execute<'a>(
        &'a self,
        args: serde_json::Value,
        _context: &'a Ctx,
        _state: &'a llm_agent::RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>>> {
        Box::pin(async move {
            #[derive(Deserialize)]
            struct In {
                query: String,
            }
            let input: In = serde_json::from_value(args)?;
            println!("[memory.archival_memory_search] query=\"{}\"", input.query);
            // TODO: Replace with semantic vector search using embeddings
            let results = self.store.search_archival(&input.query);
            let body = json!({"results": results}).to_string();
            Ok(AgentToolResult {
                content: vec![Part::text(body)],
                is_error: false,
            })
        })
    }
}

struct ArchivalUpdate {
    store: Store,
}
impl AgentTool<Ctx> for ArchivalUpdate {
    fn name(&self) -> String {
        "archival_memory_update".into()
    }
    fn description(&self) -> String {
        "Update or add a memory block in the archival memory".into()
    }
    fn parameters(&self) -> JSONSchema {
        json!({
            "type": "object",
            "properties": {"id": {"type": "string"}, "content": {"type": "string"}},
            "required": ["id", "content"],
            "additionalProperties": false
        })
    }
    fn execute<'a>(
        &'a self,
        args: serde_json::Value,
        _context: &'a Ctx,
        _state: &'a llm_agent::RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>>> {
        Box::pin(async move {
            #[derive(Deserialize)]
            struct In {
                id: String,
                content: String,
            }
            let mut input: In = serde_json::from_value(args)?;
            println!(
                "[memory.archival_memory_update] id={} len={}",
                input.id,
                input.content.len()
            );
            if input.id.trim().is_empty() {
                input.id = rand_id();
            }
            self.store.update_archival(MemoryBlock {
                id: input.id.clone(),
                content: input.content.clone(),
            });
            let resp = if input.content.trim().is_empty() {
                json!({"success": true, "action": "deleted"})
            } else {
                json!({"success": true, "action": "updated", "memory": {"id": input.id, "content": input.content}})
            };
            Ok(AgentToolResult {
                content: vec![Part::text(resp.to_string())],
                is_error: false,
            })
        })
    }
}

fn rand_id() -> String {
    format!("{:x}", std::process::id())
}

#[tokio::main]
async fn main() {
    dotenv().ok();

    // Use OpenAI gpt-4o via env var OPENAI_API_KEY
    let model = Arc::new(llm_sdk::openai::OpenAIModel::new(
        "gpt-4o",
        llm_sdk::openai::OpenAIModelOptions {
            api_key: std::env::var("OPENAI_API_KEY").expect("OPENAI_API_KEY must be set"),
            ..Default::default()
        },
    ));

    let store = Store::default();

    let memory_prompt = r"You can remember information learned from interactions with the user in two types of memory called core memory and archival memory.
Core memory is always available in your conversation context, providing essential, foundational context for keeping track of key details about the user.
As core memory is limited in size, it is important to only store the most important information. For other less important details, use archival memory.
Archival memory is infinite size, but is held outside of your immediate context, so you must explicitly run a search operation to see data inside it.
Archival memory is used to remember less significant details about the user or information found during the conversation. When the user mentions a name, topic, or details you don't know, search your archival memory to see if you have any information about it.";

    let rules_prompt = r"You cannot see prior conversation turns beyond what is provided in the current input.
When a user shares a durable preference or profile detail, call core_memory_update to store it.
When asked to recall such facts and it's not present in the current input, rely on the core memories in this prompt.
For less important or long-tail info, use archival_memory_search before answering.";

    let agent = Agent::new(
        llm_agent::AgentParams::new("memory", model.clone())
            .add_instruction(memory_prompt)
            .add_instruction(rules_prompt)
            .add_instruction(InstructionParam::AsyncFunc(Box::new({
                let store = store.clone();
                move |()| {
                    let store = store.clone();
                    Box::pin(async move {
                        let blocks = store.fetch_core();
                        Ok(format!(
                            "Core memories (JSON list):\n{}",
                            serde_json::to_string(&blocks).unwrap()
                        ))
                    })
                }
            })))
            .add_tool(CoreMemoryUpdate {
                store: store.clone(),
            })
            .add_tool(ArchivalSearch {
                store: store.clone(),
            })
            .add_tool(ArchivalUpdate {
                store: store.clone(),
            }),
    );

    // Four independent sessions (agent cannot see prior turns except via memory)
    // Turn 1 — store a core memory
    let items1: Vec<AgentItem> = vec![AgentItem::Message(Message::user(vec![Part::text(
        "Remember that my favorite color is blue.",
    )]))];
    println!("[user] Remember that my favorite color is blue.");
    let res1 = agent
        .run(AgentRequest {
            context: (),
            input: items1,
        })
        .await
        .expect("run failed");
    println!("res1: {:#?}", res1.content);

    // Turn 2 — recall using core memory (no prior messages)
    let items2: Vec<AgentItem> = vec![AgentItem::Message(Message::user(vec![Part::text(
        "What's my favorite color?",
    )]))];
    println!("[user] What's my favorite color?");
    let res2 = agent
        .run(AgentRequest {
            context: (),
            input: items2,
        })
        .await
        .expect("run failed");
    println!("res2: {:#?}", res2.content);

    // Turn 3 — capture background notes for later lookup
    let turn3 = "I captured some background notes titled 'q3-report-research' for future \
                 reference: "
        .to_string()
        + "Key data sources for the Q3 report include Salesforce pipeline exports, Google \
           Analytics weekly sessions, and the paid ads spend spreadsheet. "
        + "Please tuck this away so you can look it up later.";
    let items3: Vec<AgentItem> = vec![AgentItem::Message(Message::user(vec![Part::text(&turn3)]))];
    println!("[user] {turn3}");
    let res3 = agent
        .run(AgentRequest {
            context: (),
            input: items3,
        })
        .await
        .expect("run failed");
    println!("res3: {:#?}", res3.content);

    // Turn 4 — fetch the saved background notes
    let turn4 = "Can you pull up what we have under 'q3-report-research'?";
    let items4: Vec<AgentItem> = vec![AgentItem::Message(Message::user(vec![Part::text(turn4)]))];
    println!("[user] {turn4}");
    let res4 = agent
        .run(AgentRequest {
            context: (),
            input: items4,
        })
        .await
        .expect("run failed");
    println!("res4: {:#?}", res4.content);
}
