use async_trait::async_trait;
use dotenvy::dotenv;
use futures::lock::Mutex;
use llm_agent::{Agent, AgentItem, AgentRequest, AgentTool, AgentToolResult, RunState};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    JSONSchema, Message, Part, ResponseFormatJson, ResponseFormatOption, UserMessage,
};
use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{json, Value};
use std::{env, error::Error, sync::Arc};

#[derive(Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct SearchFlightsParams {
    #[schemars(description = "Origin city/airport")]
    from: String,
    #[schemars(description = "Destination city/airport")]
    to: String,
    #[schemars(description = "Departure date in YYYY-MM-DD")]
    date: String,
}

struct SearchFlightsTool;

#[async_trait]
impl AgentTool<()> for SearchFlightsTool {
    fn name(&self) -> String {
        "search_flights".to_string()
    }
    fn description(&self) -> String {
        "Search for flights between two cities".to_string()
    }
    fn parameters(&self) -> JSONSchema {
        schemars::schema_for!(SearchFlightsParams).into()
    }
    async fn execute(
        &self,
        args: Value,
        _context: &(),
        _state: Arc<Mutex<RunState>>,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: SearchFlightsParams = serde_json::from_value(args)?;
        println!(
            "Searching flights from {} to {} on {}",
            params.from, params.to, params.date
        );
        Ok(AgentToolResult {
            content: vec![Part::Text(
                json!([
                    {
                        "airline": "Vietnam Airlines",
                        "departure": format!("{}T10:00:00", params.date),
                        "arrival": format!("{}T12:00:00", params.date),
                        "price": 150
                    },
                    {
                        "airline": "Southwest Airlines",
                        "departure": format!("{}T11:00:00", params.date),
                        "arrival": format!("{}T13:00:00", params.date),
                        "price": 120
                    }
                ])
                .into(),
            )],
            is_error: false,
        })
    }
}

#[derive(Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct SearchHotelsParams {
    #[schemars(description = "City to search hotels in")]
    city: String,
    #[schemars(description = "Check-in date in YYYY-MM-DD")]
    check_in: String,
    #[schemars(description = "Number of nights to stay")]
    nights: u32,
}

struct SearchHotelsTool;

#[async_trait]
impl AgentTool<()> for SearchHotelsTool {
    fn name(&self) -> String {
        "search_hotels".to_string()
    }
    fn description(&self) -> String {
        "Search for hotels in a specific location".to_string()
    }
    fn parameters(&self) -> JSONSchema {
        schemars::schema_for!(SearchHotelsParams).into()
    }
    async fn execute(
        &self,
        args: Value,
        _context: &(),
        _state: Arc<Mutex<RunState>>,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: SearchHotelsParams = serde_json::from_value(args)?;
        println!(
            "Searching hotels in {} from {} for {} nights",
            params.city, params.check_in, params.nights
        );
        Ok(AgentToolResult {
            content: vec![Part::Text(
                json!([
                    {
                        "name": "The Plaza",
                        "location": params.city.to_string(),
                        "pricePerNight": 150,
                        "rating": 4.8
                    },
                    {
                        "name": "Hotel Ritz",
                        "location": params.city.to_string(),
                        "pricePerNight": 200,
                        "rating": 4.7
                    }
                ])
                .into(),
            )],
            is_error: false,
        })
    }
}

#[allow(clippy::too_many_lines)]
#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    dotenv().ok();

    let model = Arc::new(OpenAIModel::new(OpenAIModelOptions {
        api_key: env::var("OPENAI_API_KEY")
            .expect("OPENAI_API_KEY environment variable must be set"),
        model_id: "gpt-4o".to_string(),
        ..Default::default()
    }));

    // Define the response format
    let response_format = ResponseFormatOption::Json(ResponseFormatJson {
        name: "travel_plan".to_string(),
        description: Some(
            "A structured travel plan including flights, hotels, and weather forecast.".to_string(),
        ),
        schema: Some(json!({
            "type": "object",
            "properties": {
                "destination": {
                    "type": "string"
                },
                "flights": {
                    "type": "array",
                    "items": {
                        "type": "object",
                        "properties": {
                            "airline": {
                                "type": "string"
                            },
                            "departure": {
                                "type": "string"
                            },
                            "arrival": {
                                "type": "string"
                            },
                            "price": {
                                "type": "number"
                            }
                        },
                        "required": [
                            "airline",
                            "departure",
                            "arrival",
                            "price"
                        ],
                        "additionalProperties": false
                    }
                },
                "hotels": {
                    "type": "array",
                    "items": {
                        "type": "object",
                        "properties": {
                            "name": {
                                "type": "string"
                            },
                            "location": {
                                "type": "string"
                            },
                            "pricePerNight": {
                                "type": "number"
                            },
                            "rating": {
                                "type": "number"
                            }
                        },
                        "required": [
                            "name",
                            "location",
                            "pricePerNight",
                            "rating"
                        ],
                        "additionalProperties": false
                    }
                }
            },
            "required": [
                "destination",
                "flights",
                "hotels",
            ],
            "additionalProperties": false
        })),
    });

    let travel_agent = Agent::<()>::builder("Bob", model)
        .add_instruction("You are Bob, a travel agent that helps users plan their trips.")
        .add_instruction(|_ctx: &()| format!("The current time is {}", chrono::Local::now()))
        .response_format(response_format)
        .add_tool(SearchFlightsTool)
        .add_tool(SearchHotelsTool)
        .build();

    let prompt = "Plan a trip from Paris to Tokyo next week";

    let response = travel_agent.run(AgentRequest {
        input: vec![AgentItem::Message(Message::User(UserMessage {
            content: vec![Part::Text(prompt.to_string().into())],
        }))],
        context: (),
    });

    let text_part = response
        .await?
        .content
        .into_iter()
        .find_map(|part| {
            if let Part::Text(text) = part {
                Some(text)
            } else {
                None
            }
        })
        .ok_or("No text part in response")?;

    let val: Value = serde_json::from_str(&text_part.text).expect("Invalid JSON response");

    println!(
        "{}",
        serde_json::to_string_pretty(&val).expect("Failed to format JSON")
    );

    Ok(())
}
