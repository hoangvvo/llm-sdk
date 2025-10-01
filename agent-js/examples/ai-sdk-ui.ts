import {
  Agent,
  type AgentItem,
  type AgentItemTool,
  type AgentStreamEvent,
} from "@hoangvvo/llm-agent";
import { zodTool } from "@hoangvvo/llm-agent/zod";
import {
  mapMimeTypeToAudioFormat,
  type ContentDelta,
  type LanguageModelMetadata,
  type Message,
  type Part,
  type ReasoningPartDelta,
  type TextPart,
  type TextPartDelta,
  type ToolCallPartDelta,
} from "@hoangvvo/llm-sdk";
import { randomUUID } from "node:crypto";
import http from "node:http";
import z from "zod";
import { getModel } from "./get-model.ts";

// ==== Vercel AI SDK types ====

type UIMessageRole = "system" | "user" | "assistant";

type ProviderMetadata = unknown;

type TextUIPart = {
  type: "text";
  text: string;
  state?: "streaming" | "done";
  providerMetadata?: ProviderMetadata;
};

type ReasoningUIPart = {
  type: "reasoning";
  text: string;
  state?: "streaming" | "done";
  providerMetadata?: ProviderMetadata;
};

type StepStartUIPart = {
  type: "step-start";
};

type SourceUrlUIPart = {
  type: "source-url";
  sourceId: string;
  url: string;
  title?: string;
  providerMetadata?: ProviderMetadata;
};

type SourceDocumentUIPart = {
  type: "source-document";
  sourceId: string;
  mediaType: string;
  title: string;
  filename?: string;
  providerMetadata?: ProviderMetadata;
};

type FileUIPart = {
  type: "file";
  url: string;
  mediaType: string;
  filename?: string;
  providerMetadata?: ProviderMetadata;
};

type DataUIPart = {
  type: `data-${string}`;
  id?: string;
  data: unknown;
  transient?: boolean;
};

type ToolInvocationInputStreaming = {
  toolCallId: string;
  state: "input-streaming";
  input?: unknown;
  providerExecuted?: boolean;
};

type ToolInvocationInputAvailable = {
  toolCallId: string;
  state: "input-available";
  input: unknown;
  providerExecuted?: boolean;
  callProviderMetadata?: ProviderMetadata;
};

type ToolInvocationOutputAvailable = {
  toolCallId: string;
  state: "output-available";
  input: unknown;
  output: unknown;
  providerExecuted?: boolean;
  callProviderMetadata?: ProviderMetadata;
  preliminary?: boolean;
};

type ToolInvocationOutputError = {
  toolCallId: string;
  state: "output-error";
  input?: unknown;
  rawInput?: unknown;
  errorText: string;
  providerExecuted?: boolean;
  callProviderMetadata?: ProviderMetadata;
};

type UIToolInvocation =
  | ToolInvocationInputStreaming
  | ToolInvocationInputAvailable
  | ToolInvocationOutputAvailable
  | ToolInvocationOutputError;

type ToolUIPart = { type: `tool-${string}` } & UIToolInvocation;

type DynamicToolUIPart =
  | ({ type: "dynamic-tool"; toolName: string } & ToolInvocationInputStreaming)
  | ({ type: "dynamic-tool"; toolName: string } & ToolInvocationInputAvailable)
  | ({ type: "dynamic-tool"; toolName: string } & ToolInvocationOutputAvailable)
  | ({ type: "dynamic-tool"; toolName: string } & ToolInvocationOutputError);

type UIMessagePart =
  | TextUIPart
  | ReasoningUIPart
  | ToolUIPart
  | DynamicToolUIPart
  | SourceUrlUIPart
  | SourceDocumentUIPart
  | FileUIPart
  | DataUIPart
  | StepStartUIPart;

type UIMessage = {
  id: string;
  role: UIMessageRole;
  parts: UIMessagePart[];
  metadata?: unknown;
};

// ==== Vercel AI SDK stream protocol types ====

