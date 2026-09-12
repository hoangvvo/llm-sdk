import type { Message, Tool, ToolMessage } from "@hoangvvo/llm-sdk";
import { StreamAccumulator } from "@hoangvvo/llm-sdk";
import { getModel } from "./get-model.ts";

function lookupHoliday({ country, year }: { country: string; year: number }) {
  console.log(`[TOOLS lookup_holiday()] ${country} ${String(year)}`);

  // Sample data for this example. Replace with a holiday service in an application.
  if (country.toUpperCase() !== "VN" || year !== 2026) {
    throw new Error("Sample data is only available for VN in 2026");
  }
  return { holidays: [{ date: "2026-09-02", name: "National Day" }] };
}

const provider = process.env["PROVIDER"] ?? "openai";
const modelId = process.env["MODEL"] ?? "gpt-5.6-sol";
const model = getModel(provider, modelId);

const tools: Tool[] = [
  { type: "tool_search" },
  {
    type: "function",
    name: "lookup_holiday",
    description: "Look up the public holidays of a country in a given year",
    parameters: {
      type: "object",
      properties: {
        country: {
          type: "string",
          description: "ISO 3166-1 alpha-2 country code",
        },
        year: { type: "integer" },
      },
      required: ["country", "year"],
      additionalProperties: false,
    },
    // The provider loads this definition when the model finds it through search.
    defer_loading: true,
  },
];

const messages: Message[] = [
  {
    role: "user",
    content: [
      {
        type: "text",
        text: "Use tool search to find a tool that lists public holidays, then call it for Vietnam (country code VN) in 2026. Do not answer without calling the holiday tool.",
      },
    ],
  },
];

for (let turn = 0; turn < 10; turn++) {
  // Keep all tool definitions available on every request, including deferred ones.
  const stream = model.stream({ messages, tools });
  const accumulator = new StreamAccumulator();

  for await (const partial of stream) {
    console.dir(partial, { depth: null });
    accumulator.addPartial(partial);
  }

  // Execute function calls only after their streamed arguments are complete.
  const response = accumulator.computeResponse();

  // Preserve the complete response, including hosted search calls and results.
  messages.push({ role: "assistant", content: response.content });

  const toolMessage: ToolMessage = { role: "tool", content: [] };
  for (const part of response.content) {
    if (part.type === "tool-call" && part.call.type === "tool_search") {
      console.log("tool search", part.call.status, part.call.args);
    } else if (
      part.type === "tool-result" &&
      part.result.type === "tool_search"
    ) {
      console.log("discovered tools", part.status, part.result.tool_names);
    } else if (part.type === "tool-call" && part.call.type === "function") {
      // The provider executes tool search; the application executes function calls.
      if (part.call.name !== "lookup_holiday") {
        throw new Error(`Tool ${part.call.name} not found`);
      }
      const result = lookupHoliday(
        part.call.args as { country: string; year: number },
      );
      toolMessage.content.push({
        type: "tool-result",
        tool_call_id: part.tool_call_id,
        status: "completed",
        result: {
          type: "function",
          name: part.call.name,
          content: [{ type: "text", text: JSON.stringify(result) }],
        },
      });
    } else if (part.type === "text") {
      console.log(part.text);
    }
  }

  if (toolMessage.content.length === 0) break;
  messages.push(toolMessage);
}
