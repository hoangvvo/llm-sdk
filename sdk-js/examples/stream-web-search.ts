import { StreamAccumulator } from "@hoangvvo/llm-sdk";
import { getModel } from "./get-model.ts";

const provider = process.env["PROVIDER"] ?? "openai";
const modelId = process.env["MODEL"] ?? "gpt-5.6-sol";
const model = getModel(provider, modelId);

const stream = model.stream({
  messages: [
    {
      role: "user",
      content: [
        {
          type: "text",
          text: "Use web search to find the official IANA page about reserved domains. Reply with one sentence containing the word IANA and cite the source.",
        },
      ],
    },
  ],
  tools: [{ type: "web_search" }],
});

const accumulator = new StreamAccumulator();

let current = await stream.next();
while (!current.done) {
  console.dir(current.value, { depth: null });
  accumulator.addPartial(current.value);
  current = await stream.next();
}

const response = accumulator.computeResponse();
console.dir(response.content, { depth: null });