type TextStartMessageChunk = {
  type: "text-start";
  id: string;
  providerMetadata?: ProviderMetadata;
};

type TextDeltaMessageChunk = {
  type: "text-delta";
  id: string;
  delta: string;
  providerMetadata?: ProviderMetadata;
};

type TextEndMessageChunk = {
  type: "text-end";
  id: string;
  providerMetadata?: ProviderMetadata;
};

type ReasoningStartMessageChunk = {
  type: "reasoning-start";
  id: string;
  providerMetadata?: ProviderMetadata;
};

type ReasoningDeltaMessageChunk = {
  type: "reasoning-delta";
  id: string;
  delta: string;
  providerMetadata?: ProviderMetadata;
};

type ReasoningEndMessageChunk = {
  type: "reasoning-end";
  id: string;
  providerMetadata?: ProviderMetadata;
};

type ErrorMessageChunk = {
  type: "error";
  errorText: string;
};

type ToolInputStartMessageChunk = {
  type: "tool-input-start";
  toolCallId: string;
  toolName: string;
  providerExecuted?: boolean;
  dynamic?: boolean;
};

type ToolInputDeltaMessageChunk = {
  type: "tool-input-delta";
  toolCallId: string;
  inputTextDelta: string;
};

type ToolInputAvailableMessageChunk = {
  type: "tool-input-available";
  toolCallId: string;
  toolName: string;
  input: unknown;
  providerExecuted?: boolean;
  providerMetadata?: ProviderMetadata;
  dynamic?: boolean;
};

type ToolInputErrorMessageChunk = {
  type: "tool-input-error";
  toolCallId: string;
  toolName: string;
  input: unknown;
  errorText: string;
  providerExecuted?: boolean;
  providerMetadata?: ProviderMetadata;
  dynamic?: boolean;
};

type ToolOutputAvailableMessageChunk = {
  type: "tool-output-available";
  toolCallId: string;
  output: unknown;
  providerExecuted?: boolean;
  dynamic?: boolean;
  preliminary?: boolean;
};

type ToolOutputErrorMessageChunk = {
  type: "tool-output-error";
  toolCallId: string;
  errorText: string;
  providerExecuted?: boolean;
  dynamic?: boolean;
};

type SourceUrlMessageChunk = {
  type: "source-url";
  sourceId: string;
  url: string;
  title?: string;
  providerMetadata?: ProviderMetadata;
};

type SourceDocumentMessageChunk = {
  type: "source-document";
  sourceId: string;
  mediaType: string;
  title: string;
  filename?: string;
  providerMetadata?: ProviderMetadata;
};

type FileMessageChunk = {
  type: "file";
  url: string;
  mediaType: string;
  providerMetadata?: ProviderMetadata;
};

type DataUIMessageChunk = {
  type: `data-${string}`;
  id?: string;
  data: unknown;
  transient?: boolean;
};

type StepStartMessageChunk = {
  type: "start-step";
};

type StepFinishMessageChunk = {
  type: "finish-step";
};

type StartMessageChunk = {
  type: "start";
  messageId?: string;
  messageMetadata?: unknown;
};

type FinishMessageChunk = {
  type: "finish";
  messageMetadata?: unknown;
};

type AbortMessageChunk = {
  type: "abort";
};

type MessageMetadataMessageChunk = {
  type: "message-metadata";
  messageMetadata: unknown;
};

type UIMessageChunk =
  | TextStartMessageChunk
  | TextDeltaMessageChunk
  | TextEndMessageChunk
  | ReasoningStartMessageChunk
  | ReasoningDeltaMessageChunk
  | ReasoningEndMessageChunk
  | ErrorMessageChunk
  | ToolInputStartMessageChunk
  | ToolInputDeltaMessageChunk
  | ToolInputAvailableMessageChunk
  | ToolInputErrorMessageChunk
  | ToolOutputAvailableMessageChunk
  | ToolOutputErrorMessageChunk
  | SourceUrlMessageChunk
  | SourceDocumentMessageChunk
  | FileMessageChunk
  | DataUIMessageChunk
  | StepStartMessageChunk
  | StepFinishMessageChunk
  | StartMessageChunk
  | FinishMessageChunk
  | AbortMessageChunk
  | MessageMetadataMessageChunk;

