use futures::TryStreamExt;
use llm_agent::{
    Agent, AgentItem, AgentParams, AgentRequest, AgentResponse, AgentStreamEvent,
    AgentStreamItemEvent,
};
use llm_sdk::{
    llm_sdk_test::{MockLanguageModel, MockStreamResult},
    ContentDelta, Message, ModelResponse, Part, PartDelta, PartialModelResponse, TextPartDelta,
    UserMessage,
};
use std::sync::Arc;

#[tokio::test]
async fn agent_run_creates_session_runs_and_finishes() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("Mock response")],
        ..Default::default()
    });

    let agent = Agent::new(AgentParams::new("test-agent", model.clone()));

    let response = agent
        .run(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Hello")],
            }))],
        })
        .await
        .expect("agent run succeeds");

    let expected = AgentResponse {
        content: vec![Part::text("Mock response")],
        output: vec![AgentItem::Model(ModelResponse {
            content: vec![Part::text("Mock response")],
            ..Default::default()
        })],
    };

    assert_eq!(response, expected);
}

#[tokio::test]
async fn agent_run_stream_creates_session_streams_and_finishes() {
    let model = Arc::new(MockLanguageModel::new());
    model.enqueue_stream(MockStreamResult::partials(vec![PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::Text(TextPartDelta {
                text: "Mock".to_string(),
                citation: None,
            }),
        }),
        ..Default::default()
    }]));

    let agent = Agent::new(AgentParams::new("test-agent", model.clone()));

    let stream = agent
        .run_stream(AgentRequest {
            context: (),
            input: vec![AgentItem::Message(Message::User(UserMessage {
                content: vec![Part::text("Hello")],
            }))],
        })
        .await
        .expect("agent run_stream succeeds");

    let events = stream
        .map_err(|err| err.to_string())
        .try_collect::<Vec<_>>()
        .await
        .expect("collect stream");

    let expected = vec![
        AgentStreamEvent::Partial(PartialModelResponse {
            delta: Some(ContentDelta {
                index: 0,
                part: PartDelta::Text(TextPartDelta {
                    text: "Mock".to_string(),
                    citation: None,
                }),
            }),
            ..Default::default()
        }),
        AgentStreamEvent::Item(AgentStreamItemEvent {
            index: 0,
            item: AgentItem::Model(ModelResponse {
                content: vec![Part::text("Mock")],
                ..Default::default()
            }),
        }),
        AgentStreamEvent::Response(AgentResponse {
            content: vec![Part::text("Mock")],
            output: vec![AgentItem::Model(ModelResponse {
                content: vec![Part::text("Mock")],
                ..Default::default()
            })],
        }),
    ];

    assert_eq!(events, expected);
}
