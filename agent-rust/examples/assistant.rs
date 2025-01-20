use dotenvy::dotenv;
use llm_agent::{Agent, AgentParams, AgentRequest, AgentTool, AgentToolResult, InstructionParam};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    Message, Part, ResponseFormatOption, UserMessage,
};
use schemars::JsonSchema;
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

#[derive(Debug, Deserialize)]
struct GetWeatherParams {
    city: String,
}

// Define the JSON schema using `schemars` crate
#[derive(Debug, Deserialize, JsonSchema)]
struct SendMessageParams {
    #[schemars(description = "The message to send")]
    message: String,
    #[schemars(description = "The phone number to send the message to")]
    phone_number: String,
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

    let get_weather_tool = AgentTool::new(
        "get_weather",
        "Get weather for a given city",
        json!({
          "type": "object",
          "properties": {
            "city": {
              "type": "string",
              "description": "The city to get the weather for"
            }
          },
          "required": ["city"],
          "additionalProperties": false
        }),
        |params: GetWeatherParams, _ctx: Arc<MyContext>| async move {
            println!("Getting weather for {}", params.city);

            Ok(AgentToolResult {
                content: vec![Part::Text(
                    json!({
                        "city": params.city,
                        "forecast": "Sunny",
                        "temperatureC": 25
                    })
                    .into(),
                )],
                is_error: false,
            })
        },
    );

    let send_message_tool = AgentTool::new(
        "send_message",
        "Send a text message to a phone number",
        schemars::schema_for!(SendMessageParams).into(),
        |params: SendMessageParams, _ctx| async move {
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
                    .into(),
                )],
                is_error: false,
            })
        },
    );

    // Create the Agent
    let my_assistant = Agent::<MyContext>::new(AgentParams {
        name: "Mai".to_string(),
        model,
        instructions: vec![
            // Static instruction
            "You are Mai, a helpful assistant. Answer questions to the best of your ability."
                .into(),
            // Dynamic instruction
            InstructionParam::Func(|ctx: &MyContext| {
                format!("You are talking to {}", ctx.user_name)
            }),
        ],
        response_format: ResponseFormatOption::Text,
        tools: vec![get_weather_tool, send_message_tool],
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
