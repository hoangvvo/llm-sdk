use llm_agent::{
    mcp::{MCPInit, MCPParams, MCPStdioParams, MCPToolkit},
    Agent, AgentError, AgentParams, BoxedError,
};
use llm_sdk::llm_sdk_test::MockLanguageModel;
use std::sync::{Arc, Mutex};

#[derive(Clone, Debug, PartialEq)]
struct TestContext {
    command: String,
}

#[tokio::test]
async fn mcp_initialization_resolves_async_params_from_context() {
    let model = Arc::new(MockLanguageModel::new());
    let contexts = Arc::new(Mutex::new(Vec::new()));
    let init = MCPInit::from_async_fn({
        let contexts = contexts.clone();
        move |context: &TestContext| {
            let context = context.clone();
            contexts
                .lock()
                .expect("contexts lock")
                .push(context.clone());
            async move {
                Ok(MCPParams::Stdio(MCPStdioParams {
                    command: context.command,
                    args: vec![],
                }))
            }
        }
    });
    let agent =
        Agent::new(AgentParams::new("test_agent", model).add_toolkit(MCPToolkit::new(init)));

    let result = agent
        .create_session(TestContext {
            command: "__llm_sdk_missing_mcp_binary__".to_string(),
        })
        .await;

    assert!(matches!(result, Err(AgentError::Init(_))));
    assert_eq!(
        *contexts.lock().expect("contexts lock"),
        vec![TestContext {
            command: "__llm_sdk_missing_mcp_binary__".to_string(),
        }]
    );
}

#[tokio::test]
async fn mcp_initialization_wraps_resolver_errors() {
    let model = Arc::new(MockLanguageModel::new());
    let toolkit = MCPToolkit::new(MCPInit::from_fn(
        |(): &()| -> Result<MCPParams, BoxedError> {
            Err(std::io::Error::other("credential lookup failed").into())
        },
    ));
    let agent = Agent::new(AgentParams::new("test_agent", model).add_toolkit(toolkit));

    let result = agent.create_session(()).await;

    match result {
        Err(AgentError::Init(error)) => {
            assert!(error.to_string().contains("credential lookup failed"));
        }
        _ => panic!("expected agent initialization error"),
    }
}
