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

for (const part of response.content) {
  if (part.type === "tool-call" && part.call.type === "web_search") {
    console.log("web search", part.call.status, part.call.action);
  } else if (part.type === "tool-result" && part.result.type === "web_search") {
    console.log("sources", part.result.sources);
  } else if (part.type === "text") {
    console.log(part.text, part.citations ?? []);
  }
}
