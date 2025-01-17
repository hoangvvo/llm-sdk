use dotenvy::dotenv;
use futures::FutureExt;
use llm_agent::{Agent, AgentParams, AgentRequest, AgentTool, AgentToolResult, InstructionParam};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    Message, Part, ResponseFormatOption, UserMessage,
};
use serde::Deserialize;
use serde_json::json;
use std::{
    env,
    io::{self, Write},
    sync::Arc,
};

// Define the context interface that can be accessed in the instructions and
// tools
#[derive(Clone)]
struct MyContext {
    pub user_name: String,
}

#[allow(clippy::too_many_lines)]
#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    dotenv().ok();

    // Define the model to use for the Agent
    let model = Arc::new(OpenAIModel::new(OpenAIModelOptions {
        api_key: env::var("OPENAI_API_KEY")
            .expect("OPENAI_API_KEY environment variable must be set"),
        model_id: "gpt-4o".to_string(),
        ..Default::default()
    }));

    // Define tools
    let get_time_tool = AgentTool {
        name: "get_time".to_string(),
        description: "Get the current time".to_string(),
        parameters: json!({
          "type": "object",
          "properties": {},
          "additionalProperties": false
        }),
        execute: Box::new(|_params, _ctx| {
            async move {
                Ok(AgentToolResult {
                    content: vec![Part::Text(
                        json!({ "current_time": chrono::Utc::now().to_rfc3339() })
                            .to_string()
                            .into(),
                    )],
                    is_error: false,
                })
            }
            .boxed()
        }),
    };

    let get_weather_tool = AgentTool {
        name: "get_weather".to_string(),
        description: "Get weather for a given city".to_string(),
        parameters: json!({
          "type": "object",
          "properties": {
            "city": {
              "type": "string",
              "description": "The name of the city"
            }
          },
          "required": ["city"],
          "additionalProperties": false
        }),
        execute: Box::new(|params, _ctx| {
            async move {
                #[derive(Debug, Deserialize)]
                struct GetWeatherParams {
                    city: String,
                }

                let params = serde_json::from_value::<GetWeatherParams>(params)
                    .map_err(|e| anyhow::anyhow!("Invalid parameters: {}", e))?;

                println!("Getting weather for {}", params.city);

                Ok(AgentToolResult {
                    content: vec![Part::Text(
                        json!({
                            "city": params.city,
                            "forecast": "Sunny",
                            "temperatureC": 25
                        })
                        .to_string()
                        .into(),
                    )],
                    is_error: false,
                })
            }
            .boxed()
        }),
    };

    let send_message_tool = AgentTool {
        name: "send_message".to_string(),
        description: "Send a text message".to_string(),
        parameters: json!({
          "type": "object",
          "properties": {
            "message": {
              "type": "string",
              "description": "The message to send"
            },
            "phone_number": {
              "type": "string",
              "description": "The phone number to send the message to"
            }
          },
          "required": ["message", "phone_number"],
          "additionalProperties": false
        }),
        execute: Box::new(|params, _ctx| {
            async move {
                #[derive(Debug, Deserialize)]
                struct SendMessageParams {
                    message: String,
                    phone_number: String,
                }

                let params = serde_json::from_value::<SendMessageParams>(params)
                    .map_err(|e| anyhow::anyhow!("Invalid parameters: {}", e))?;

                println!(
                    "Sending message to {}: {}",
                    params.phone_number, params.message
                );

                Ok(AgentToolResult {
                    content: vec![Part::Text(
                        json!({
                            "message": params.message,
                            "status": "sent"
                        })
                        .to_string()
                        .into(),
                    )],
                    is_error: false,
                })
            }
            .boxed()
        }),
    };

    // Create the Agent
    let my_assistant = Agent::<MyContext>::new(AgentParams {
        name: "Mai".to_string(),
        model,
        instructions: vec![
            InstructionParam::String(
                "You are Mai, a helpful assistant. Answer questions to the best of your ability."
                    .to_string(),
            ),
            // Dynamic instruction
            InstructionParam::Func(|ctx: &MyContext| {
                format!("You are talking to {}", ctx.user_name)
            }),
        ],
        response_format: ResponseFormatOption::Text,
        tools: vec![get_time_tool, get_weather_tool, send_message_tool],
    });

    // Implement the CLI to interact with the Agent
    let mut messages: Vec<Message> = Vec::new();

    // Get user name
    let user_name = read_line("Your name: ")?;

    let context = MyContext { user_name };

    println!("Type 'exit' to quit");

    loop {
        let user_input = read_line("> ")?;

        if user_input.is_empty() {
            continue;
        }

        if user_input.to_lowercase() == "exit" {
            break;
        }

        // Add user message
        messages.push(Message::User(UserMessage {
            content: vec![Part::Text(user_input.into())],
        }));

        // Call assistant
        let response = my_assistant
            .run(AgentRequest {
                context: context.clone(),
                messages: messages.clone(),
            })
            .await?;

        // Update messages with response
        messages = response.messages;

        println!("{:#?}", response.content);
    }

    Ok(())
}

fn read_line(prompt: &str) -> io::Result<String> {
    print!("{prompt}");
    io::stdout().flush()?;

    let mut input = String::new();
    io::stdin().read_line(&mut input)?;
    Ok(input.trim().to_string())
}
