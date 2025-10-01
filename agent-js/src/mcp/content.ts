import { mapMimeTypeToAudioFormat, type Part } from "@hoangvvo/llm-sdk";
import type {
  AudioContent,
  ImageContent,
  ResourceLink,
  TextContent,
} from "@modelcontextprotocol/sdk/types.js";

export type MCPContent =
  | TextContent
  | ImageContent
  | AudioContent
  | ResourceLink;

export function convertMCPContentToParts(contents: MCPContent[]): Part[] {
  const parts: Part[] = [];

  for (const content of contents) {
    switch (content.type) {
      case "text":
        parts.push({ type: "text", text: content.text });
        break;
      case "image":
        parts.push({
          type: "image",
          data: content.data,
          mime_type: content.mimeType,
        });
        break;
      case "audio":
        parts.push({
          type: "audio",
          data: content.data,
          format: mapMimeTypeToAudioFormat(content.mimeType),
        });
        break;
    }
  }

  return parts;
}
