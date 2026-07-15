import { getModel } from "./get-model.ts";

const provider = process.env["PROVIDER"] ?? "openai";
const modelId = process.env["MODEL"] ?? "gpt-5.6-sol";
const model = getModel(provider, modelId);

const response = await model.generate({
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

console.dir(response.content, { depth: null });
