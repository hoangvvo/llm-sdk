use futures::future::BoxFuture;
use llm_agent::{
    Agent, AgentError, AgentItem, AgentParams, AgentRequest, AgentTool, BoxedError, RunSession,
    Toolkit, ToolkitSession,
};
use llm_sdk::{
    llm_sdk_test::{MockGenerateResult, MockLanguageModel},
    LanguageModelError, Message, Part,
};
use std::sync::{
    atomic::{AtomicUsize, Ordering},
    Arc,
};

struct LifecycleToolkit {
    create_error: Option<&'static str>,
    close_error: Option<&'static str>,
    close_calls: Arc<AtomicUsize>,
}

impl Toolkit<()> for LifecycleToolkit {
    fn create_session<'a>(
        &'a self,
        _context: &'a (),
    ) -> BoxFuture<'a, Result<Box<dyn ToolkitSession<()> + Send + Sync>, BoxedError>> {
        Box::pin(async move {
            if let Some(message) = self.create_error {
                return Err(std::io::Error::other(message).into());
            }

            Ok(Box::new(LifecycleToolkitSession {
                close_error: self.close_error,
                close_calls: self.close_calls.clone(),
            }) as Box<dyn ToolkitSession<()> + Send + Sync>)
        })
    }
}

struct LifecycleToolkitSession {
    close_error: Option<&'static str>,
    close_calls: Arc<AtomicUsize>,
}

impl ToolkitSession<()> for LifecycleToolkitSession {
    fn system_prompt(&self) -> Option<String> {
        None
    }

    fn tools(&self) -> Vec<AgentTool<()>> {
        Vec::new()
    }

    fn close(self: Box<Self>) -> BoxFuture<'static, Result<(), BoxedError>> {
        Box::pin(async move {
            self.close_calls.fetch_add(1, Ordering::SeqCst);
            if let Some(message) = self.close_error {
                return Err(std::io::Error::other(message).into());
            }
            Ok(())
        })
    }
}

fn toolkit(
    create_error: Option<&'static str>,
    close_error: Option<&'static str>,
    close_calls: &Arc<AtomicUsize>,
) -> LifecycleToolkit {
    LifecycleToolkit {
        create_error,
        close_error,
        close_calls: close_calls.clone(),
    }
}

#[tokio::test]
async fn run_session_closes_partial_initialization_on_toolkit_failure() {
    let initialized_close_calls = Arc::new(AtomicUsize::new(0));
    let failed_close_calls = Arc::new(AtomicUsize::new(0));
    let model = Arc::new(MockLanguageModel::new());
    let params = AgentParams::new("test-agent", model)
        .add_toolkit(toolkit(None, None, &initialized_close_calls))
        .add_toolkit(toolkit(
            Some("second toolkit failed"),
            None,
            &failed_close_calls,
        ));

    let result = RunSession::new(Arc::new(params), ()).await;

    assert!(matches!(result, Err(AgentError::Init(_))));
    assert_eq!(initialized_close_calls.load(Ordering::SeqCst), 1);
    assert_eq!(failed_close_calls.load(Ordering::SeqCst), 0);
}

#[tokio::test]
async fn run_session_close_attempts_every_toolkit_and_reports_failure() {
    let failing_close_calls = Arc::new(AtomicUsize::new(0));
    let successful_close_calls = Arc::new(AtomicUsize::new(0));
    let model = Arc::new(MockLanguageModel::new());
    let params = AgentParams::new("test-agent", model)
        .add_toolkit(toolkit(None, Some("cleanup failed"), &failing_close_calls))
        .add_toolkit(toolkit(None, None, &successful_close_calls));
    let session = RunSession::new(Arc::new(params), ())
        .await
        .expect("session initializes");

    let result = session.close().await;

    assert!(matches!(result, Err(AgentError::Cleanup(_))));
    assert_eq!(failing_close_calls.load(Ordering::SeqCst), 1);
    assert_eq!(successful_close_calls.load(Ordering::SeqCst), 1);
}

#[tokio::test]
async fn agent_run_preserves_model_error_when_cleanup_fails() {
    let close_calls = Arc::new(AtomicUsize::new(0));
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(MockGenerateResult::error(LanguageModelError::InvalidInput(
        "generation failed".to_string(),
    )));
    let agent = Agent::new(AgentParams::new("test-agent", model).add_toolkit(toolkit(
        None,
        Some("cleanup failed"),
        &close_calls,
    )));

    let result = agent
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::user(vec![Part::text("Hello")]))],
        })
        .await;

    assert!(matches!(result, Err(AgentError::LanguageModel(_))));
    assert_eq!(close_calls.load(Ordering::SeqCst), 1);
}
