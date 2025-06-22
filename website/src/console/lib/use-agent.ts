import type {
  AgentItem,
  AgentItemMessage,
  AgentRequest,
  AgentStreamEvent,
  AgentStreamPartialEvent,
} from "@hoangvvo/llm-agent";
import type {
  AudioPart,
  AudioPartDelta,
  ImagePart,
  Part,
  ReasoningPart,
  TextPart,
  ToolCallPart,
} from "@hoangvvo/llm-sdk";
import { stream as eventStream } from "fetch-event-stream";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ModelSelection } from "../components/sidebar.tsx";
import type {
  AgentBehaviorSettings,
  ApiKeys,
  LoggedEvent,
  McpServerConfig,
} from "../types.ts";
import { createId } from "./utils.ts";

interface UseAgentConfig<Context> {
  runStreamUrl: string;
  modelSelection: ModelSelection | null;
  providerApiKeys: ApiKeys;
  userContext: Context;
  enabledTools: string[];
  mcpServers: McpServerConfig[];
  disabledInstructions: boolean;
  agentBehavior: AgentBehaviorSettings;
  toolsInitialized: boolean;
}

interface UseAgentOptions {
  onAudioDelta?: (delta: AudioPartDelta) => void | Promise<void>;
  onToolResult?: (
    toolName: string,
    output: Part[],
    input: Record<string, unknown>,
  ) => void | Promise<void>;
}

interface ConversationSendOptions {
  items: AgentItem[];
}

interface SendUserMessageOptions {
  parts: Part[];
}

interface UseAgentReturn {
  items: AgentItem[];
  nextItems: AgentItem[];
  streamingParts: Part[];
  isStreaming: boolean;
  error: string | null;
  setError: (next: string | null) => void;
  eventLog: LoggedEvent[];
  sendUserMessage: (options: SendUserMessageOptions) => Promise<void>;
  sendConversation: (options: ConversationSendOptions) => Promise<void>;
  abort: () => void;
  resetConversation: () => void;
}

type ServerToClientEvent = AgentStreamEvent | { event: "error"; error: string };

function isAbortError(error: unknown): boolean {
  if (error instanceof DOMException) {
    return error.name === "AbortError";
  }
  if (error instanceof Error) {
    return error.name === "AbortError";
  }
  return false;
}

