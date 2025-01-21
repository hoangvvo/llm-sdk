use std::{env, sync::Arc, time::Duration};

use dotenvy::dotenv;
use llm_agent::{Agent, AgentRequest, AgentTool, AgentToolResult};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    Message,
};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use tokio::time::Instant;

#[derive(Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct DelegateParams {
    #[schemars(
        description = "A clear and concise description of the task the agent should achieve. 
 Replace any possessive pronouns or ambiguous terms with the actual entity names if possible
 so there is enough information for the agent to process without additional context"
    )]
    task: String,
}

/// Implement the agent delegation pattern, where a main agent delegates tasks
/// to sub-agents. The main agent uses the results from the sub-agents'
/// execution to make informed decisions and coordinate overall behavior.
fn delegate<TCtx>(agent: Arc<Agent<TCtx>>, description: &str) -> AgentTool<TCtx>
where
    TCtx: Send + Clone + Sync + 'static,
{
    AgentTool::<TCtx>::new(
        format!("transfer_to_{}", agent.name),
        format!(
            "Use this tool to transfer the task to {}, which can help with:\n{}",
            agent.name, description,
        ),
        schemars::schema_for!(DelegateParams).into(),
        move |params: DelegateParams, ctx, _| {
            let agent = agent.clone();
            async move {
                let result = agent
                    .run(AgentRequest {
                        messages: vec![Message::user(vec![params.task])],
                        context: (*ctx).clone(),
                    })
                    .await?;
                Ok(AgentToolResult {
                    content: result.content,
                    is_error: false,
                })
            }
        },
    )
}

pub struct Order {
    customer_name: String,
    address: String,
    quantity: u32,
    completion_time: Instant,
}

#[derive(Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct CreateOrderParams {
    customer_name: String,
    address: String,
    quantity: u32,
}

#[derive(Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct GetOrdersParams {}

#[derive(Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct DeliverOrderParams {
    customer_name: String,
    address: String,
}

#[derive(Serialize)]
struct OrderStatus {
    customer_name: String,
    address: String,
    quantity: u32,
    status: String,
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    dotenv().ok();

    let model = Arc::new(OpenAIModel::new(OpenAIModelOptions {
        api_key: env::var("OPENAI_API_KEY")
            .expect("OPENAI_API_KEY environment variable must be set"),
        model_id: "gpt-4o".to_string(),
        ..Default::default()
    }));

    let orders = Arc::new(tokio::sync::Mutex::new(Vec::<Order>::new()));

    // Order processing agent
    let order_agent = {
        let orders_clone = orders.clone();
        Arc::new(
            Agent::<()>::builder("order", model.clone())
                .add_instruction("You are an order processing agent. Your job is to handle customer orders efficiently and accurately.")
                .tools(vec![
                    AgentTool::new(
                        "create_order",
                        "Create a new customer order",
                        schemars::schema_for!(CreateOrderParams).into(),
                        move |params: CreateOrderParams, _, _| {
                            let orders = orders_clone.clone();
                            async move {
                                println!("[order.create_order] Creating order for {} with quantity {}", params.customer_name, params.quantity);

                                let mut orders_guard = orders.lock().await;

                                // Randomly finish between 1 to 10 seconds
                                let completion_duration = Duration::from_millis((rand::random::<u64>() % 9000) + 1000);

                                orders_guard.push(Order {
                                    customer_name: params.customer_name,
                                    address: params.address,
                                    quantity: params.quantity,
                                    completion_time: Instant::now() + completion_duration,
                                });

                                Ok(AgentToolResult {
                                    content: vec![serde_json::json!({ "status": "creating" }).to_string().into()],
                                    is_error: false,
                                })
                            }
                        },
                    ),
                    AgentTool::new(
                        "get_orders",
                        "Retrieve the list of customer orders and their status (completed or pending)",
                        serde_json::json!({
                            "type": "object",
                            "properties": {},
                            "additionalProperties": false,
                            "required": []
                        }),
                        move |_params: GetOrdersParams, _, _| {
                            let orders = orders.clone();
                            async move {
                                let mut orders_guard = orders.lock().await;
                                let now = Instant::now();

                                let mut result = Vec::new();
                                let mut completed_count = 0;

                                for order in orders_guard.iter() {
                                    let status = if order.completion_time <= now {
                                        completed_count += 1;
                                        "completed"
                                    } else {
                                        "pending"
                                    };

                                    result.push(OrderStatus {
                                        customer_name: order.customer_name.clone(),
                                        address: order.address.clone(),
                                        quantity: order.quantity,
                                        status: status.to_string(),
                                    });
                                }
                                println!("[order.get_orders] Retrieving orders. Found {completed_count} completed orders.");

                                // Remove completed orders
                                orders_guard.retain(|order| order.completion_time > now);

                                Ok(AgentToolResult {
                                    content: vec![serde_json::to_string(&result)?.into()],
                                    is_error: false,
                                })
                            }
                        },
                    ),
                ])
                .build(),
        )
    };

    // Delivery agent
    let delivery_agent = Arc::new(
        Agent::<()>::builder("delivery", model.clone())
            .add_instruction("You are a delivery agent. Your job is to ensure timely and accurate delivery of customer orders.")
            .tools(vec![
                AgentTool::new(
                    "deliver_order",
                    "Deliver a customer order",
                    schemars::schema_for!(DeliverOrderParams).into(),
                    |params: DeliverOrderParams, _, _| async move {
                        println!( "[delivery.deliver_order] Delivering order for {} to {}", params.customer_name, params.address);

                        Ok(AgentToolResult {
                            content: vec![serde_json::json!({ "status": "delivering" }).to_string().into()],
                            is_error: false,
                        })
                    },
                ),
            ])
            .build(),
    );

    // Coordinator agent
    let coordinator = Arc::new(
        Agent::<()>::builder("coordinator", model.clone())
            .add_instruction("You are a coordinator agent. Your job is to delegate tasks to the appropriate sub-agents (order processing and delivery) and ensure smooth operation.
You should also poll the order status in every turn to send them for delivery once they are ready.")
            .add_instruction("Respond by letting me know what you did and what is the result from the sub-agents.")
            .add_instruction("For the purpose of demo:
- you can think of random customer name and address. To be fun, use those from fictions and literatures.
- every time you are called (NEXT), you should randomly create 0 to 1 order.")
            .tools(vec![
                delegate(order_agent.clone(), "handling customer orders and get order statuses"),
                delegate(delivery_agent.clone(), "delivering processed orders"),
            ])
            .build(),
    );

    let mut messages = Vec::new();

    // Main loop
    loop {
        println!("\n--- New iteration ---");

        messages.push(Message::user(vec!["Next"]));

        let response = coordinator
            .run(AgentRequest {
                messages: messages.clone(),
                context: (),
            })
            .await?;

        println!("Response: {:?}", response.content);

        // Update messages with the new items
        messages.extend(response.items.iter().filter_map(|item| match item {
            llm_agent::RunItem::Message(msg) => Some(msg.clone()),
        }));

        // Wait 5 seconds before next iteration
        tokio::time::sleep(Duration::from_secs(5)).await;
    }
}
