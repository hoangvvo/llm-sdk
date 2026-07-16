use futures::future::BoxFuture;
use llm_agent::{AgentFunctionTool, AgentTool, AgentToolResult, BoxedError, RunState};
use llm_sdk::{FunctionTool, JSONSchema, Tool, WebSearchTool};
use serde_json::{json, Value};

struct LookupTool;

impl AgentFunctionTool<()> for LookupTool {
    fn name(&self) -> String {
        "lookup".to_string()
    }

    fn description(&self) -> String {
        "Look up a record".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        json!({"type": "object", "properties": {}})
    }

    fn execute<'a>(
        &'a self,
        _args: Value,
        _context: &'a (),
        _state: &'a RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, BoxedError>> {
        Box::pin(async {
            Ok(AgentToolResult {
                content: vec![],
                is_error: false,
            })
        })
    }
}

#[test]
fn converts_public_tool_variants_for_model_use() {
    let function = AgentTool::function(LookupTool);
    assert_eq!(function.name(), "lookup");
    assert_eq!(
        Tool::from(&function),
        Tool::Function(FunctionTool::new(
            "lookup",
            "Look up a record",
            json!({"type": "object", "properties": {}}),
        ))
    );

    let web_search = WebSearchTool {
        allowed_domains: Some(vec!["example.com".to_string()]),
        ..Default::default()
    };
    let hosted = AgentTool::<()>::web_search(web_search.clone());
    assert_eq!(hosted.name(), "web_search");
    assert_eq!(Tool::from(&hosted), Tool::WebSearch(web_search));
}
