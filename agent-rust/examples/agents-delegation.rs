use async_trait::async_trait;
use dotenvy::dotenv;
use futures::lock::{Mutex, MutexGuard};
use llm_agent::{Agent, AgentItem, AgentRequest, AgentTool, AgentToolResult, RunState};
use llm_sdk::{
    openai::{OpenAIModel, OpenAIModelOptions},
    JSONSchema, Message,
};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::{env, error::Error, sync::Arc, time::Duration};
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
struct AgentTransferTool<TCtx> {
    agent: Agent<TCtx>,
    description: String,
}

impl<TCtx> AgentTransferTool<TCtx> {
    fn new(agent: Agent<TCtx>, description: &str) -> Self {
        Self {
            agent,
            description: description.into(),
        }
    }
}

#[async_trait]
impl<TCtx> AgentTool<TCtx> for AgentTransferTool<TCtx>
where
    TCtx: Send + Sync + Clone + 'static,
{
    fn name(&self) -> String {
        format!("transfer_to_{}", &self.agent.name)
    }
    fn description(&self) -> String {
        format!(
            "Use this tool to transfer the task to {}, which can help with:\n{}",
            &self.agent.name, &self.description,
        )
    }
    fn parameters(&self) -> JSONSchema {
        schemars::schema_for!(DelegateParams).into()
    }
    async fn execute(
        &self,
        params: Value,
        context: &TCtx,
        _state: Arc<Mutex<RunState>>,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: DelegateParams = serde_json::from_value(params)?;
        let result = self
            .agent
            .run(AgentRequest {
                input: vec![AgentItem::Message(Message::user(vec![params.task]))],
                context: (*context).clone(),
            })
            .await?;
        Ok(AgentToolResult {
            content: result.content,
            is_error: false,
        })
    }
}

struct Order {
    customer_name: String,
    address: String,
    quantity: u32,
    completion_time: Instant,
}

#[derive(Clone)]
struct MyContext(Arc<Mutex<Vec<Order>>>);

impl MyContext {
    async fn push_order(&self, order: Order) {
        let mut guard = self.0.lock().await;
        guard.push(order);
    }

    async fn get_orders(&self) -> MutexGuard<'_, Vec<Order>> {
        self.0.lock().await
    }

    async fn prune_orders(&self) {
        let now = Instant::now();
        let mut guard = self.0.lock().await;
        guard.retain(|order| order.completion_time > now);
    }
}

#[derive(Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct CreateOrderParams {
    customer_name: String,
    address: String,
    quantity: u32,
}

struct CreateOrderTool;

#[async_trait]
impl AgentTool<MyContext> for CreateOrderTool {
    fn name(&self) -> String {
        "create_order".to_string()
    }
    fn description(&self) -> String {
        "Create a new customer order".to_string()
    }
    fn parameters(&self) -> JSONSchema {
        schemars::schema_for!(CreateOrderParams).into()
    }
    async fn execute(
        &self,
        params: Value,
        context: &MyContext,
        _state: Arc<Mutex<RunState>>,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: CreateOrderParams = serde_json::from_value(params)?;
        println!(
            "[order.create_order] Creating order for {} with quantity {}",
            params.customer_name, params.quantity
        );
        // Randomly finish between 1 to 10 seconds
        let completion_duration = Duration::from_millis((rand::random::<u64>() % 9000) + 1000);
        context
            .push_order(Order {
                customer_name: params.customer_name,
                address: params.address,
                quantity: params.quantity,
                completion_time: Instant::now() + completion_duration,
            })
            .await;
        Ok(AgentToolResult {
            content: vec![serde_json::json!({ "status": "creating" })
                .to_string()
                .into()],
            is_error: false,
        })
    }
}

#[derive(Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct GetOrdersParams {}

#[derive(Serialize)]
struct OrderStatus {
    customer_name: String,
    address: String,
    quantity: u32,
    status: String,
}

struct GetOrdersTool;

