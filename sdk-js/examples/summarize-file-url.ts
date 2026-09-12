import { getModel } from "./get-model.ts";

const provider = process.env["PROVIDER"] ?? "openai";
const modelId = process.env["MODEL"] ?? "gpt-5.6-sol";
const model = getModel(provider, modelId);

const response = await model.generate({
  messages: [
    {
      role: "user",
      content: [
        { type: "text", text: "Summarize the attached PDF." },
        {
          type: "file",
          // The provider fetches this URL; no download or base64 encoding is needed here.
          url: "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf",
          mime_type: "application/pdf",
        },
      ],
    },
  ],
});

console.dir(response, { depth: null });