type MessageStreamEvent = UIMessageChunk;

type ChatTrigger = "submit-message" | "regenerate-message";

interface ChatRequestBody {
  id?: string;
  trigger?: ChatTrigger;
  messageId?: string;
  messages?: UIMessage[];
  provider?: string;
  modelId?: string;
  metadata?: LanguageModelMetadata;
  [key: string]: unknown;
}

// ==== Agent setup ====

type ChatContext = Record<string, never>;

const timeTool = zodTool({
  name: "get_current_time",
  description: "Get the current server time in ISO 8601 format.",
  parameters: z.object({}),
  execute() {
    return {
      content: [
        {
          type: "text",
          text: new Date().toISOString(),
        } satisfies TextPart,
      ],
      is_error: false,
    };
  },
});

const weatherTool = zodTool({
  name: "get_local_weather",
  description:
    "Return a lightweight weather forecast for a given city using mock data.",
  parameters: z.object({
    location: z.string().describe("City name to look up weather for."),
  }),
  execute({ location }) {
    const conditions = ["sunny", "cloudy", "rainy", "breezy"];
    const condition = conditions[location.length % conditions.length];
    return {
      content: [
        {
          type: "text",
          text: JSON.stringify({
            location,
            condition,
            temperatureC: 18 + (location.length % 10),
          }),
        } satisfies TextPart,
      ],
      is_error: false,
    };
  },
});

/**
 * Builds an `Agent` that mirrors the model/provider configuration requested by
 * the UI. The returned agent streams `AgentStreamEvent`s that we later adapt
 * into Vercel's data stream protocol.
 */
function createAgent(
  provider: string,
  modelId: string,
  metadata?: LanguageModelMetadata,
) {
  const model = getModel(provider, modelId, metadata);

  return new Agent<ChatContext>({
    name: "UIExampleAgent",
    model,
    instructions: [
      "You are an assistant orchestrated by the llm-agent SDK.",
      "Use the available tools when they can provide better answers.",
    ],
    tools: [timeTool, weatherTool],
  });
}

// ==== Streaming helpers ====

interface TextStreamState {
  id: string;
}

interface ReasoningStreamState {
  id: string;
}

interface ToolCallStreamState {
  toolCallId: string;
  toolName: string;
  argsBuffer: string;
  didEmitStart: boolean;
}

interface SSEWriter {
  write: (event: MessageStreamEvent) => void;
  close: () => void;
}

/**
 * Wraps the raw `ServerResponse` with helpers for emitting Server-Sent Events.
 * Vercel's AI SDK data stream protocol uses SSE under the hood, so having a
 * dedicated writer keeps the transport concerns isolated from the adapter
 * logic that follows.
 */
function createSSEWriter(res: http.ServerResponse): SSEWriter {
  return {
    write(event: MessageStreamEvent) {
      res.write(`data: ${JSON.stringify(event)}\n\n`);
    },
    close() {
      res.write("data: [DONE]\n\n");
      res.end();
    },
  };
}

/**
 * Attempts to parse a JSON payload coming from streaming tool arguments or
 * tool results. The protocol expects structured data, but tool authors can
 * return arbitrary strings. If parsing fails we fall back to the raw string so
 * the UI can still render something meaningful.
 */
function safeJsonParse(rawText: string): unknown {
  try {
    return JSON.parse(rawText);
  } catch {
    return rawText;
  }
}

/**
 * Bridges `AgentStreamEvent`s to the Vercel AI SDK data stream protocol.
 *
 * - If you use this class with `llm-agent`, pass all events emitted by
 * `Agent.runStream` to the `write` method.
 * - If you use this class with `llm-sdk` directly, pass all events emitted by
 * `LanguageModel.stream` to the `writeDelta` method.
 */
