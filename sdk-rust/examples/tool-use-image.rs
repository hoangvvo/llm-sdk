use dotenvy::dotenv;
use llm_sdk::{LanguageModelInput, Message, Part, Tool, ToolCallPart};
use serde_json::json;

mod common;

const RED_PIXEL_PNG_BASE64: &str =
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg==";

struct ColorSample {
    mime_type: &'static str,
    data: &'static str,
}

fn get_color_sample() -> ColorSample {
    println!("[TOOLS get_color_sample()] Returning a red sample image");
    ColorSample {
        mime_type: "image/png",
        data: RED_PIXEL_PNG_BASE64,
    }
}

#[tokio::main]
async fn main() {
    dotenv().ok();

    let model = common::get_model("openai", "gpt-4o");

    let tools: Vec<Tool> = vec![Tool {
        name: "get_color_sample".into(),
        description: "Get a color sample image".into(),
        parameters: json!({
          "type": "object",
          "properties": {},
          "additionalProperties": false
        }),
    }];

    let mut messages = vec![Message::user(vec![Part::text(
        "What color is the image returned by the tool? Answer with one word.",
    )])];

    let mut response;
    let mut max_turn_left = 10;

    loop {
        response = model
            .generate(LanguageModelInput {
                messages: messages.clone(),
                tools: Some(tools.clone()),
                ..Default::default()
            })
            .await
            .unwrap();

        messages.push(Message::assistant(response.content.clone()));

        let tool_calls: Vec<ToolCallPart> = response
            .content
            .iter()
            .filter_map(|p| match p {
                Part::ToolCall(tc) => Some(tc.clone()),
                _ => None,
            })
            .collect();

        if tool_calls.is_empty() {
            break;
        }

        let mut tool_results: Vec<Part> = Vec::new();

        for call in tool_calls {
            let sample = match call.tool_name.as_str() {
                "get_color_sample" => get_color_sample(),
                other => panic!("tool {other} not found"),
            };

            tool_results.push(Part::tool_result(
                call.tool_call_id.clone(),
                call.tool_name.clone(),
                vec![Part::image(sample.data, sample.mime_type)],
            ));
        }

        messages.push(Message::tool(tool_results));

        max_turn_left -= 1;
        if max_turn_left <= 0 {
            break;
        }
    }

    println!("{response:#?}");
}
