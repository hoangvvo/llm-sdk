use serde::{Deserialize, Deserializer};
use serde_json::Value;

use super::responses_api::*;

impl<'de> Deserialize<'de> for Item {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        let value = Value::deserialize(deserializer)?;
        let item_type = value.get("type").and_then(Value::as_str).map(str::to_owned);

        macro_rules! decode {
            ($variant:ident, $type:ty) => {{
                return Ok(serde_json::from_value::<$type>(value.clone())
                    .map(Item::$variant)
                    .unwrap_or_else(|_| Item::Unknown(value)));
            }};
        }

        match item_type.as_deref() {
            Some("code_interpreter_call") => {
                decode!(CodeInterpreterToolCall, CodeInterpreterToolCall)
            }
            Some("computer_call") => decode!(ComputerToolCall, ComputerToolCall),
            Some("local_shell_call") => decode!(LocalShellToolCall, LocalShellToolCall),
            Some("mcp_approval_request") => decode!(MCPApprovalRequest, MCPApprovalRequest),
            Some("mcp_call") => decode!(MCPToolCall, MCPToolCall),
            Some("file_search_call") => decode!(FileSearchToolCall, FileSearchToolCall),
            Some("web_search_call") => decode!(WebSearchToolCall, WebSearchToolCall),
            Some("function_call") => decode!(FunctionToolCall, FunctionToolCall),
            Some("image_generation_call") => decode!(ImageGenToolCall, ImageGenToolCall),
            Some("local_shell_call_output") => {
                decode!(LocalShellToolCallOutput, LocalShellToolCallOutput)
            }
            Some("apply_patch_call") => {
                decode!(ApplyPatchToolCallItemParam, ApplyPatchToolCallItemParam)
            }
            Some("mcp_list_tools") => decode!(MCPListTools, MCPListTools),
            Some("mcp_approval_response") => {
                decode!(MCPApprovalResponse, MCPApprovalResponse)
            }
            Some("custom_tool_call") => decode!(CustomToolCall, CustomToolCall),
            Some("computer_call_output") => {
                decode!(ComputerCallOutputItemParam, ComputerCallOutputItemParam)
            }
            Some("function_call_output") => {
                decode!(FunctionCallOutputItemParam, FunctionCallOutputItemParam)
            }
            Some("reasoning") => decode!(ReasoningItem, ReasoningItem),
            Some("shell_call") => {
                decode!(FunctionShellCallItemParam, FunctionShellCallItemParam)
            }
            Some("shell_call_output") => {
                decode!(
                    FunctionShellCallOutputItemParam,
                    FunctionShellCallOutputItemParam
                )
            }
            Some("apply_patch_call_output") => decode!(
                ApplyPatchToolCallOutputItemParam,
                ApplyPatchToolCallOutputItemParam
            ),
            Some("custom_tool_call_output") => {
                decode!(CustomToolCallOutput, CustomToolCallOutput)
            }
            Some("message")
                if value.get("role").and_then(Value::as_str) == Some("assistant")
                    && value.get("id").is_some()
                    && value.get("status").is_some() =>
            {
                decode!(OutputMessage, OutputMessage)
            }
            Some("message") => decode!(InputMessage, InputMessage),
            Some("tool_search_call") => {
                decode!(ToolSearchCallItemParam, ToolSearchCallItemParam)
            }
            Some("tool_search_output") => {
                decode!(ToolSearchOutputItemParam, ToolSearchOutputItemParam)
            }
            Some("compaction") => {
                decode!(CompactionSummaryItemParam, CompactionSummaryItemParam)
            }
            None if value.get("role").is_some() && value.get("content").is_some() => {
                decode!(InputMessage, InputMessage)
            }
            _ => Ok(Item::Unknown(value)),
        }
    }
}