class DataStreamProtocolAdapter {
  readonly #writer: SSEWriter;
  readonly #textStateMap = new Map<number, TextStreamState>();
  readonly #reasoningStateMap = new Map<number, ReasoningStreamState>();
  readonly #toolCallStateMap = new Map<number, ToolCallStreamState>();
  #stepHasStarted = false;
  #closed = false;

  constructor(res: http.ServerResponse) {
    this.#writer = createSSEWriter(res);
    const messageId = `msg_${randomUUID()}`;
    this.#writer.write({ type: "start", messageId });
  }

  /**
   * Consumes one `AgentStreamEvent` emitted by `Agent.runStream`. Each event is
   * translated into the corresponding data stream chunks expected by the AI
   * SDK frontend.
   */
  write(event: AgentStreamEvent): void {
    if (this.#closed) {
      return;
    }

    switch (event.event) {
      case "partial":
        this.#ensureStepStarted();
        if (event.delta) {
          this.writeDelta(event.delta);
        }
        break;
      case "item":
        this.#finishStep();
        if (event.item.type === "tool") {
          this.#ensureStepStarted();
          this.#writeForToolItem(event.item);
          this.#finishStep();
        }
        break;
      case "response":
        // The final agent response does not translate to an extra stream part.
        break;
    }
  }

  /**
   * Emits an error chunk so the frontend can surface failures alongside the
   * running message instead of silently terminating the stream.
   */
  emitError(errorText: string): void {
    if (this.#closed) {
      return;
    }
    this.#writer.write({ type: "error", errorText });
  }

  /**
   * Flushes any open stream parts, emits the terminating `finish` message, and
   * ends the SSE connection.
   */
  close(): void {
    if (this.#closed) {
      return;
    }
    this.#finishStep();
    this.#writer.write({ type: "finish" });
    this.#writer.close();
    this.#closed = true;
  }

