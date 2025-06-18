import {
  Agent,
  tool,
  type AgentItem,
  type AgentResponse,
  type AgentToolResult,
} from "@hoangvvo/llm-agent";
import { SpanStatusCode, trace } from "@opentelemetry/api";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { resourceFromAttributes } from "@opentelemetry/resources";
import { SimpleSpanProcessor } from "@opentelemetry/sdk-trace-base";
import { NodeTracerProvider } from "@opentelemetry/sdk-trace-node";
import { getModel } from "./get-model.ts";

const provider = new NodeTracerProvider({
  resource: resourceFromAttributes({
    "service.name": "agent-js-tracing-example",
  }),
  spanProcessors: [new SimpleSpanProcessor(new OTLPTraceExporter())],
});

provider.register();

// We'll use this tracer inside tool implementations for nested spans.
const tracer = trace.getTracer("examples/agent-js/tracing");

interface AgentContext {
  customer_name: string;
}

const model = getModel("openai", "gpt-4o-mini");

const getWeatherTool = tool({
  name: "get_weather",
  description: "Get the current weather for a city",
  parameters: {
    type: "object",
    properties: {
      city: { type: "string", description: "City to get the weather for" },
    },
    required: ["city"],
    additionalProperties: false,
  },
  async execute({ city }: { city: string }) {
    return tracer.startActiveSpan(
      "tools.get_weather",
      async (span): Promise<AgentToolResult> => {
        try {
          // Record the city lookup while simulating work.
          span.setAttribute("weather.city", city);
          await new Promise((resolve) => setTimeout(resolve, 100));
          return {
            content: [
              {
                type: "text",
                text: JSON.stringify({
                  city,
                  forecast: "Sunny",
                  temperatureC: 24,
                }),
              },
            ],
            is_error: false,
          };
        } catch (error) {
          span.recordException(error as Error);
          span.setStatus({
            code: SpanStatusCode.ERROR,
            message: String(error),
          });
          throw error;
        } finally {
          span.end();
        }
      },
    );
  },
});

const notifyContactTool = tool({
  name: "send_notification",
  description: "Send a text message to a recipient",
  parameters: {
    type: "object",
    properties: {
      phone_number: { type: "string" },
      message: { type: "string" },
    },
    required: ["phone_number", "message"],
    additionalProperties: false,
  },
  async execute({
    phone_number,
    message,
  }: {
    phone_number: string;
    message: string;
  }) {
    return tracer.startActiveSpan(
      "tools.send_notification",
      async (span): Promise<AgentToolResult> => {
        try {
          // Capture metadata about the outbound notification.
          span.setAttribute("notification.phone", phone_number);
          span.setAttribute("notification.message_length", message.length);
          await new Promise((resolve) => setTimeout(resolve, 80));
          return {
            content: [
              {
                type: "text",
                text: JSON.stringify({ status: "sent", phone_number, message }),
              },
            ],
            is_error: false,
          };
        } catch (error) {
          span.recordException(error as Error);
          span.setStatus({
            code: SpanStatusCode.ERROR,
            message: String(error),
          });
          throw error;
        } finally {
          span.end();
        }
      },
    );
  },
});

const agent = new Agent<AgentContext>({
  name: "Trace Assistant",
  model,
  instructions: [
    // Keep these instructions aligned with the Rust/Go tracing examples.
    "Coordinate weather updates and notifications for clients.",
    "When a request needs both a forecast and a notification, call get_weather before send_notification and summarize the tool results in your reply.",
    ({ customer_name }) =>
      `When asked to contact someone, include a friendly note from ${customer_name}.`,
  ],
  tools: [getWeatherTool, notifyContactTool],
});

// Single-turn request that forces both tools to run.
const items: AgentItem[] = [
  {
    type: "message",
    role: "user",
    content: [
      {
        type: "text",
        text: "Please check the weather for Seattle today and text Mia at +1-555-0100 with the summary.",
      },
    ],
  },
];

const response: AgentResponse = await agent.run({
  context: { customer_name: "Skyline Tours" },
  input: items,
});

console.log(JSON.stringify(response.content, null, 2));

await provider.forceFlush();

await provider.shutdown();
