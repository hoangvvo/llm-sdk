use futures::future::BoxFuture;
use llm_agent::{
    Agent, AgentFunctionTool, AgentItem, AgentRequest, AgentToolResult, BoxedError,
    InstructionParam, RunState,
};
use llm_sdk::{
    llm_sdk_test::MockLanguageModel, AudioFormat, AudioOptions, FunctionTool, JSONSchema, Message,
    Modality, ModelResponse, Part, ReasoningOptions, ResponseFormatJson, ResponseFormatOption,
    Tool, WebSearchTool,
};
use serde_json::{json, Value};
use std::sync::Arc;

struct LookupTool;

impl AgentFunctionTool<TestContext> for LookupTool {
    fn name(&self) -> String {
        "lookup".to_string()
    }

    fn description(&self) -> String {
        "Look up a record".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        json!({
            "type": "object",
            "properties": {"query": {"type": "string"}},
            "required": ["query"],
            "additionalProperties": false,
        })
    }

    fn execute<'a>(
        &'a self,
        _args: Value,
        _context: &'a TestContext,
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

#[derive(Clone)]
struct TestContext {
    tenant: String,
}

#[tokio::test]
async fn agent_builder_forwards_complete_public_configuration() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("configured")],
        ..Default::default()
    });
    let response_format = ResponseFormatOption::Json(ResponseFormatJson {
        name: "answer".to_string(),
        description: Some("A configured answer".to_string()),
        schema: Some(json!({
            "type": "object",
            "properties": {"answer": {"type": "string"}},
            "required": ["answer"],
            "additionalProperties": false,
        })),
    });
    let audio = AudioOptions {
        format: Some(AudioFormat::Mp3),
        voice: Some("alloy".to_string()),
        language: Some("en".to_string()),
    };
    let reasoning = ReasoningOptions {
        enabled: true,
        budget_tokens: Some(256),
    };
    let web_search = WebSearchTool {
        allowed_domains: Some(vec!["example.com".to_string()]),
        ..Default::default()
    };
    let agent = Agent::builder("configured-agent", model.clone())
        .instructions(vec![
            InstructionParam::String("Static".to_string()),
            InstructionParam::Func(Box::new(|context: &TestContext| {
                Ok(format!("Tenant: {}", context.tenant))
            })),
        ])
        .add_tool(LookupTool)
        .add_tool(web_search.clone())
        .response_format(response_format.clone())
        .max_turns(3)
        .temperature(0.2)
        .top_p(0.8)
        .top_k(12)
        .presence_penalty(0.1)
        .frequency_penalty(0.3)
        .modalities(vec![Modality::Text, Modality::Audio])
        .audio(audio.clone())
        .reasoning(reasoning.clone())
        .build();

    agent
        .run(AgentRequest {
            context: TestContext {
                tenant: "acme".to_string(),
            },
            input: vec![AgentItem::Message(Message::user(vec![Part::text(
                "Configure this",
            )]))],
        })
        .await
        .expect("agent run should succeed");

    let inputs = model.tracked_generate_inputs();
    assert_eq!(inputs.len(), 1);
    let input = &inputs[0];
    assert_eq!(input.system_prompt.as_deref(), Some("Static\nTenant: acme"));
    assert_eq!(
        input.messages,
        vec![Message::user(vec![Part::text("Configure this")])]
    );
    assert_eq!(
        input.tools,
        Some(vec![
            Tool::Function(FunctionTool::new(
                "lookup",
                "Look up a record",
                LookupTool.parameters(),
            )),
            Tool::WebSearch(web_search),
        ])
    );
    assert_eq!(input.response_format, Some(response_format));
    assert_eq!(input.temperature, Some(0.2));
    assert_eq!(input.top_p, Some(0.8));
    assert_eq!(input.top_k, Some(12));
    assert_eq!(input.presence_penalty, Some(0.1));
    assert_eq!(input.frequency_penalty, Some(0.3));
    assert_eq!(
        input.modalities,
        Some(vec![Modality::Text, Modality::Audio])
    );
    assert_eq!(input.audio, Some(audio));
    assert_eq!(input.reasoning, Some(reasoning));
}