  #ensureStepStarted(): void {
    if (this.#stepHasStarted) {
      return;
    }
    this.#stepHasStarted = true;
    this.#writer.write({ type: "start-step" });
  }

  #finishStep(): void {
    if (!this.#stepHasStarted) {
      return;
    }
    this.#flushStates();
    this.#stepHasStarted = false;
    this.#writer.write({ type: "finish-step" });
  }

  #flushStates(): void {
    for (const [index, textState] of this.#textStateMap) {
      this.#writer.write({ type: "text-end", id: textState.id });
      this.#textStateMap.delete(index);
    }

    for (const [index, reasoningState] of this.#reasoningStateMap) {
      this.#writer.write({ type: "reasoning-end", id: reasoningState.id });
      this.#reasoningStateMap.delete(index);
    }

    for (const [index, toolCallState] of this.#toolCallStateMap) {
      const { toolCallId, toolName, argsBuffer } = toolCallState;
      if (toolCallId && toolName && argsBuffer.length > 0) {
        this.#writer.write({
          type: "tool-input-available",
          toolCallId,
          toolName,
          input: safeJsonParse(argsBuffer),
        });
      }
      this.#toolCallStateMap.delete(index);
    }
  }

  /**
   * Consumes one `ContentDelta` emitted by `LanguageModel.stream`. Each delta is
   * translated into the corresponding data stream chunks expected by the AI
   * SDK frontend.
   */
  writeDelta(contentDelta: ContentDelta): void {
    switch (contentDelta.part.type) {
      case "text":
        this.#writeForTextPartDelta(contentDelta.index, contentDelta.part);
        break;
      case "reasoning":
        this.#writeForReasoningPartDelta(contentDelta.index, contentDelta.part);
        break;
      case "audio":
        this.#flushStates();
        break;
      case "image":
        this.#flushStates();
        break;
      case "tool-call":
        this.#writeForToolCallPartDelta(contentDelta.index, contentDelta.part);
        break;
    }
  }

  #writeForTextPartDelta(index: number, part: TextPartDelta): void {
    let existingTextState = this.#textStateMap.get(index);
    if (!existingTextState) {
      this.#flushStates();
      existingTextState = { id: `text_${randomUUID()}` };
      this.#textStateMap.set(index, existingTextState);
      this.#writer.write({ type: "text-start", id: existingTextState.id });
    }

    this.#writer.write({
      type: "text-delta",
      id: existingTextState.id,
      delta: part.text,
    });
  }

  #writeForReasoningPartDelta(index: number, part: ReasoningPartDelta): void {
    let existingReasoningState = this.#reasoningStateMap.get(index);
    if (!existingReasoningState) {
      this.#flushStates();
      existingReasoningState = { id: `reasoning_${part.id ?? randomUUID()}` };
      this.#reasoningStateMap.set(index, existingReasoningState);
      this.#writer.write({
        type: "reasoning-start",
        id: existingReasoningState.id,
      });
    }

    this.#writer.write({
      type: "reasoning-delta",
      id: existingReasoningState.id,
      delta: part.text,
    });
  }

  #writeForToolCallPartDelta(index: number, part: ToolCallPartDelta): void {
    let existingToolCallState = this.#toolCallStateMap.get(index);
    if (!existingToolCallState) {
      this.#flushStates();
      existingToolCallState = {
        toolCallId: part.tool_call_id ?? "",
        toolName: part.tool_name ?? "",
        argsBuffer: "",
        didEmitStart: false,
      };
      this.#toolCallStateMap.set(index, existingToolCallState);
    }

    existingToolCallState.toolCallId =
      part.tool_call_id ?? existingToolCallState.toolCallId;
    existingToolCallState.toolName =
      part.tool_name ?? existingToolCallState.toolName;

    if (
      !existingToolCallState.didEmitStart &&
      existingToolCallState.toolCallId.length > 0 &&
      existingToolCallState.toolName.length > 0
    ) {
      existingToolCallState.didEmitStart = true;
      this.#writer.write({
        type: "tool-input-start",
        toolCallId: existingToolCallState.toolCallId,
        toolName: existingToolCallState.toolName,
      });
    }

    if (part.args) {
      existingToolCallState.argsBuffer += part.args;
      this.#writer.write({
        type: "tool-input-delta",
        toolCallId: existingToolCallState.toolCallId,
        inputTextDelta: part.args,
      });
    }
  }

  #writeForToolItem(item: AgentItemTool): void {
    this.#flushStates();
    const textParts = item.output
      .filter((part): part is TextPart => part.type === "text")
      .map((part) => part.text)
      .join("");

    const hasTextOutput = textParts.length > 0;
    const parsedOutput = hasTextOutput ? safeJsonParse(textParts) : item.output;

    this.#writer.write({
      type: "tool-output-available",
      toolCallId: item.tool_call_id,
      output: parsedOutput,
    });
  }
}

// ==== Adapter layers ====

/**
 * Converts UI message parts produced by the Vercel AI SDK components back into
 * the core `Part` representation understood by `@hoangvvo/llm-sdk`. This lets
 * us replay prior chat history into the agent while preserving tool calls and
 * intermediate reasoning steps.
 */