#[async_trait]
impl AgentTool<MyContext> for GetOrdersTool {
    fn name(&self) -> String {
        "get_orders".to_string()
    }
    fn description(&self) -> String {
        "Retrieve the list of customer orders and their status (completed or pending)".to_string()
    }
    fn parameters(&self) -> JSONSchema {
        schemars::schema_for!(GetOrdersParams).into()
    }
    async fn execute(
        &self,
        _params: Value,
        context: &MyContext,
        _state: Arc<Mutex<RunState>>,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let now = Instant::now();

        let mut result = Vec::new();
        let mut completed_count = 0;

        let orders_guard = context.get_orders().await;
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
        context.prune_orders().await;

        Ok(AgentToolResult {
            content: vec![serde_json::to_string(&result)?.into()],
            is_error: false,
        })
    }
}

#[derive(Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
struct DeliverOrderParams {
    customer_name: String,
    address: String,
}

pub struct DeliverOrderTool;

#[async_trait]
impl AgentTool<MyContext> for DeliverOrderTool {
    fn name(&self) -> String {
        "deliver_order".to_string()
    }
    fn description(&self) -> String {
        "Deliver a customer order".to_string()
    }
    fn parameters(&self) -> JSONSchema {
        schemars::schema_for!(DeliverOrderParams).into()
    }
    async fn execute(
        &self,
        params: Value,
        _context: &MyContext,
        _state: Arc<Mutex<RunState>>,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: DeliverOrderParams = serde_json::from_value(params)?;
        println!(
            "[delivery.deliver_order] Delivering order for {} to {}",
            params.customer_name, params.address
        );

        Ok(AgentToolResult {
            content: vec![serde_json::json!({ "status": "delivering" })
                .to_string()
                .into()],
            is_error: false,
        })
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    dotenv().ok();

    let model = Arc::new(OpenAIModel::new(OpenAIModelOptions {
        api_key: env::var("OPENAI_API_KEY")
            .expect("OPENAI_API_KEY environment variable must be set"),
        model_id: "gpt-4o".to_string(),
        ..Default::default()
    }));

    // Order processing agent
    let order_agent =  Agent::<MyContext>::builder("order", model.clone())
            .add_instruction("You are an order processing agent. Your job is to handle customer orders efficiently and accurately.")
            .add_tool(CreateOrderTool)
            .add_tool(GetOrdersTool)
            .build();

    // Delivery agent
    let delivery_agent =   Agent::<MyContext>::builder("delivery", model.clone())
            .add_instruction("You are a delivery agent. Your job is to ensure timely and accurate delivery of customer orders.")
            .add_tool(DeliverOrderTool)
            .build();

    // Coordinator agent
    let coordinator =  Agent::<MyContext>::builder("coordinator", model.clone())
            .add_instruction("You are a coordinator agent. Your job is to delegate tasks to the appropriate sub-agents (order processing and delivery) and ensure smooth operation.
You should also poll the order status in every turn to send them for delivery once they are ready.")
            .add_instruction("Respond by letting me know what you did and what is the result from the sub-agents.")
            .add_instruction("For the purpose of demo:
- you can think of random customer name and address. To be fun, use those from fictions and literatures.
- every time you are called (NEXT), you should randomly create 0 to 1 order.")
            .add_tool(AgentTransferTool::new(order_agent, "handling customer orders and get order statuses")) // Delegate order creation to order agent
            .add_tool(AgentTransferTool::new(delivery_agent, "delivering processed orders")) // Delegate delivery to delivery agent
            .build();

    let orders: Arc<Mutex<Vec<Order>>> = Arc::new(Mutex::new(Vec::<Order>::new()));

    let mut input = Vec::new();

    // Main loop
    loop {
        println!("\n--- New iteration ---");

        input.push(AgentItem::Message(Message::user(vec!["Next"])));

        let response = coordinator
            .run(AgentRequest {
                input: input.clone(),
                context: MyContext(orders.clone()),
            })
            .await?;

        println!("Response: {:?}", response.content);

        // Append items with the output items
        input.extend(response.output.clone());

        // Wait 5 seconds before next iteration
        tokio::time::sleep(Duration::from_secs(5)).await;
    }
}
