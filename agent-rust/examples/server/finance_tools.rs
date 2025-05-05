use async_trait::async_trait;
use llm_agent::{AgentTool, AgentToolResult};
use llm_sdk::{JSONSchema, Part};
use serde::Deserialize;
use serde_json::Value;
use std::error::Error;

use crate::context::MyContext;

// Finance Tools
#[derive(Deserialize)]
struct GetStockPriceParams {
    symbol: String,
}

pub struct GetStockPriceTool;

#[async_trait]
impl AgentTool<MyContext> for GetStockPriceTool {
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

    async fn execute(
        &self,
        args: Value,
        _context: &MyContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
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
                            if let Some(results) = chart.get("result").and_then(|r| r.as_array()) {
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
                                                .and_then(serde_json::Value::as_f64)
                                                .map(|t| chrono::DateTime::from_timestamp(t as i64, 0)
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
    }
}

// Crypto Tools
#[derive(Deserialize)]
struct GetCryptoPriceParams {
    symbol: String,
    currency: String,
    include_market_data: bool,
}

pub struct GetCryptoPriceTool;

#[async_trait]
impl AgentTool<MyContext> for GetCryptoPriceTool {
    fn name(&self) -> String {
        "get_crypto_price".to_string()
    }

    fn description(&self) -> String {
        "Get cryptocurrency price and market information".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "symbol": {
                    "type": "string",
                    "description": "Cryptocurrency symbol (e.g., bitcoin, ethereum)"
                },
                "currency": {
                    "type": ["string", "null"],
                    "description": "Currency to get price in (default: usd)"
                },
                "include_market_data": {
                    "type": ["boolean", "null"],
                    "description": "Include additional market data"
                }
            },
            "required": ["symbol", "currency", "include_market_data"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: Value,
        _context: &MyContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: GetCryptoPriceParams = serde_json::from_value(args)?;

        let currency = if params.currency.is_empty() {
            "usd"
        } else {
            &params.currency
        };
        let include_market_data = if params.include_market_data {
            "true"
        } else {
            "false"
        };

        let url = format!(
            "https://api.coingecko.com/api/v3/simple/price?ids={}&vs_currencies={}&include_market_cap={}&include_24hr_vol={}&include_24hr_change={}",
            params.symbol, currency, include_market_data, include_market_data, include_market_data
        );

        let client = reqwest::Client::new();

        match client.get(&url).send().await {
            Ok(response) => {
                if !response.status().is_success() {
                    return Ok(AgentToolResult {
                        content: vec![Part::text(format!(
                            "Failed to get crypto price for {}",
                            params.symbol
                        ))],
                        is_error: true,
                    });
                }

                match response.json::<Value>().await {
                    Ok(data) => {
                        if let Some(crypto_data) = data.get(&params.symbol) {
                            let mut result = serde_json::Map::new();
                            result
                                .insert("symbol".to_string(), Value::String(params.symbol.clone()));
                            result.insert(
                                "currency".to_string(),
                                Value::String(currency.to_string()),
                            );

                            if let Some(price) = crypto_data.get(currency) {
                                result.insert("price".to_string(), price.clone());
                            }

                            if params.include_market_data {
                                if let Some(market_cap) =
                                    crypto_data.get(format!("{currency}_market_cap"))
                                {
                                    result.insert("market_cap".to_string(), market_cap.clone());
                                }
                                if let Some(volume) =
                                    crypto_data.get(format!("{currency}_24h_vol"))
                                {
                                    result.insert("24h_volume".to_string(), volume.clone());
                                }
                                if let Some(change) =
                                    crypto_data.get(format!("{currency}_24h_change"))
                                {
                                    result.insert("24h_change".to_string(), change.clone());
                                }
                            }

                            return Ok(AgentToolResult {
                                content: vec![Part::text(serde_json::to_string_pretty(
                                    &Value::Object(result),
                                )?)],
                                is_error: false,
                            });
                        }

                        Ok(AgentToolResult {
                            content: vec![Part::text("Cryptocurrency not found".to_string())],
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
    }
}