function uiMessagePartToPart(part: UIMessagePart): Part[] {
  if (part.type === "text") {
    return [
      {
        type: "text",
        text: part.text,
      },
    ];
  }
  if (part.type === "reasoning") {
    return [
      {
        type: "reasoning",
        text: part.text,
      },
    ];
  }
  if (part.type === "dynamic-tool") {
    return [
      {
        type: "tool-call",
        args: part.input as Record<string, unknown>,
        tool_call_id: part.toolCallId,
        tool_name: part.toolName,
      },
    ];
  }
  if (part.type === "file") {
    // part.url is in the format of "data:<mediaType>;base64,<data>"
    // We only interest in the raw base64 data for our representation
    let data: string;
    const sepIndex = part.url.indexOf(",");
    if (sepIndex !== -1) {
      data = part.url.slice(sepIndex + 1);
    } else {
      data = part.url;
    }
    if (part.mediaType.startsWith("image/")) {
      return [
        {
          type: "image",
          data: data,
          mime_type: part.mediaType,
        },
      ];
    }
    if (part.mediaType.startsWith("audio/")) {
      return [
        {
          type: "audio",
          data: data,
          format: mapMimeTypeToAudioFormat(part.mediaType),
        },
      ];
    }
    if (part.mediaType.startsWith("text/")) {
      return [
        {
          type: "text",
          text: Buffer.from(data, "base64").toString("utf-8"),
        },
      ];
    }
    // Unsupported file type
    return [];
  }
  if (part.type.startsWith("tool-")) {
    const toolUIPart = part as ToolUIPart;
    const toolName = part.type.slice("tool-".length);
    switch (toolUIPart.state) {
      case "input-available":
        return [
          {
            type: "tool-call",
            args: toolUIPart.input as Record<string, unknown>,
            tool_call_id: toolUIPart.toolCallId,
            tool_name: toolName,
          },
        ];
      case "output-available":
        return [
          {
            type: "tool-call",
            args: toolUIPart.input as Record<string, unknown>,
            tool_call_id: toolUIPart.toolCallId,
            tool_name: toolName,
          },
          {
            type: "tool-result",
            content: [
              {
                type: "text",
                text: JSON.stringify(toolUIPart.output),
              },
            ],
            tool_call_id: toolUIPart.toolCallId,
            tool_name: toolName,
          },
        ];
      case "output-error":
        return [
          {
            type: "tool-call",
            args: toolUIPart.input as Record<string, unknown>,
            tool_call_id: toolUIPart.toolCallId,
            tool_name: toolName,
          },
          {
            type: "tool-result",
            content: [
              {
                type: "text",
                text: toolUIPart.errorText,
              },
            ],
            tool_call_id: toolUIPart.toolCallId,
            tool_name: toolName,
            is_error: true,
          },
        ];
    }
  }
  return [];
}

/**
 * Flattens the UI message history into an `Message[]` so it can be passed to
 * `Agent.runStream`. The agent expects user, assistant, and tool messages as
 * separate timeline entries; this helper enforces the ordering invariants while
 * translating the UI-specific tool call format.
 */
function uiMessagesToMessages(messages: UIMessage[]): Message[] {
  const items: Message[] = [];

  // We will work with all AgentItemMessage of role "user", "assistant", and "tool"
  // There can only be two possible sequences:
  // - user -> assistant -> user -> assistant ...
  // - user -> assistant -> tool -> assistant -> tool -> assistant ...

  for (const message of messages) {
    switch (message.role) {
      case "user": {
        const parts = message.parts.flatMap(uiMessagePartToPart);
        if (parts.length > 0) {
          items.push({
            role: "user",
            content: parts,
          });
        }
        break;
      }
      case "assistant": {
        // The assistant case is a bit tricky because the message may contain
        // tool calls and tool results. UIMessage does not have a specialized
        // role for tool results.
        const parts = message.parts.flatMap(uiMessagePartToPart);
        for (const part of parts) {
          switch (part.type) {
            // Handle assistant final output parts
            case "text":
            case "reasoning":
            case "audio":
            case "image": {
              // If the last item is an assistant message, append to it
              const lastItem = items[items.length - 1];
              const secondLastItem = items[items.length - 2];
              if (lastItem?.role === "assistant") {
                lastItem.content.push(part);
              } else if (
                lastItem?.role === "tool" &&
                secondLastItem?.role === "assistant"
              ) {
                secondLastItem.content.push(part);
              } else {
                items.push({
                  role: "assistant",
                  content: [part],
                });
              }
              break;
            }
            case "tool-call": {
              const lastItem = items[items.length - 1];
              const secondLastItem = items[items.length - 2];
              if (lastItem?.role === "assistant") {
                lastItem.content.push(part);
              } else if (
                lastItem?.role === "tool" &&
                secondLastItem?.role === "assistant"
              ) {
                secondLastItem.content.push(part);
              } else {
                items.push({
                  role: "assistant",
                  content: [part],
                });
              }
              break;
            }
            case "tool-result": {
              const lastItem = items[items.length - 1];
              if (lastItem?.role === "tool") {
                lastItem.content.push(part);
              } else {
                items.push({
                  role: "tool",
                  content: [part],
                });
              }
            }
          }
        }
        break;
      }
    }
  }

  return items;
}

