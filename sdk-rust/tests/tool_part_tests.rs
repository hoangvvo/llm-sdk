use llm_sdk::{
    Part, ToolCall, ToolCallPart, ToolResult, ToolResultPart, WebSearchAction, WebSearchToolCall,
    WebSearchToolCallStatus, WebSearchToolResult,
};
use serde_json::json;

#[test]
fn web_search_tool_parts_use_nested_discriminators() {
    let part = Part::ToolCall(ToolCallPart {
        tool_call_id: "ws_1".to_string(),
        call: ToolCall::WebSearch(WebSearchToolCall {
            action: Some(WebSearchAction::Search {
                queries: vec!["sdk docs".to_string()],
            }),
            status: Some(WebSearchToolCallStatus::Completed),
        }),
        signature: None,
        id: None,
    });
    let value = serde_json::to_value(&part).unwrap();
    assert_eq!(value["type"], json!("tool-call"));
    assert_eq!(value["call"]["type"], json!("web_search"));
    let decoded: Part = serde_json::from_value(value).unwrap();
    assert!(matches!(
        decoded,
        Part::ToolCall(ToolCallPart {
            call: ToolCall::WebSearch(_),
            ..
        })
    ));

    let result = Part::ToolResult(ToolResultPart {
        tool_call_id: "ws_1".to_string(),
        result: ToolResult::WebSearch(WebSearchToolResult {
            sources: vec![],
            error_code: Some("unavailable".to_string()),
        }),
        status: llm_sdk::ToolResultStatus::Failed,
    });
    let value = serde_json::to_value(&result).unwrap();
    assert_eq!(value["result"]["type"], json!("web_search"));
    assert_eq!(value["result"]["error_code"], json!("unavailable"));
    let decoded: Part = serde_json::from_value(value).unwrap();
    assert!(matches!(
        decoded,
        Part::ToolResult(ToolResultPart {
            result: ToolResult::WebSearch(WebSearchToolResult {
                error_code: Some(code),
                ..
            }),
            ..
        }) if code == "unavailable"
    ));
}
