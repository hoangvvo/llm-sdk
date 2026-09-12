use dotenvy::dotenv;
use llm_sdk::{
    FunctionTool, LanguageModelInput, Message, Part, Tool, ToolCall, ToolResult, ToolResultPart,
    ToolSearchTool,
};
use serde::Deserialize;
use serde_json::{json, Value};

mod common;

#[derive(Deserialize)]
struct LookupHolidayArgs {
    country: String,
    year: u32,
}

fn lookup_holiday(args: &LookupHolidayArgs) -> Value {
    println!("[TOOLS lookup_holiday()] {} {}", args.country, args.year);

    // Sample data for this example. Replace with a holiday service in an
    // application.
    assert!(
        args.country.eq_ignore_ascii_case("VN") && args.year == 2026,
        "Sample data is only available for VN in 2026"
    );
    json!({ "holidays": [{ "date": "2026-09-02", "name": "National Day" }] })
}

#[tokio::main]
async fn main() {
    dotenv().ok();

    let provider = std::env::var("PROVIDER").unwrap_or_else(|_| "openai".to_string());
    let model_id = std::env::var("MODEL").unwrap_or_else(|_| "gpt-5.6-sol".to_string());
    let model = common::get_model(&provider, &model_id);

    let tools: Vec<Tool> = vec![
        ToolSearchTool::new().into(),
        FunctionTool::new(
            "lookup_holiday",
            "Look up the public holidays of a country in a given year",
            json!({
                "type": "object",
                "properties": {
                    "country": {
                        "type": "string",
                        "description": "ISO 3166-1 alpha-2 country code"
                    },
                    "year": { "type": "integer" }
                },
                "required": ["country", "year"],
                "additionalProperties": false
            }),
        )
        // The provider loads this definition when the model finds it through search.
        .with_defer_loading(true)
        .into(),
    ];

    let mut messages = vec![Message::user([Part::text(
        "Use tool search to find a tool that lists public holidays, then call it for Vietnam \
         (country code VN) in 2026. Do not answer without calling the holiday tool.",
    )])];

    for _ in 0..10 {
        // Keep all tool definitions available on every request, including deferred
        // ones.
        let response = model
            .generate(LanguageModelInput::new(messages.clone()).with_tools(tools.clone()))
            .await
            .expect("Generation failed");

        // Preserve the complete response, including hosted search calls and results.
        messages.push(Message::assistant(response.content.clone()));

        let mut tool_results: Vec<Part> = Vec::new();
        for part in response.content {
            match part {
                Part::ToolCall(part) => match part.call {
                    ToolCall::ToolSearch(call) => println!("tool search: {call:#?}"),
                    ToolCall::Function(call) => {
                        // The provider executes tool search; the application executes function
                        // calls.
                        assert_eq!(call.name, "lookup_holiday", "Unknown tool");
                        let args: LookupHolidayArgs = serde_json::from_value(call.args)
                            .expect("Failed to parse lookup_holiday args");
                        let result = lookup_holiday(&args);
                        tool_results.push(
                            ToolResultPart::new(
                                part.tool_call_id,
                                call.name,
                                vec![Part::text(result.to_string())],
                            )
                            .into(),
                        );
                    }
                    _ => {}
                },
                Part::ToolResult(part) => {
                    if let ToolResult::ToolSearch(result) = part.result {
                        println!(
                            "discovered tools: {:?} {:?}",
                            part.status, result.tool_names
                        );
                    }
                }
                Part::Text(text) => println!("{}", text.text),
                _ => {}
            }
        }

        if tool_results.is_empty() {
            break;
        }
        messages.push(Message::tool(tool_results));
    }
}
