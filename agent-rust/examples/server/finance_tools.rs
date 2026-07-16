use crate::context::MyContext;
use futures::future::BoxFuture;
use llm_agent::{AgentFunctionTool, AgentToolResult};
use llm_sdk::{JSONSchema, Part};
use serde::Deserialize;
use serde_json::Value;
use std::error::Error;

// Finance Tools
#[derive(Deserialize)]
struct GetStockPriceParams {
    symbol: String,
}

pub struct GetStockPriceTool;

impl AgentFunctionTool<MyContext> for GetStockPriceTool {
    fn name(&self) -> String {
        "get_stock_price".to_string()
    }

    fn description(&self) -> String {
        "Get current or historical stock price information".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "symbol": {
                    "type": "string",
                    "description": "Stock ticker symbol"
                }
            },
            "required": ["symbol"],
            "additionalProperties": false
        })
    }

    fn execute<'a>(
        &'a self,
        args: Value,
        _context: &'a MyContext,
        _state: &'a llm_agent::RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn Error + Send + Sync>>> {
        Box::pin(async move {
            let params: GetStockPriceParams = serde_json::from_value(args)?;

            let url = format!(
                "https://query1.finance.yahoo.com/v8/finance/chart/{}",
                params.symbol
            );
            let client = reqwest::Client::new();

            match client.get(&url).send().await {
                Ok(response) => {
                    if !response.status().is_success() {
                        return Ok(AgentToolResult {
                            content: vec![Part::text(format!(
                                "Failed to get stock price for {}",
                                params.symbol
                            ))],
                            is_error: true,
                        });
                    }

                    match response.json::<Value>().await {
                        Ok(data) => {
                            if let Some(chart) = data.get("chart") {
                                if let Some(results) =
                                    chart.get("result").and_then(|r| r.as_array())
                                {
                                    if let Some(quote) = results.first() {
                                        if let Some(meta) = quote.get("meta") {
                                            let result = serde_json::json!({
                                                "symbol": params.symbol,
                                                "price": meta.get("regularMarketPrice"),
                                                "open": meta.get("regularMarketOpen"),
                                                "high": meta.get("regularMarketDayHigh"),
                                                "low": meta.get("regularMarketDayLow"),
                                                "previous_close": meta.get("previousClose"),
                                                "timestamp": meta.get("regularMarketTime")
                                                    .and_then(serde_json::Value::as_i64)
                                                    .map(|t| chrono::DateTime::from_timestamp(t, 0)
                                                        .map(|dt| dt.to_rfc3339())
                                                        .unwrap_or_default())
                                            });

                                            return Ok(AgentToolResult {
                                                content: vec![Part::text(
                                                    serde_json::to_string_pretty(&result)?,
                                                )],
                                                is_error: false,
                                            });
                                        }
                                    }
                                }
                            }

                            Ok(AgentToolResult {
                                content: vec![Part::text("No data found".to_string())],
                                is_error: true,
                            })
                        }
                        Err(e) => Ok(AgentToolResult {
                            content: vec![Part::text(format!("Error: {e}"))],
                            is_error: true,
                        }),
                    }
                }
                Err(e) => Ok(AgentToolResult {
                    content: vec![Part::text(format!("Error: {e}"))],
                    is_error: true,
                }),
            }
        })
    }
}