export function useAgent<Context>(
  config: UseAgentConfig<Context>,
  options: UseAgentOptions = {},
): UseAgentReturn {
  const {
    runStreamUrl,
    modelSelection,
    providerApiKeys,
    userContext,
    enabledTools,
    mcpServers,
    disabledInstructions,
    agentBehavior,
    toolsInitialized,
  } = config;

  const [items, setItems] = useState<AgentItem[]>([]);
  const [nextItems, setNextItems] = useState<AgentItem[]>([]);
  const [streamingParts, setStreamingParts] = useState<Part[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [eventLog, setEventLog] = useState<LoggedEvent[]>([]);

  const itemsRef = useRef<AgentItem[]>([]);
  const abortControllerRef = useRef<AbortController | null>(null);

  useEffect(() => {
    itemsRef.current = items;
  }, [items]);

  useEffect(() => {
    return () => {
      abortControllerRef.current?.abort();
    };
  }, []);

  const logEvent = useCallback(
    (direction: "client" | "server", name: string, payload: unknown) => {
      setEventLog((prev) => [
        ...prev,
        {
          id: createId(),
          direction,
          name,
          timestamp: new Date().toISOString(),
          payload: sanitizePayload(payload),
        },
      ]);
    },
    [],
  );

  const abort = useCallback(() => {
    abortControllerRef.current?.abort();
  }, []);

  const resetConversation = useCallback(() => {
    abort();
    setItems([]);
    setNextItems([]);
    setStreamingParts([]);
  }, [abort]);

  const runAgent = useCallback(
    async (pendingUserItems: AgentItem[]) => {
      if (!modelSelection) {
        setError("Select a model before sending a message");
        return;
      }

      const controller = new AbortController();
      abortControllerRef.current = controller;

      const pendingItems: AgentItem[] = [...pendingUserItems];
      const conversation: AgentItem[] = [
        ...itemsRef.current,
        ...pendingUserItems,
      ];

      let streamingParts: Part[] = [];

      setNextItems(pendingUserItems);
      setError(null);
      setIsStreaming(true);
      setStreamingParts([]);

      try {
        const inputPayload = {
          provider: modelSelection.provider,
          model_id: modelSelection.modelId,
          input: {
            input: conversation,
            context: userContext,
          } satisfies AgentRequest<Context>,
          enabled_tools: toolsInitialized ? enabledTools : undefined,
          mcp_servers: prepareMcpServerPayload(mcpServers),
          disabled_instructions: disabledInstructions,
          temperature: agentBehavior.temperature,
          top_p: agentBehavior.top_p,
          top_k: agentBehavior.top_k,
          frequency_penalty: agentBehavior.frequency_penalty,
          presence_penalty: agentBehavior.presence_penalty,
        };

        logEvent("client", "request", inputPayload);

        const iterator = await eventStream(runStreamUrl, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            ...(providerApiKeys[modelSelection.provider]
              ? { Authorization: providerApiKeys[modelSelection.provider] }
              : {}),
          },
          body: JSON.stringify(inputPayload),
          signal: controller.signal,
          credentials: "include",
        });

        for await (const message of iterator) {
          if (!message.data) {
            continue;
          }
          if (message.data === "[DONE]") {
            break;
          }

          let parsed: ServerToClientEvent;
          try {
            parsed = JSON.parse(message.data) as ServerToClientEvent;
          } catch (err) {
            console.error("Failed to parse stream payload", err, message.data);
            continue;
          }

          logEvent("server", `event:${parsed.event}`, parsed);

          if (parsed.event === "error") {
            throw new Error(parsed.error);
          }

          if (parsed.event === "partial") {
            const deltaPart = parsed.delta?.part;
            if (deltaPart?.type === "audio" && deltaPart.audio_data) {
              try {
                await options.onAudioDelta?.(deltaPart);
              } catch (err) {
                console.error("Failed to handle audio delta", err);
              }
            }
            streamingParts = reduceContentDelta(streamingParts, parsed);
            setStreamingParts([...streamingParts]);
            continue;
          }

          if (parsed.event === "item") {
            const { item: newItem } = parsed;
            setNextItems((prev) => [...prev, newItem]);
            pendingItems.push(newItem);
            setStreamingParts([]);
            // Notify tool results so client can update local context
            if (newItem.type === "tool") {
              const toolItem = newItem as unknown as {
                tool_name: string;
                output: Part[];
                input: Record<string, unknown>;
              };
              try {
                await options.onToolResult?.(
                  toolItem.tool_name,
                  toolItem.output,
                  toolItem.input,
                );
              } catch (err) {
                console.error("onToolResult handler failed", err);
              }
            }
            continue;
          }
        }

        setItems((prev) => {
          const next = [...prev, ...pendingItems];
          itemsRef.current = next;
          return next;
        });
        setNextItems([]);
      } catch (err) {
        if (isAbortError(err)) {
          setItems((prev) => {
            const next = [...prev, ...pendingItems];

            // If there are still streaming parts when aborting, attach them
            // to the last message
            if (streamingParts.length > 0) {
              next.push({
                type: "message",
                role: "assistant",
                content: streamingParts,
              });
            }

            itemsRef.current = next;
            return next;
          });
          setNextItems([]);
        } else if (err instanceof Response) {
          const info = await err.text();
          setError(() =>
            info.trim().length > 0
              ? `Request failed (${String(err.status)}): ${info}`
              : `Request failed with status ${String(err.status)}`,
          );
        } else if (err instanceof Error) {
          setError(err.message);
        } else {
          setError("Unknown error while streaming response");
        }
      } finally {
        abortControllerRef.current = null;
        setStreamingParts([]);
        setIsStreaming(false);
      }
    },
    [
      agentBehavior,
      disabledInstructions,
      enabledTools,
      logEvent,
      modelSelection,
      options.onAudioDelta,
      providerApiKeys,
      runStreamUrl,
      toolsInitialized,
      userContext,
    ],
  );

  const sendConversation = useCallback(
    async ({ items: pending }: ConversationSendOptions) => {
      if (pending.length === 0) {
        return;
      }
      await runAgent(pending);
    },
    [runAgent],
  );

  const sendUserMessage = useCallback(
    async ({ parts }: SendUserMessageOptions) => {
      const userItem: AgentItemMessage = {
        type: "message",
        role: "user",
        content: parts,
      };
      await sendConversation({ items: [userItem] });
    },
    [sendConversation],
  );

  return useMemo(
    () => ({
      items,
      nextItems,
      streamingParts,
      isStreaming,
      error,
      setError,
      eventLog,
      sendUserMessage,
      sendConversation,
      abort,
      resetConversation,
    }),
    [
      abort,
      error,
      eventLog,
      isStreaming,
      items,
      nextItems,
      resetConversation,
      sendConversation,
      sendUserMessage,
      setError,
      streamingParts,
    ],
  );
}