// ==== HTTP handlers ====

/**
 * HTTP handler for the `/api/chat` endpoint. It validates the incoming
 * `ChatRequestBody`, rehydrates the chat history, and streams the agent run
 * back to the frontend using `DataStreamProtocolAdapter` so the response follows
 * the Vercel AI SDK data streaming protocol.
 */
async function handleChatRequest(
  req: http.IncomingMessage,
  res: http.ServerResponse,
) {
  try {
    const bodyText = await readRequestBody(req);

    let body: ChatRequestBody;
    try {
      body = JSON.parse(bodyText) as ChatRequestBody;
    } catch (err) {
      res.writeHead(400, { "Content-Type": "application/json" });
      res.end(
        JSON.stringify({
          error: err instanceof Error ? err.message : "Invalid request body",
        }),
      );
      return;
    }

    const provider =
      typeof body.provider === "string" ? body.provider : "openai";
    const modelId =
      typeof body.modelId === "string" ? body.modelId : "gpt-4o-mini";
    const agent = createAgent(provider, modelId, body.metadata);

    const uiHistory = Array.isArray(body.messages) ? body.messages : [];
    const historyMessages = uiMessagesToMessages(uiHistory);
    const items: AgentItem[] = historyMessages.map((message) => ({
      type: "message",
      ...message,
    }));

    res.writeHead(200, {
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache, no-transform",
      Connection: "keep-alive",
      "x-vercel-ai-ui-message-stream": "v1",
      "Access-Control-Allow-Origin": "*",
    });

    res.flushHeaders?.();

    const adapter = new DataStreamProtocolAdapter(res);

    let clientClosed = false;
    req.on("close", () => {
      clientClosed = true;
    });

    const stream = agent.runStream({
      context: {},
      input: items,
    });

    try {
      for await (const event of stream) {
        if (clientClosed) {
          break;
        }
        adapter.write(event);
      }
    } catch (err) {
      console.error(err);
      adapter.emitError(err instanceof Error ? err.message : "Unknown error");
    }

    if (clientClosed && typeof stream.return === "function") {
      await stream.throw(new Error("Client closed connection"));
    }

    if (!clientClosed) {
      adapter.close();
    }
  } catch (err) {
    console.error(err);
    res.writeHead(500, { "Content-Type": "application/json" });
    res.end(
      JSON.stringify({
        error: err instanceof Error ? err.message : "Unknown error",
      }),
    );
  }
}

/**
 * Accumulates the raw request payload so we can decode the JSON `ChatRequestBody`.
 */
function readRequestBody(req: http.IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    let data = "";
    req.on("data", (chunk) => {
      data += String(chunk);
    });
    req.on("end", () => resolve(data));
    req.on("error", reject);
  });
}

const port = 8000;

http
  .createServer((req, res) => {
    if (req.method === "OPTIONS") {
      res.writeHead(204, {
        "Access-Control-Allow-Origin": "*",
        "Access-Control-Allow-Headers": "content-type",
      });
      res.end();
      return;
    }

    if (req.method === "POST" && req.url === "/api/chat") {
      void handleChatRequest(req, res);
      return;
    }

    res.writeHead(404, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "Not found" }));
  })
  .listen(port, () => {
    console.log(
      `AI SDK UI example server listening on http://localhost:${port}`,
    );
  });
