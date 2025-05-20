# llm-agent for Rust

A Rust library to implement LLM agents that work with any LLM providers.

## Usage

```rust
use async_trait::async_trait;
use dotenvy::dotenv;
use llm_agent::{Agent, AgentItem, AgentRequest, AgentTool, AgentToolResult, RunState};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    JSONSchema, Message, Part,
};
use schemars::JsonSchema;
use serde::Deserialize;
use serde_json::{json, Value};
use std::{
    env,
    error::Error,
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

// Define the agent tools
struct GetWeatherTool;

#[async_trait]
impl AgentTool<MyContext> for GetWeatherTool {
    fn name(&self) -> String {
        "get_weather".to_string()
    }
    fn description(&self) -> String {
        "Get weather for a given city".to_string()
    }
    fn parameters(&self) -> JSONSchema {
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
        })
    }
    async fn execute(
        &self,
        args: Value,
        _context: &MyContext,
        _state: &RunState,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: GetWeatherParams = serde_json::from_value(args)?;
        println!("Getting weather for {}", params.city);

        Ok(AgentToolResult {
            content: vec![Part::text(
                json!({
                    "city": params.city,
                    "forecast": "Sunny",
                    "temperatureC": 25
                })
                .to_string(),
            )],
            is_error: false,
        })
    }
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

struct SendMessageTool;

#[async_trait]
impl AgentTool<MyContext> for SendMessageTool {
    fn name(&self) -> String {
        "send_message".to_string()
    }
    fn description(&self) -> String {
        "Send a text message to a phone number".to_string()
    }
    fn parameters(&self) -> JSONSchema {
        schemars::schema_for!(SendMessageParams).into()
    }
    async fn execute(
        &self,
        args: Value,
        _context: &MyContext,
        _state: &RunState,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: SendMessageParams = serde_json::from_value(args)?;
        println!(
            "Sending message to {}: {}",
            params.phone_number, params.message
        );

        Ok(AgentToolResult {
            content: vec![Part::text(
                json!({
                    "message": params.message,
                    "status": "sent"
                })
                .to_string(),
            )],
            is_error: false,
        })
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    dotenv().ok();

    // Define the model to use for the Agent
    let model = Arc::new(OpenAIModel::new(OpenAIModelOptions {
        api_key: env::var("OPENAI_API_KEY")
            .expect("OPENAI_API_KEY environment variable must be set"),
        model_id: "gpt-4o".to_string(),
        ..Default::default()
    }));

    // Create the Agent
    let my_assistant = Agent::<MyContext>::builder("Mai", model)
        .add_instruction(
            "You are Mai, a helpful assistant. Answer questions to the best of your ability.",
        )
        .add_instruction(|ctx: &MyContext| Ok(format!("You are talking to {}", ctx.user_name)))
        .add_tool(GetWeatherTool)
        .add_tool(SendMessageTool)
        .build();

    // Implement the CLI to interact with the Agent
    let mut items = Vec::<AgentItem>::new();

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
        items.push(AgentItem::Message(Message::user(vec![Part::text(
            user_input,
        )])));

        // Call assistant
        let response = my_assistant
            .run(AgentRequest {
                context: context.clone(),
                input: items.clone(),
            })
            .await?;

        // Append items with the output items
        items.extend(response.output.clone());

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

## Examples

Find examples in the [examples](./examples/) folder:

- [`agent`: Simple Example](./examples/agent.rs)
- [`instructions`: Static and dynamic instructions](./examples/instructions.rs)
- [`tools`: Executable tools](./examples/tools.rs)
- [`structured-output`: Structured Output](./examples/structured-output.rs)
- [`agents-delegation`: Multi-agent Delegation](./examples/agents-delegation.rs)
- [`artifacts`: Artifacts/Canvas feature](./examples/artifacts.rs)
- [`memory`: Memory pattern (core + archival)](./examples/memory.rs)
- [`planner-executor`: Plan TODOs and execute](./examples/planner-executor.rs)

```bash
cargo run --example agent
```

An example server that exposes an API to interact with the agent can be found in [examples/server](./examples/server). This can be used to test the agent with the [console application](../website).

## License

[MIT](https://github.com/hoangvvo/llm-sdk/blob/main/LICENSE)
