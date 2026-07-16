import type {
  AgentItem,
  AgentItemMessage,
  AgentStreamEvent,
  AgentStreamPartialEvent,
} from "@hoangvvo/llm-agent";
import type {
  AudioOptions,
  AudioPart,
  AudioPartDelta,
  Citation,
  ImagePart,
  Modality,
  Part,
  ReasoningOptions,
  ReasoningPart,
  TextPart,
  ToolCallPart,
  WebSearchUserLocation,
} from "@hoangvvo/llm-sdk";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type {
  AgentBehaviorSettings,
  ApiKeys,
  LoggedEvent,
  McpServerConfig,
  MyContext,
  WebSearchSettings,
} from "../types.ts";
import { getCredentialProvider } from "../types.ts";
import {
  WEB_SEARCH_OPTIONS_PROVIDERS,
  WEB_SEARCH_PROVIDERS,
} from "../types.ts";
import { createAgent } from "../../../../agent-js/examples/server/agent.ts";
import { createBrowserModel } from "./browser-model.ts";
import type { ModelOption, ModelSelection } from "./use-console-app-state.ts";
import { useLocalStorageState } from "./use-local-storage-state.ts";
import { createId } from "./utils.ts";

interface UseAgentConfig {
  modelSelection: ModelSelection | null;
  model: ModelOption | null | undefined;
  providerApiKeys: ApiKeys;
  userContext: MyContext;
  enabledTools: string[];
  enabledToolkits: string[];
  webSearch: WebSearchSettings;
  mcpServers: McpServerConfig[];
  agentBehavior: AgentBehaviorSettings;
  audio: AudioOptions | undefined;
  reasoning: ReasoningOptions | undefined;
  modalities: Modality[] | undefined;
  historyStorageKey?: string;
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

function isAbortError(error: unknown): boolean {
  if (error instanceof DOMException) {
    return error.name === "AbortError";
  }
  if (error instanceof Error) {
    return error.name === "AbortError";
  }
  return false;
}

export function useAgent(
  config: UseAgentConfig,
  { onAudioDelta, onToolResult }: UseAgentOptions = {},
): UseAgentReturn {
  const {
    modelSelection,
    model,
    providerApiKeys,
    userContext,
    enabledTools,
    enabledToolkits,
    webSearch,
    mcpServers,
    agentBehavior,
    audio,
    reasoning,
    modalities,
    historyStorageKey,
  } = config;

  const hasHistoryKey = Boolean(historyStorageKey);
  const [items, setItems] = useLocalStorageState<AgentItem[]>(
    historyStorageKey ?? "__agent-history-noop__",
    () => [],
  );
  const [nextItems, setNextItems] = useState<AgentItem[]>([]);
  const [streamingParts, setStreamingParts] = useState<Part[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [eventLog, setEventLog] = useState<LoggedEvent[]>([]);

  const itemsRef = useRef<AgentItem[]>([]);
  const streamRef = useRef<AsyncGenerator<AgentStreamEvent, unknown> | null>(
    null,
  );
  const abortRequestedRef = useRef(false);
  const hasHydratedRef = useRef(!hasHistoryKey);

  useEffect(() => {
    itemsRef.current = items;
  }, [items]);

  useEffect(() => {
    return () => {
      void streamRef.current?.return(undefined);
    };
  }, []);

  const logEvent = useCallback(
    (direction: "console" | "agent", name: string, payload: unknown) => {
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
    abortRequestedRef.current = true;
    void streamRef.current?.return(undefined);
  }, []);

  const resetConversation = useCallback(() => {
    abort();
    setItems([]);
    setNextItems([]);
    setStreamingParts([]);
    itemsRef.current = [];
  }, [abort, setItems]);

  const hydrateConversation = useCallback(
    (history: AgentItem[]) => {
      abort();
      setItems(history);
      itemsRef.current = history;
      setNextItems([]);
      setStreamingParts([]);
      hasHydratedRef.current = true;
    },
    [abort, setItems],
  );

  const runAgent = useCallback(
    async (pendingUserItems: AgentItem[]) => {
      if (!modelSelection || !model) {
        setError("Select a model before sending a message");
        return;
      }

      abortRequestedRef.current = false;

      const pendingItems: AgentItem[] = [...pendingUserItems];
      const conversation: AgentItem[] = [
        ...itemsRef.current,
        ...pendingUserItems,
      ];

      let streamingParts: Part[] = [];
      let iterator: AsyncGenerator<AgentStreamEvent, unknown> | null = null;

      setNextItems(pendingUserItems);
      setError(null);
      setIsStreaming(true);
      setStreamingParts([]);

      try {
        const apiKey =
          providerApiKeys[getCredentialProvider(modelSelection.provider)];
        if (!apiKey) {
          throw new Error("Enter an API key for the selected provider");
        }

        const webSearchOptions = prepareWebSearchPayload(
          webSearch,
          modelSelection.provider,
        );
        const mcpServerOptions = prepareMcpServerPayload(mcpServers) ?? [];
        const languageModel = createBrowserModel({
          provider: modelSelection.provider,
          modelId: modelSelection.modelId,
          metadata: model.metadata,
          apiKey,
        });
        const agent = createAgent(languageModel, {
          enabledTools,
          enabledToolkits,
          ...(webSearchOptions ? { webSearch: webSearchOptions } : {}),
          mcpServers: mcpServerOptions,
          ...agentBehavior,
          ...(audio ? { audio } : {}),
          ...(reasoning ? { reasoning } : {}),
          ...(modalities ? { modalities } : {}),
        });
        const request = { input: conversation, context: userContext };

        logEvent("console", "browser:run", {
          execution: "browser",
          provider: modelSelection.provider,
          model_id: modelSelection.modelId,
          input: request,
          enabled_tools: enabledTools,
          enabled_toolkits: enabledToolkits,
          web_search: webSearchOptions,
          mcp_servers: mcpServerOptions,
          ...agentBehavior,
          audio,
          reasoning,
          modalities,
        });

        iterator = agent.runStream(request);
        streamRef.current = iterator;

        for await (const parsed of iterator) {
          logEvent("agent", `event:${parsed.event}`, parsed);

          if (parsed.event === "partial") {
            const deltaPart = parsed.delta?.part;
            if (deltaPart?.type === "audio" && deltaPart.data) {
              try {
                await onAudioDelta?.(deltaPart);
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
                await onToolResult?.(
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

        if (abortRequestedRef.current && streamingParts.length > 0) {
          pendingItems.push({
            type: "message",
            role: "assistant",
            content: streamingParts,
          });
        }
        setItems((prev) => {
          const next = [...prev, ...pendingItems];
          itemsRef.current = next;
          return next;
        });
        setNextItems([]);
      } catch (err) {
        if (abortRequestedRef.current || isAbortError(err)) {
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
        if (streamRef.current === iterator) {
          streamRef.current = null;
        }
        setStreamingParts([]);
        setIsStreaming(false);
      }
    },
    [
      audio,
      agentBehavior,
      enabledTools,
      enabledToolkits,
      logEvent,
      modelSelection,
      model,
      modalities,
      mcpServers,
      onAudioDelta,
      onToolResult,
      providerApiKeys,
      reasoning,
      userContext,
      webSearch,
      setItems,
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
      hydrateConversation,
    }),
    [
      abort,
      hydrateConversation,
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

function prepareWebSearchPayload(
  settings: WebSearchSettings,
  provider: string,
): Omit<WebSearchSettings, "enabled"> | undefined {
  if (!settings.enabled || !WEB_SEARCH_PROVIDERS.includes(provider)) {
    return undefined;
  }

  if (!WEB_SEARCH_OPTIONS_PROVIDERS.includes(provider)) {
    return {};
  }

  const allowedDomains = settings.allowed_domains
    ?.map((domain) => domain.trim())
    .filter((domain) => domain.length > 0);
  const locationEntries = Object.entries(settings.user_location ?? {})
    .map(([key, value]) => [key, value?.trim()] as const)
    .filter((entry): entry is [string, string] => Boolean(entry[1]));
  const userLocation = Object.fromEntries(
    locationEntries,
  ) as WebSearchUserLocation;

  return {
    ...(allowedDomains && allowedDomains.length > 0
      ? { allowed_domains: allowedDomains }
      : {}),
    ...(locationEntries.length > 0 ? { user_location: userLocation } : {}),
  };
}

function prepareMcpServerPayload(
  servers: McpServerConfig[],
): McpServerConfig[] | undefined {
  if (servers.length === 0) {
    return undefined;
  }

  const sanitized: McpServerConfig[] = [];

  for (const server of servers) {
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
  }

  return sanitized.length > 0 ? sanitized : undefined;
}

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
      const previousPart: TextPart =
        next[index]?.type === "text" ? next[index] : { type: "text", text: "" };
      const citations = [...(previousPart.citations ?? [])];
      if (part.citation?.source) {
        const citation: Citation = { source: part.citation.source };
        if (part.citation.title !== undefined) {
          citation.title = part.citation.title;
        }
        if (part.citation.cited_text !== undefined) {
          citation.cited_text = part.citation.cited_text;
        }
        if (part.citation.start_index !== undefined) {
          citation.start_index = part.citation.start_index;
        }
        if (part.citation.end_index !== undefined) {
          citation.end_index = part.citation.end_index;
        }
        if (part.citation.signature !== undefined) {
          citation.signature = part.citation.signature;
        }
        citations.push(citation);
      }
      const textPart: TextPart = {
        type: "text",
        text: `${previousPart.text}${part.text}`,
      };
      if (citations.length > 0) {
        textPart.citations = citations;
      }
      const signature = part.signature ?? previousPart.signature;
      if (signature !== undefined) {
        textPart.signature = signature;
      }
      next[index] = textPart;
      break;
    }
    case "image": {
      const previousImage: ImagePart =
        next[index]?.type === "image"
          ? (next[index] as ImagePart)
          : {
              type: "image",
              mime_type: "image/png",
              data: "",
            };
      next[index] = {
        ...previousImage,
        mime_type: part.mime_type ?? previousImage.mime_type,
        data: previousImage.data + (part.data ?? ""),
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
              data: "",
            };
      next[index] = {
        ...previousAudio,
        format: part.format ?? previousAudio.format,
        data: "",
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
      next[index] = {
        ...previousReasoning,
        text: `${previousReasoning.text ?? ""}${part.text ?? ""}`,
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
      const normalizedKey = key.toLowerCase();
      if (
        normalizedKey === "authorization" ||
        normalizedKey === "token" ||
        normalizedKey.endsWith("_token") ||
        normalizedKey.endsWith("api_key") ||
        normalizedKey.endsWith("apikey")
      ) {
        result[key] = "[redacted]";
      } else if (key === "data" && typeof entry === "string") {
        result[key] = `[${String(entry.length)} bytes base64]`;
      } else {
        result[key] = sanitizePayload(entry);
      }
    }
    return result;
  }
  return value;
}
