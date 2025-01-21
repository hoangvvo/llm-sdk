# llm-agent for Rust

A Rust library to implement LLM agents that work with any LLM providers.

## Usage

```rust
use dotenvy::dotenv;
use llm_agent::{Agent, AgentRequest, AgentTool, AgentToolResult};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    Message, Part, UserMessage,
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
#[serde(deny_unknown_fields)]
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

    // Define the agent tools

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
    let my_assistant = Agent::<MyContext>::builder("Mai", model)
        .add_instruction(
            "You are Mai, a helpful assistant. Answer questions to the best of your ability.",
        )
        .add_instruction(|ctx: &MyContext| format!("You are talking to {}", ctx.user_name))
        .tools(vec![get_weather_tool, send_message_tool])
        .build();

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

        // Add user message as the input
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

        // Update messages with the new items
        messages.extend(response.items.iter().filter_map(|item| match item {
            llm_agent::RunItem::Message(msg) => Some(msg.clone()),
        }));

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
```

Find examples in the [examples](./examples/) folder:

- [`agent`: Simple Example](./examples/agent.rs)
- [`structured-output`: Structured Output](./examples/structured-output.rs)
- [`agents-delegation`: Multi-agent Delegation](./examples/agents-delegation.rs)

```bash
cargo run --example agent
```

## License

[MIT](https://github.com/hoangvvo/llm-sdk/blob/main/LICENSE)
