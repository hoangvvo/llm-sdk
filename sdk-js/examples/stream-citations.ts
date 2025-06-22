import { StreamAccumulator } from "@hoangvvo/llm-sdk";
import { getModel } from "./get-model.ts";

const model = getModel("anthropic", "claude-opus-4-20250514");

const stream = await model.stream({
  messages: [
    {
      role: "user",
      content: [
        // Provide sources as part of the user message
        {
          type: "source",
          source: "https://health-site.example/articles/coffee-benefits",
          title: "Coffee Health Benefits: What the Research Shows",
          content: [
            {
              type: "text",
              text: [
                "Coffee contains over 1,000 bioactive compounds, with caffeine being the most studied.",
                "A typical 8-ounce cup contains 80-100mg of caffeine.",
                "Research shows moderate coffee consumption (3-4 cups daily) is associated with reduced risk of type 2 diabetes, Parkinson's disease, and liver disease.",
                "The antioxidants in coffee, particularly chlorogenic acid, may contribute to these protective effects beyond just the caffeine content.",
              ].join(" "),
            },
          ],
        },
        {
          type: "text",
          text: [
            "Based on what you know about coffee's health benefits and caffeine content,",
            "what would be the optimal daily coffee consumption for someone who wants the health benefits but is sensitive to caffeine?",
            "Consider timing and metabolism.",
          ].join(" "),
        },
      ],
    },
    {
      role: "assistant",
      content: [
        // The model requests a tool call to get more data, which includes sources
        {
          type: "tool-call",
          tool_call_id: "caffeine_lookup_456",
          tool_name: "lookup",
          args: {
            query:
              "caffeine sensitivity optimal timing metabolism coffee health benefits",
          },
        },
      ],
    },
    {
      role: "tool",
      content: [
        {
          type: "tool-result",
          tool_name: "lookup",
          tool_call_id: "caffeine_lookup_456",
          // Provide other sources as part of the tool result
          content: [
            {
              type: "source",
              source:
                "https://medical-journal.example/2024/caffeine-metabolism-study",
              title:
                "Optimizing Coffee Intake for Caffeine-Sensitive Individuals",
              content: [
                {
                  type: "text",
                  text: [
                    "For caffeine-sensitive individuals, the half-life of caffeine extends to 8-12 hours compared to the average 5-6 hours.",
                    "These individuals experience effects at doses as low as 50mg.",
                    "Research shows consuming 1-2 cups (100-200mg caffeine) before noon provides 75% of coffee's antioxidant benefits while minimizing side effects like insomnia and anxiety.",
                    "Splitting intake into smaller doses (half-cups) throughout the morning can further reduce sensitivity reactions while maintaining beneficial compound levels.",
                  ].join(" "),
                },
              ],
            },
          ],
        },
      ],
    },
  ],
});

const accumulator = new StreamAccumulator();

for await (const partial of stream) {
  console.dir(partial, { depth: null });
  accumulator.addPartial(partial);
}

const response = accumulator.computeResponse();

console.dir(response.content, { depth: null });
