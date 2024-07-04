import { AnthropicModel } from "../src/anthropic/anthropic.js";
import { GoogleModel } from "../src/google/google.js";
import { OpenAIModel } from "../src/openai/openai.js";

export const openaiModel = new OpenAIModel({
  modelId: "gpt-4o",
  apiKey: process.env["OPENAI_API_KEY"] as string,
});
export const anthropicModel = new AnthropicModel({
  modelId: "claude-3-opus-20240229",
  apiKey: process.env["ANTHROPIC_API_KEY"] as string,
});
export const googleModel = new GoogleModel({
  modelId: "gemini-1.5-pro",
  apiKey: process.env["GOOGLE_API_KEY"] as string,
});
