use llm_agent::{
    mcp::{MCPParams, MCPStdioParams, MCPStreamableHTTPParams},
    AgentItem, AgentItemTool, AgentResponse, AgentResponseStatus, AgentStreamEvent,
    AgentStreamItemEvent,
};
use llm_sdk::{
    ContentDelta, Message, ModelResponse, Part, PartDelta, PartialModelResponse, TextPartDelta,
    ToolResultStatus,
};
use serde_json::json;

#[test]
fn agent_response_text_returns_only_non_empty_text() {
    let response = AgentResponse {
        status: AgentResponseStatus::Completed,
        output: Vec::new(),
        content: vec![
            Part::text("Hello"),
            Part::image("AAEC", "image/png"),
            Part::text(""),
            Part::text("world"),
        ],
    };

    assert_eq!(response.text(), "Hello world");
}

#[test]
fn agent_items_follow_the_public_json_contract_and_round_trip() {
    let cases = vec![
        (
            AgentItem::Message(Message::user(vec![Part::text("Hello")])),
            json!({
                "type": "message",
                "role": "user",
                "content": [{"type": "text", "text": "Hello"}],
            }),
        ),
        (
            AgentItem::Model(ModelResponse {
                content: vec![Part::text("Hi")],
                ..Default::default()
            }),
            json!({
                "type": "model",
                "content": [{"type": "text", "text": "Hi"}],
            }),
        ),
        (
            AgentItem::Tool(AgentItemTool {
                tool_call_id: "call_1".to_string(),
                tool_name: "lookup".to_string(),
                input: json!({"id": 42}),
                output: vec![Part::text("found")],
                status: ToolResultStatus::Completed,
            }),
            json!({
                "type": "tool",
                "tool_call_id": "call_1",
                "tool_name": "lookup",
                "input": {"id": 42},
                "output": [{"type": "text", "text": "found"}],
                "status": "completed",
            }),
        ),
    ];

    for (item, expected) in cases {
        let encoded = serde_json::to_value(&item).expect("serialize item");
        assert_eq!(encoded, expected);
        let decoded: AgentItem = serde_json::from_value(encoded).expect("deserialize item");
        assert_eq!(decoded, item);
    }
}

#[test]
fn agent_stream_events_follow_the_public_json_contract_and_round_trip() {
    let cases = vec![
        (
            AgentStreamEvent::Partial(PartialModelResponse {
                delta: Some(ContentDelta {
                    index: 0,
                    part: PartDelta::Text(TextPartDelta::new("Hi")),
                }),
                ..Default::default()
            }),
            json!({
                "event": "partial",
                "delta": {"index": 0, "part": {"type": "text", "text": "Hi"}},
            }),
        ),
        (
            AgentStreamEvent::Item(AgentStreamItemEvent {
                index: 2,
                item: AgentItem::Model(ModelResponse {
                    content: vec![Part::text("Hi")],
                    ..Default::default()
                }),
            }),
            json!({
                "event": "item",
                "index": 2,
                "item": {
                    "type": "model",
                    "content": [{"type": "text", "text": "Hi"}],
                },
            }),
        ),
        (
            AgentStreamEvent::Response(AgentResponse {
                status: AgentResponseStatus::Completed,
                output: Vec::new(),
                content: vec![Part::text("Done")],
            }),
            json!({
                "event": "response",
                "output": [],
                "content": [{"type": "text", "text": "Done"}],
                "status": "completed",
            }),
        ),
    ];

    for (event, expected) in cases {
        let encoded = serde_json::to_value(&event).expect("serialize event");
        assert_eq!(encoded, expected);
        let decoded: AgentStreamEvent = serde_json::from_value(encoded).expect("deserialize event");
        assert_eq!(decoded, event);
    }
}

#[test]
fn mcp_params_follow_the_public_json_contract_and_round_trip() {
    let cases = vec![
        (
            MCPParams::Stdio(MCPStdioParams {
                command: "uvx".to_string(),
                args: vec!["server.py".to_string()],
            }),
            json!({"type": "stdio", "command": "uvx", "args": ["server.py"]}),
        ),
        (
            MCPParams::StreamableHttp(MCPStreamableHTTPParams {
                url: "https://example.com/mcp".to_string(),
                authorization: Some("Bearer token".to_string()),
            }),
            json!({
                "type": "streamable-http",
                "url": "https://example.com/mcp",
                "authorization": "Bearer token",
            }),
        ),
    ];

    for (params, expected) in cases {
        let encoded = serde_json::to_value(&params).expect("serialize MCP params");
        assert_eq!(encoded, expected);
        let decoded: MCPParams = serde_json::from_value(encoded).expect("deserialize MCP params");
        let round_trip = serde_json::to_value(decoded).expect("serialize decoded MCP params");
        assert_eq!(round_trip, expected);
    }
}

#[test]
fn mcp_params_reject_unknown_variants() {
    let result = serde_json::from_value::<MCPParams>(json!({
        "type": "websocket",
        "url": "https://example.com",
    }));

    assert!(result.is_err());
}
