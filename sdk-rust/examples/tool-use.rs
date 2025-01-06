use dotenvy::dotenv;
use llm_sdk::{
    AssistantMessage, LanguageModelInput, Message, Part, Tool, ToolCallPart, ToolMessage,
    ToolResultPart, UserMessage,
};
use serde::Deserialize;
use serde_json::{json, Value};

mod common;

const STOCK_PRICE: i64 = 100;

#[derive(Debug)]
struct Account {
    balance: i64,
}
impl Account {
    fn new(balance: i64) -> Self {
        Self { balance }
    }
    fn trade(&mut self, args: &TradeArgs) -> Value {
        println!(
            "[TOOLS trade()] Trading {} shares of {} with action: {}",
            args.quantity, args.symbol, args.action
        );

        let change = match args.action.as_str() {
            "buy" => -args.quantity * STOCK_PRICE,
            "sell" => args.quantity * STOCK_PRICE,
            _ => 0,
        };

        self.balance += change;

        json!({
            "success": true,
            "balance": self.balance,
            "balance_change": change
        })
    }
}

#[derive(Debug, Clone, Deserialize)]
struct TradeArgs {
    action: String, // "buy" | "sell"
    quantity: i64,
    symbol: String,
}

#[tokio::main]
async fn main() {
    dotenv().ok();

    let mut account = Account::new(1000);

    let model = common::get_model("openai", "gpt-4o");

    let tools: Vec<Tool> = vec![Tool {
        name: "trade".into(),
        description: "Trade stocks".into(),
        parameters: json!({
          "type": "object",
          "properties": {
              "action": {
                  "type": "string",
                  "enum": ["buy", "sell"],
                  "description": "The action to perform"
              },
              "quantity": {
                  "type": "number",
                  "description": "The number of stocks to trade"
              },
              "symbol": {
                  "type": "string",
                  "description": "The stock symbol"
              }
          },
          "required": ["action", "quantity", "symbol"],
          "additionalProperties": false
        }),
    }];

    let mut messages = vec![Message::User(UserMessage {
        content: vec![Part::Text("I would like to buy 50 NVDA stocks.".into())],
    })];

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

        messages.push(Message::Assistant(AssistantMessage {
            content: response.content.clone(),
        }));

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
            let result_json = match call.tool_name.as_str() {
                "trade" => {
                    let args: TradeArgs = serde_json::from_value(call.args.clone())
                        .expect("Failed to parse tool call arguments");
                    account.trade(&args)
                }
                other => panic!("tool {other} not found"),
            };

            let result_str = result_json.to_string();

            tool_results.push(Part::ToolResult(ToolResultPart {
                tool_name: call.tool_name.clone(),
                tool_call_id: call.tool_call_id.clone(),
                content: vec![Part::Text(result_str.into())],
                is_error: Some(false),
            }));
        }

        messages.push(Message::Tool(ToolMessage {
            content: tool_results,
        }));

        max_turn_left -= 1;
        if max_turn_left <= 0 {
            break;
        }
    }

    println!("{response:#?}");
}
