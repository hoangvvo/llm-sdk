use async_trait::async_trait;
use llm_agent::{AgentTool, AgentToolResult};
use llm_sdk::{JSONSchema, Part};
use serde::Deserialize;
use serde_json::Value;
use std::error::Error;

use crate::context::MyContext;

// Weather Tools
#[derive(Deserialize)]
struct GetCoordinatesParams {
    location: String,
}

pub struct GetCoordinatesTool;

#[async_trait]
impl AgentTool<MyContext> for GetCoordinatesTool {
    fn name(&self) -> String {
        "get_coordinates".to_string()
    }

    fn description(&self) -> String {
        "Get latitude and longitude coordinates for a location".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "location": {
                    "type": "string",
                    "description": "Location name (city, state, country)"
                }
            },
            "required": ["location"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: Value,
        context: &MyContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: GetCoordinatesParams = serde_json::from_value(args)?;

        let api_key = context
            .geo_api_key
            .as_ref()
            .ok_or("Geo API key not provided")?;

        let url = format!(
            "https://api.opencagedata.com/geocode/v1/json?q={}&key={}&limit=1",
            urlencoding::encode(&params.location),
            api_key
        );

        let client = reqwest::Client::new();

        match client.get(&url).send().await {
            Ok(response) => {
                if !response.status().is_success() {
                    return Ok(AgentToolResult {
                        content: vec![Part::text(format!(
                            "Failed to get coordinates for '{}'",
                            params.location
                        ))],
                        is_error: true,
                    });
                }

                match response.json::<Value>().await {
                    Ok(data) => {
                        if let Some(results) = data.get("results").and_then(|r| r.as_array()) {
                            if let Some(first_result) = results.first() {
                                if let Some(geometry) = first_result.get("geometry") {
                                    let result = serde_json::json!({
                                        "location": params.location,
                                        "latitude": geometry.get("lat"),
                                        "longitude": geometry.get("lng"),
                                        "formatted": first_result.get("formatted")
                                    });

                                    return Ok(AgentToolResult {
                                        content: vec![Part::text(serde_json::to_string_pretty(
                                            &result,
                                        )?)],
                                        is_error: false,
                                    });
                                }
                            }
                        }

                        Ok(AgentToolResult {
                            content: vec![Part::text("Location not found".to_string())],
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

#[derive(Deserialize)]
struct GetWeatherParams {
    latitude: f64,
    longitude: f64,
    include_forecast: Option<bool>,
}

pub struct GetWeatherTool;

#[async_trait]
impl AgentTool<MyContext> for GetWeatherTool {
    fn name(&self) -> String {
        "get_weather".to_string()
    }

    fn description(&self) -> String {
        "Get current weather information for specific coordinates".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "latitude": {
                    "type": "number",
                    "description": "Latitude coordinate"
                },
                "longitude": {
                    "type": "number",
                    "description": "Longitude coordinate"
                },
                "include_forecast": {
                    "type": ["boolean", "null"],
                    "description": "Include weather forecast (default: false)"
                }
            },
            "required": ["latitude", "longitude", "include_forecast"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: Value,
        context: &MyContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: GetWeatherParams = serde_json::from_value(args)?;

        let api_key = context
            .tomorrow_api_key
            .as_ref()
            .ok_or("Tomorrow API key not provided")?;

        let include_forecast = params.include_forecast.unwrap_or(false);
        let timeline = if include_forecast { "1d" } else { "realtime" };

        let url = format!(
            "https://api.tomorrow.io/v4/weather/{}?location={},{}&apikey={}",
            timeline, params.latitude, params.longitude, api_key
        );

        let client = reqwest::Client::new();

        match client.get(&url).send().await {
            Ok(response) => {
                if !response.status().is_success() {
                    return Ok(AgentToolResult {
                        content: vec![Part::text("Failed to fetch weather data".to_string())],
                        is_error: true,
                    });
                }

                match response.json::<Value>().await {
                    Ok(data) => {
                        let mut result = serde_json::Map::new();
                        result.insert(
                            "latitude".to_string(),
                            Value::Number(serde_json::Number::from_f64(params.latitude).unwrap()),
                        );
                        result.insert(
                            "longitude".to_string(),
                            Value::Number(serde_json::Number::from_f64(params.longitude).unwrap()),
                        );

                        if let Some(data_obj) = data.get("data") {
                            if timeline == "realtime" {
                                if let Some(values) = data_obj.get("values") {
                                    result.insert("current_weather".to_string(), values.clone());
                                }
                            } else if let Some(timelines) = data_obj.get("timelines") {
                                result.insert("forecast".to_string(), timelines.clone());
                            }
                        }

                        Ok(AgentToolResult {
                            content: vec![Part::text(serde_json::to_string_pretty(
                                &Value::Object(result),
                            )?)],
                            is_error: false,
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
