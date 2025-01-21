use dotenvy::dotenv;
use llm_agent::{Agent, AgentRequest, AgentTool, AgentToolResult};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    Message, Part, ResponseFormatJson, ResponseFormatOption, UserMessage,
};
use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::json;
use std::{env, sync::Arc};

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

#[allow(clippy::too_many_lines)]
#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    dotenv().ok();

    let model = Arc::new(OpenAIModel::new(OpenAIModelOptions {
        api_key: env::var("OPENAI_API_KEY")
            .expect("OPENAI_API_KEY environment variable must be set"),
        model_id: "gpt-4o".to_string(),
        ..Default::default()
    }));

    let search_flights_tool = AgentTool::new(
        "search_flights",
        "Search for flights between two cities",
        schemars::schema_for!(SearchFlightsParams).into(),
        |params: SearchFlightsParams, _ctx| async move {
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
        },
    );

    let search_hotels_tool = AgentTool::new(
        "search_hotels",
        "Search for hotels in a specific location",
        schemars::schema_for!(SearchHotelsParams).into(),
        |params: SearchHotelsParams, _ctx| async move {
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
                    .to_string()
                    .into(),
                )],
                is_error: false,
            })
        },
    );

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
        .tools(vec![search_flights_tool, search_hotels_tool])
        .build();

    let prompt = "Plan a trip from Paris to Tokyo next week";

    let response = travel_agent.run(AgentRequest {
        messages: vec![Message::User(UserMessage {
            content: vec![Part::Text(prompt.to_string().into())],
        })],
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

    let val: serde_json::Value =
        serde_json::from_str(&text_part.text).expect("Invalid JSON response");

    println!(
        "{}",
        serde_json::to_string_pretty(&val).expect("Failed to format JSON")
    );

    Ok(())
}
