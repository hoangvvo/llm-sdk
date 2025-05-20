use std::{
    collections::HashMap,
    sync::{Arc, Mutex},
};

use dotenvy::dotenv;
use llm_agent::{Agent, AgentItem, AgentRequest, AgentTool, AgentToolResult};
use llm_sdk::{JSONSchema, Message, Part};
use serde::{Deserialize, Serialize};
use serde_json::json;

#[derive(Clone)]
enum ArtifactKind {
    Markdown,
    Text,
    Code,
}

#[derive(Clone, Serialize, Debug)]
struct Artifact {
    id: String,
    title: String,
    kind: String,
    content: String,
    version: u32,
    updated_at: String,
}

#[derive(Default, Clone)]
struct Store {
    m: Arc<Mutex<HashMap<String, Artifact>>>,
}

impl Store {
    fn create(&self, title: String, kind: String, content: String) -> Artifact {
        let id = format!(
            "{:x}",
            chrono::Utc::now().timestamp_nanos_opt().unwrap_or(0)
        );
        let a = Artifact {
            id: id.clone(),
            title,
            kind,
            content,
            version: 1,
            updated_at: chrono::Utc::now().to_rfc3339(),
        };
        self.m.lock().unwrap().insert(id.clone(), a.clone());
        a
    }
    fn update(&self, id: &str, content: String) -> (Artifact, String) {
        let mut map = self.m.lock().unwrap();
        let a = map.get_mut(id).expect("artifact not found");
        let before = a.content.clone();
        a.content = content;
        a.version += 1;
        a.updated_at = chrono::Utc::now().to_rfc3339();
        (a.clone(), before)
    }
    fn get(&self, id: &str) -> Artifact {
        self.m
            .lock()
            .unwrap()
            .get(id)
            .expect("artifact not found")
            .clone()
    }
    fn list(&self) -> Vec<Artifact> {
        self.m.lock().unwrap().values().cloned().collect()
    }
    fn delete(&self, id: &str) -> bool {
        self.m.lock().unwrap().remove(id).is_some()
    }
}

// Minimal colored line diff using similar
fn render_diff(old_text: &str, new_text: &str) -> String {
    use similar::{ChangeTag, TextDiff};
    let diff = TextDiff::from_lines(old_text, new_text);
    let mut out = String::new();
    for change in diff.iter_all_changes() {
        let (sign, color) = match change.tag() {
            ChangeTag::Delete => ("- ", "\x1b[31m"),
            ChangeTag::Insert => ("+ ", "\x1b[32m"),
            ChangeTag::Equal => ("  ", "\x1b[2m"),
        };
        out.push_str(color);
        out.push_str(sign);
        out.push_str(change.to_string().as_str());
        out.push_str("\x1b[0m");
    }
    out
}

// No context
type Ctx = ();

