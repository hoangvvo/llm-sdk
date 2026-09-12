import { getModel } from "./get-model.ts";

const fileUrl =
  "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf";
const fileRes = await fetch(fileUrl);
if (!fileRes.ok) {
  throw new Error(`Failed to fetch file: ${String(fileRes.status)}`);
}
const file = await fileRes.arrayBuffer();

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
          data: Buffer.from(file).toString("base64"),
          mime_type: "application/pdf",
          filename: "dummy.pdf",
        },
      ],
    },
  ],
});

console.dir(response, { depth: null });