function prepareMcpServerPayload(
  servers: McpServerConfig[],
): McpServerConfig[] | undefined {
  if (servers.length === 0) {
    return undefined;
  }

  const sanitized: McpServerConfig[] = [];

  for (const server of servers) {
    if (server.type === "streamable-http") {
      const url = server.url.trim();
      if (!url) {
        continue;
      }
      const authorization = server.authorization?.trim();
      if (authorization) {
        sanitized.push({ type: "streamable-http", url, authorization });
      } else {
        sanitized.push({ type: "streamable-http", url });
      }
    } else {
      const command = server.command.trim();
      if (!command) {
        continue;
      }
      const args = server.args
        ?.map((arg) => arg.trim())
        .filter((arg) => arg.length > 0);
      if (args && args.length > 0) {
        sanitized.push({ type: "stdio", command, args });
      } else {
        sanitized.push({ type: "stdio", command });
      }
    }
  }

  return sanitized.length > 0 ? sanitized : undefined;
}

/* eslint-disable @typescript-eslint/no-unnecessary-type-assertion */
function reduceContentDelta(
  parts: Part[],
  partial: AgentStreamPartialEvent,
): Part[] {
  const delta = partial.delta;
  if (!delta) {
    return parts;
  }

  const part = delta.part;
  if (typeof part !== "object" || typeof part.type !== "string") {
    return parts;
  }

  const index = typeof delta.index === "number" ? delta.index : parts.length;
  const next = [...parts];

  switch (part.type) {
    case "text": {
      const previousText =
        next[index]?.type === "text" && typeof next[index].text === "string"
          ? next[index].text
          : "";
      const text = `${previousText}${part.text}`;
      next[index] = {
        type: "text",
        text,
      } satisfies TextPart;
      break;
    }
    case "image": {
      const previousImage: ImagePart =
        next[index]?.type === "image"
          ? (next[index] as ImagePart)
          : {
              type: "image",
              mime_type: "image/png",
              image_data: "",
            };
      next[index] = {
        ...previousImage,
        mime_type: part.mime_type ?? previousImage.mime_type,
        image_data: previousImage.image_data + (part.image_data ?? ""),
        width: part.width ?? previousImage.width,
        height: part.height ?? previousImage.height,
        id: part.id ?? previousImage.id,
      } satisfies ImagePart;
      break;
    }
    case "audio": {
      const previousAudio: AudioPart =
        next[index]?.type === "audio"
          ? (next[index] as AudioPart)
          : {
              type: "audio",
              format: part.format ?? "wav",
              audio_data: "",
            };
      next[index] = {
        ...previousAudio,
        format: part.format ?? previousAudio.format,
        audio_data: "",
        sample_rate: part.sample_rate ?? previousAudio.sample_rate,
        channels: part.channels ?? previousAudio.channels,
        transcript: `${previousAudio.transcript ?? ""}${part.transcript ?? ""}`,
        id: part.id ?? previousAudio.id,
      } satisfies AudioPart;
      break;
    }
    case "reasoning": {
      const previousReasoning: ReasoningPart =
        next[index]?.type === "reasoning"
          ? (next[index] as ReasoningPart)
          : { type: "reasoning", text: "" };
      const separator = previousReasoning.text ? "\n" : "";
      next[index] = {
        ...previousReasoning,
        text: `${previousReasoning.text}${separator}${part.text}`,
        signature: part.signature ?? previousReasoning.signature,
        id: part.id ?? previousReasoning.id,
      } satisfies ReasoningPart;
      break;
    }
    case "tool-call": {
      const previousToolCall: ToolCallPart =
        next[index]?.type === "tool-call"
          ? (next[index] as ToolCallPart)
          : {
              type: "tool-call",
              tool_name: part.tool_name ?? "unknown",
              args: {},
              tool_call_id: part.tool_call_id ?? "",
            };
      next[index] = {
        ...previousToolCall,
        tool_name: part.tool_name ?? previousToolCall.tool_name,
        args: {},
        tool_call_id: part.tool_call_id ?? previousToolCall.tool_call_id,
        id: part.id ?? previousToolCall.id,
      } satisfies ToolCallPart;
      break;
    }
    default:
      return next;
  }

  return next;
}

function sanitizePayload(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => sanitizePayload(item));
  }
  if (value && typeof value === "object") {
    const result: Record<string, unknown> = {};
    for (const [key, entry] of Object.entries(
      value as Record<string, unknown>,
    )) {
      if (
        (key === "audio_data" || key === "image_data") &&
        typeof entry === "string"
      ) {
        result[key] = `[${String(entry.length)} bytes base64]`;
      } else {
        result[key] = sanitizePayload(entry);
      }
    }
    return result;
  }
  return value;
}