struct ArtifactCreate {
    store: Store,
}
#[async_trait::async_trait]
impl AgentTool<Ctx> for ArtifactCreate {
    fn name(&self) -> String {
        "artifact_create".into()
    }
    fn description(&self) -> String {
        "Create a new document and return it".into()
    }
    fn parameters(&self) -> JSONSchema {
        json!({
            "type":"object",
            "properties":{
                "title":{"type":"string"},
                "kind":{"type":"string", "enum":["markdown","text","code"]},
                "content":{"type":"string"}
            },
            "required":["title","kind","content"],
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
            title: String,
            kind: String,
            content: String,
        }
        let p: In = serde_json::from_value(args)?;
        println!("[artifacts.create] title={} kind={}", p.title, p.kind);
        let a = self.store.create(p.title, p.kind, p.content);
        Ok(AgentToolResult {
            content: vec![Part::text(json!({"artifact":a}).to_string())],
            is_error: false,
        })
    }
}

struct ArtifactUpdate {
    store: Store,
}
#[async_trait::async_trait]
impl AgentTool<Ctx> for ArtifactUpdate {
    fn name(&self) -> String {
        "artifact_update".into()
    }
    fn description(&self) -> String {
        "Replace the content of a document and return it".into()
    }
    fn parameters(&self) -> JSONSchema {
        json!({
            "type":"object",
            "properties":{ "id":{"type":"string"}, "content":{"type":"string"} },
            "required":["id","content"],
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
            id: String,
            content: String,
        }
        let p: In = serde_json::from_value(args)?;
        let before = self.store.get(&p.id).content;
        println!("[artifacts.update] id={} len={}", p.id, p.content.len());
        let (a, _before) = self.store.update(&p.id, p.content);
        println!(
            "\n=== Diff (old → new) ===\n{}========================\n",
            render_diff(&before, &a.content)
        );
        Ok(AgentToolResult {
            content: vec![Part::text(json!({"artifact":a}).to_string())],
            is_error: false,
        })
    }
}

struct ArtifactGet {
    store: Store,
}
#[async_trait::async_trait]
impl AgentTool<Ctx> for ArtifactGet {
    fn name(&self) -> String {
        "artifact_get".into()
    }
    fn description(&self) -> String {
        "Fetch a document by id".into()
    }
    fn parameters(&self) -> JSONSchema {
        json!({"type":"object","properties":{"id":{"type":"string"}},"required":["id"],"additionalProperties":false})
    }
    async fn execute(
        &self,
        args: serde_json::Value,
        _ctx: &Ctx,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>> {
        #[derive(Deserialize)]
        struct In {
            id: String,
        }
        let p: In = serde_json::from_value(args)?;
        println!("[artifacts.get] id={}", p.id);
        let a = self.store.get(&p.id);
        Ok(AgentToolResult {
            content: vec![Part::text(json!({"artifact":a}).to_string())],
            is_error: false,
        })
    }
}

struct ArtifactList {
    store: Store,
}
#[async_trait::async_trait]
impl AgentTool<Ctx> for ArtifactList {
    fn name(&self) -> String {
        "artifact_list".into()
    }
    fn description(&self) -> String {
        "List all documents".into()
    }
    fn parameters(&self) -> JSONSchema {
        json!({"type":"object","properties":{},"additionalProperties":false})
    }
    async fn execute(
        &self,
        _args: serde_json::Value,
        _ctx: &Ctx,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>> {
        println!("[artifacts.list]");
        let list = self.store.list();
        Ok(AgentToolResult {
            content: vec![Part::text(json!({"artifacts":list}).to_string())],
            is_error: false,
        })
    }
}

struct ArtifactDelete {
    store: Store,
}
#[async_trait::async_trait]
impl AgentTool<Ctx> for ArtifactDelete {
    fn name(&self) -> String {
        "artifact_delete".into()
    }
    fn description(&self) -> String {
        "Delete a document by id".into()
    }
    fn parameters(&self) -> JSONSchema {
        json!({"type":"object","properties":{"id":{"type":"string"}},"required":["id"],"additionalProperties":false})
    }
    async fn execute(
        &self,
        args: serde_json::Value,
        _ctx: &Ctx,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>> {
        #[derive(Deserialize)]
        struct In {
            id: String,
        }
        let p: In = serde_json::from_value(args)?;
        println!("[artifacts.delete] id={}", p.id);
        let ok = self.store.delete(&p.id);
        Ok(AgentToolResult {
            content: vec![Part::text(json!({"success":ok}).to_string())],
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
    let overview = "Use documents (artifacts/canvases) for substantive deliverables like \
                    documents, plans, specs, or code. Keep chat replies brief and \
                    status-oriented; put the full content into a document via the tools. Always \
                    reference documents by id.";
    let rules = "- Prefer creating/updating documents instead of pasting large content into \
                 chat\n- When asked to revise or extend prior work, read/update the relevant \
                 document\n- Keep the chat response short: what changed, where it lives (document \
                 id), and next steps\n";

    let agent = Agent::new(
        llm_agent::AgentParams::new("artifacts", model)
            .add_instruction(overview)
            .add_instruction(rules)
            .add_tool(ArtifactCreate {
                store: store.clone(),
            })
            .add_tool(ArtifactUpdate {
                store: store.clone(),
            })
            .add_tool(ArtifactGet {
                store: store.clone(),
            })
            .add_tool(ArtifactList {
                store: store.clone(),
            })
            .add_tool(ArtifactDelete {
                store: store.clone(),
            }),
    );

    let items1: Vec<AgentItem> = vec![AgentItem::Message(Message::user(vec![Part::text(
        "We need a product requirements document for a new Todo app. Please draft it in markdown \
         with sections: Overview, Goals, Non-Goals, Requirements. Keep your chat reply short and \
         save the full document to a separate document we can keep iterating on.",
    )]))];
    let res1 = agent
        .run(AgentRequest {
            context: (),
            input: items1,
        })
        .await
        .expect("run failed");
    println!("{:?}", res1.content);
    println!("Documents after creation:\n{:?}", store.list());

    let items2: Vec<AgentItem> = vec![AgentItem::Message(Message::user(vec![Part::text(
        "Please revise the document: expand the Goals section with 3 concrete goals and add a \
         Milestones section. Keep your chat reply brief.",
    )]))];
    let res2 = agent
        .run(AgentRequest {
            context: (),
            input: items2,
        })
        .await
        .expect("run failed");
    println!("{:?}", res2.content);
    println!("Documents after update:\n{:?}", store.list());
}
