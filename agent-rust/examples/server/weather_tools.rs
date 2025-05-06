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
        "Get coordinates (latitude and longitude) from a location name".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "location": {
                    "type": "string",
                    "description": "The location name, e.g. Paris, France"
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

        let env_key = std::env::var("GEO_API_KEY").ok();
        let api_key = context
            .geo_api_key
            .as_ref()
            .or(env_key.as_ref())
            .ok_or("API Key not provided. You can also provide the value on the UI with the Context field 'geo_api_key'. Get a free API key at https://geocode.maps.co/")?;

        let url = format!(
            "https://geocode.maps.co/search?q={}&api_key={}",
            urlencoding::encode(&params.location),
            api_key
        );

        let client = reqwest::Client::new();

        match client.get(&url).send().await {
            Ok(response) => {
                if !response.status().is_success() {
                    return Ok(AgentToolResult {
                        content: vec![Part::text(format!(
                            "Error fetching coordinates: {} {}",
                            response.status().as_u16(),
                            response.status().canonical_reason().unwrap_or("")
                        ))],
                        is_error: true,
                    });
                }

                match response.json::<Vec<Value>>().await {
                    Ok(items) => {
                        if let Some(first_item) = items.first() {
                            if let (Some(lat), Some(lon)) = (
                                first_item.get("lat").and_then(|v| v.as_str()),
                                first_item.get("lon").and_then(|v| v.as_str()),
                            ) {
                                let result = serde_json::json!({
                                    "latitude": lat,
                                    "longitude": lon
                                });

                                return Ok(AgentToolResult {
                                    content: vec![Part::text(serde_json::to_string(&result)?)],
                                    is_error: false,
                                });
                            }
                        }

                        Ok(AgentToolResult {
                            content: vec![Part::text(format!(
                                "No coordinates found for location: {}",
                                params.location
                            ))],
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
    latitude: String,
    longitude: String,
    units: String,
    timesteps: String,
    #[serde(rename = "startTime")]
    start_time: String,
}

pub struct GetWeatherTool;

#[async_trait]
impl AgentTool<MyContext> for GetWeatherTool {
    fn name(&self) -> String {
        "get_weather".to_string()
    }

    fn description(&self) -> String {
        "Get current weather from latitude and longitude".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "latitude": {
                    "type": "string",
                    "description": "The latitude"
                },
                "longitude": {
                    "type": "string",
                    "description": "The longitude"
                },
                "units": {
                    "type": "string",
                    "enum": ["metric", "imperial"],
                    "description": "Units"
                },
                "timesteps": {
                    "type": "string",
                    "enum": ["current", "1h", "1d"],
                    "description": "Timesteps"
                },
                "startTime": {
                    "type": "string",
                    "description": "Start time in ISO format"
                }
            },
            "required": ["latitude", "longitude", "units", "timesteps", "startTime"],
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

        let env_key = std::env::var("TOMORROW_API_KEY").ok();
        let api_key = context
            .tomorrow_api_key
            .as_ref()
            .or(env_key.as_ref())
            .ok_or("API Key not provided. You can also provide the value on the UI with the Context field 'tomorrow_api_key'. Get a free API key at https://tomorrow.io/")?;

        let fields = "temperature,temperatureApparent,humidity";

        let url = format!(
            "https://api.tomorrow.io/v4/timelines?location={},{}&fields={}&timesteps={}&units={}&startTime={}&apikey={}",
            params.latitude, params.longitude, fields, params.timesteps, params.units, params.start_time, api_key
        );

        let client = reqwest::Client::new();

        match client.get(&url).send().await {
            Ok(response) => {
                if !response.status().is_success() {
                    return Ok(AgentToolResult {
                        content: vec![Part::text(format!(
                            "Error fetching weather: {} {}",
                            response.status().as_u16(),
                            response.status().canonical_reason().unwrap_or("")
                        ))],
                        is_error: true,
                    });
                }

                match response.json::<Value>().await {
                    Ok(data) => Ok(AgentToolResult {
                        content: vec![Part::text(serde_json::to_string(&data)?)],
                        is_error: false,
                    }),
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
