import { Agent, tool } from "@hoangvvo/llm-agent";
import { zodTool } from "@hoangvvo/llm-agent/zod";
import type { Message } from "@hoangvvo/llm-sdk";
import { OpenAIModel } from "@hoangvvo/llm-sdk/openai";
import assert from "node:assert";
import { z } from "zod";

// Implement the agent delegation pattern, where a main agent delegates tasks
// to sub-agents. The main agent uses the results from the sub-agents'
// execution to make informed decisions and coordinate overall behavior.
function delegate<TContext>(agent: Agent<TContext>, description: string) {
  return tool({
    name: `transfer_to_${agent.name}`,
    description: `Use this tool to transfer the task to ${agent.name}, which can help with:
${description}`,
    parameters: {
      type: "object",
      properties: {
        task: {
          type: "string",
          description:
            "A clear and concise description of the task the agent should achieve." +
            " Replace any possessive pronouns or ambiguous terms with the actual entity names if possible" +
            " so there is enough information for the agent to process without additional context",
        },
      },
      required: ["task"],
      additionalProperties: false,
    },
    async execute(args: { task: string }, context: TContext) {
      const result = await agent.run({
        context,
        messages: [
          {
            role: "user",
            content: [{ type: "text", text: args.task }],
          },
        ],
      });
      return {
        content: result.content,
        is_error: false,
      };
    },
  });
}

assert(process.env["OPENAI_API_KEY"], "OPENAI_API_KEY must be set");
const model = new OpenAIModel({
  apiKey: process.env["OPENAI_API_KEY"],
  modelId: "gpt-4o",
});

let ORDERS: {
  customer_name: string;
  address: string;
  quantity: number;
  completionTime: Date;
}[] = [];

// Order processing agent
const orderAgent = new Agent({
  name: "order",
  model,
  instructions: [
    "You are an order processing agent. Your job is to handle customer orders efficiently and accurately.",
  ],
  tools: [
    zodTool({
      name: "create_order",
      description: "Create a new customer order",
      parameters: z.object({
        customer_name: z.string(),
        address: z.string(),
        quantity: z.number(),
      }),
      execute(args) {
        console.log(
          `[delivery.create_order] Creating order for ${args.customer_name} with quantity ${String(args.quantity)}`,
        );

        ORDERS.push({
          ...args,
          // Randomly finish between 1 to 10 seconds
          completionTime: new Date(
            Date.now() + Math.floor(Math.random() * 10000) + 1000,
          ),
        });

        return Promise.resolve({
          content: [
            { type: "text", text: JSON.stringify({ status: "creating" }) },
          ],
          is_error: false,
        });
      },
    }),
    zodTool({
      name: "get_orders",
      description:
        "Retrieve the list of customer orders and their status (completed or pending)",
      parameters: z.object({}),
      execute() {
        const now = new Date();

        const result = ORDERS.map(
          ({ customer_name, quantity, address, completionTime }) =>
            ({
              customer_name,
              quantity,
              address,
              status: completionTime <= now ? "completed" : "pending",
            }) as const,
        );

        const completedCount = result.filter(
          (order) => order.status === "completed",
        ).length;

        console.log(
          `[delivery.get_orders] Retrieving orders. Found ${String(completedCount)} completed orders.`,
        );

        // remove completed orders
        ORDERS = ORDERS.filter(({ completionTime }) => completionTime > now);

        return Promise.resolve({
          content: [{ type: "text", text: JSON.stringify(result) }],
          is_error: false,
        });
      },
    }),
  ],
});

// Delivery agent
const deliveryAgent = new Agent({
  name: "delivery",
  model,
  instructions: [
    `You are a delivery agent. Your job is to ensure timely and accurate delivery of customer orders.`,
  ],
  tools: [
    zodTool({
      name: "deliver_order",
      description: "Deliver a customer order",
      parameters: z.object({
        customer_name: z.string(),
        address: z.string(),
      }),
      execute(args) {
        console.log(
          `[delivery.deliver_order] Delivering order for ${args.customer_name} to ${args.address}`,
        );

        return Promise.resolve({
          content: [
            { type: "text", text: JSON.stringify({ status: "delivering" }) },
          ],
          is_error: false,
        });
      },
    }),
  ],
});

// Coordinator agent
const coordinator = new Agent({
  name: "coordinator",
  model,
  instructions: [
    `You are a coordinator agent. Your job is to delegate tasks to the appropriate sub-agents (order processing and delivery) and ensure smooth operation.
You should also poll the order status in every turn to send them for delivery once they are ready.
`,
    `Respond by letting me know what you did and what is the result from the sub-agents.`,
    `For the purpose of demo:
- you can think of random customer name and address. To be fun, use those from fictions and literatures.
- every time you are called (NEXT), you should randomly create 0 to 1 order.`,
  ],
  tools: [
    delegate(orderAgent, "handling customer orders and get order statuses"),
    delegate(deliveryAgent, "delivering processed orders"),
  ],
});

const messages: Message[] = [];

for (;;) {
  console.log("\n--- New iteration ---");

  messages.push({
    role: "user",
    content: [{ type: "text", text: "Next" }],
  });

  const response = await coordinator.run({
    messages,
    context: {},
  });

  console.dir(response.content, { depth: null });

  // Update messages with the new items
  messages.push(...response.items);

  await new Promise((resolve) => setTimeout(resolve, 5000));
}
