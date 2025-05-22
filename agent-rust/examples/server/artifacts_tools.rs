use async_trait::async_trait;
use chrono::Utc;
use llm_agent::{AgentTool, AgentToolResult};
use llm_sdk::{JSONSchema, Part};
use rand::RngCore;
use serde::Deserialize;
use serde_json::Value;

use crate::context::{Artifact, ArtifactKind, MyContext};

fn rand_id(n_bytes: usize) -> String {
    let mut b = vec![0u8; n_bytes];
    rand::thread_rng().fill_bytes(&mut b);
    // hex encode
    b.iter().map(|x| format!("{:02x}", x)).collect::<String>()
}

fn find_artifact<'a>(ctx: &'a MyContext, id: &str) -> Option<&'a Artifact> {
    ctx.artifacts
        .as_ref()
        .and_then(|list| list.iter().find(|a| a.id == id))
}

// artifact_create
#[derive(Deserialize)]
struct ArtifactCreateParams {
    title: String,
    kind: ArtifactKind,
    content: String,
}

pub struct ArtifactCreateTool;

#[async_trait]
impl AgentTool<MyContext> for ArtifactCreateTool {
    fn name(&self) -> String {
        "artifact_create".to_string()
    }

    fn description(&self) -> String {
        "Create a new document and return an instruction for the client to persist it".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "title": {"type": "string"},
                "kind": {"type": "string", "enum": ["markdown", "text", "code"]},
                "content": {"type": "string"}
            },
            "required": ["title", "kind", "content"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: Value,
        _context: &MyContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>> {
        let params: ArtifactCreateParams = serde_json::from_value(args)?;
        let now = Utc::now().to_rfc3339();
        let id = rand_id(5);
        let artifact = Artifact {
            id,
            title: params.title,
            kind: params.kind,
            content: params.content,
            version: Some(1),
            updated_at: Some(now),
        };
        let payload = serde_json::json!({"op": "artifact_create", "artifact": artifact});
        Ok(AgentToolResult {
            content: vec![Part::text(payload.to_string())],
            is_error: false,
        })
    }
}

// artifact_update
#[derive(Deserialize)]
struct ArtifactUpdateParams {
    id: String,
    content: String,
}

pub struct ArtifactUpdateTool;

#[async_trait]
impl AgentTool<MyContext> for ArtifactUpdateTool {
    fn name(&self) -> String {
        "artifact_update".to_string()
    }
    fn description(&self) -> String {
        "Replace document content and return an instruction for the client to persist changes"
            .to_string()
    }
    fn parameters(&self) -> JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {"id": {"type": "string"}, "content": {"type": "string"}},
            "required": ["id", "content"],
            "additionalProperties": false
        })
    }
    async fn execute(
        &self,
        args: Value,
        context: &MyContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>> {
        let params: ArtifactUpdateParams = serde_json::from_value(args)?;
        let prev = find_artifact(context, &params.id);
        let now = Utc::now().to_rfc3339();
        let next_version = prev.and_then(|a| a.version).unwrap_or(0) + 1;
        let title = prev
            .map(|a| a.title.clone())
            .unwrap_or_else(|| "Untitled".to_string());
        let kind = prev
            .map(|a| a.kind.clone())
            .unwrap_or(ArtifactKind::Markdown);
        let prev_content = prev.map(|a| a.content.clone()).unwrap_or_default();
        let artifact = Artifact {
            id: params.id.clone(),
            title,
            kind,
            content: params.content,
            version: Some(next_version),
            updated_at: Some(now),
        };
        let payload = serde_json::json!({"op": "artifact_update", "id": params.id, "prev_content": prev_content, "artifact": artifact});
        Ok(AgentToolResult {
            content: vec![Part::text(payload.to_string())],
            is_error: false,
        })
    }
}

// artifact_get
#[derive(Deserialize)]
struct ArtifactGetParams {
    id: String,
}
pub struct ArtifactGetTool;

#[async_trait]
impl AgentTool<MyContext> for ArtifactGetTool {
    fn name(&self) -> String {
        "artifact_get".to_string()
    }
    fn description(&self) -> String {
        "Fetch a document from the current client context".to_string()
    }
    fn parameters(&self) -> JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {"id": {"type": "string"}},
            "required": ["id"],
            "additionalProperties": false
        })
    }
    async fn execute(
        &self,
        args: Value,
        context: &MyContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>> {
        let params: ArtifactGetParams = serde_json::from_value(args)?;
        let artifact = find_artifact(context, &params.id);
        let payload =
            serde_json::json!({"op": "artifact_get", "id": params.id, "artifact": artifact});
        Ok(AgentToolResult {
            content: vec![Part::text(payload.to_string())],
            is_error: false,
        })
    }
}

// artifact_list
pub struct ArtifactListTool;

#[async_trait]
impl AgentTool<MyContext> for ArtifactListTool {
    fn name(&self) -> String {
        "artifact_list".to_string()
    }
    fn description(&self) -> String {
        "List documents from the current client context".to_string()
    }
    fn parameters(&self) -> JSONSchema {
        serde_json::json!({"type": "object", "properties": {}, "additionalProperties": false})
    }
    async fn execute(
        &self,
        _args: Value,
        context: &MyContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>> {
        let payload = serde_json::json!({"op": "artifact_list", "artifacts": context.artifacts});
        Ok(AgentToolResult {
            content: vec![Part::text(payload.to_string())],
            is_error: false,
        })
    }
}

// artifact_delete
#[derive(Deserialize)]
struct ArtifactDeleteParams {
    id: String,
}
pub struct ArtifactDeleteTool;

#[async_trait]
impl AgentTool<MyContext> for ArtifactDeleteTool {
    fn name(&self) -> String {
        "artifact_delete".to_string()
    }
    fn description(&self) -> String {
        "Delete a document by id (client will persist)".to_string()
    }
    fn parameters(&self) -> JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {"id": {"type": "string"}},
            "required": ["id"],
            "additionalProperties": false
        })
    }
    async fn execute(
        &self,
        args: Value,
        _context: &MyContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn std::error::Error + Send + Sync>> {
        let params: ArtifactDeleteParams = serde_json::from_value(args)?;
        let payload = serde_json::json!({"op": "artifact_delete", "id": params.id});
        Ok(AgentToolResult {
            content: vec![Part::text(payload.to_string())],
            is_error: false,
        })
    }
}
