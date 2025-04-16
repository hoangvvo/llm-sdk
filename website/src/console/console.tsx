import type {
  AgentItem,
  AgentItemMessage,
  AgentRequest,
  AgentStreamEvent,
  AgentStreamEventPartial,
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
import { ChatPane } from "./components/chat-pane.tsx";
import { Composer } from "./components/composer.tsx";
import { EventsPane } from "./components/events-pane.tsx";
import {
  Sidebar,
  type ModelOption,
  type ModelSelection,
} from "./components/sidebar.tsx";
import { useAudio } from "./lib/use-audio.ts";
import { useFetchInitialData } from "./lib/use-fetch-initial-data.ts";
import { useLocalStorageState } from "./lib/use-local-storage-state.ts";
import { base64ToArrayBuffer, createId } from "./lib/utils.ts";
import type {
  AgentBehaviorSettings,
  ApiKeys,
  LoggedEvent,
  ModelInfo,
  MyContext,
  ToolInfo,
} from "./types.ts";

const API_BASE_URL = "http://localhost:4000";
const RUN_STREAM_URL = `${API_BASE_URL}/run-stream`;
const MODELS_URL = `${API_BASE_URL}/models`;
const TOOLS_URL = `${API_BASE_URL}/tools`;

const STORAGE_KEY_MODEL = "console-selected-model";
const STORAGE_KEY_PROVIDER_PREFIX = "console-provider-api-key:";
const STORAGE_KEY_CONTEXT = "console-user-context";
const STORAGE_KEY_ENABLED_TOOLS = "console-enabled-tools";
const STORAGE_KEY_DISABLED_INSTRUCTIONS = "console-disabled-instructions";
const STORAGE_KEY_AGENT_BEHAVIOR = "console-agent-behavior";

type ServerToClientEvent = AgentStreamEvent | { event: "error"; error: string };

export function ConsoleApp() {
  const [items, setItems] = useState<AgentItem[]>([]);
  const [nextItems, setNextItems] = useState<AgentItem[]>([]);
  const [streamingParts, setStreamingParts] = useState<Part[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [activeTab, setActiveTab] = useState<"chat" | "events">("chat");
  const [eventLog, setEventLog] = useState<LoggedEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [modelOptions, setModelOptions] = useState<ModelOption[]>([]);
  const [modelSelection, setModelSelection] =
    useLocalStorageState<ModelSelection | null>(STORAGE_KEY_MODEL, null);
  const [toolOptions, setToolOptions] = useState<ToolInfo[]>([]);
  const [enabledTools, setEnabledTools] = useLocalStorageState<string[]>(
    STORAGE_KEY_ENABLED_TOOLS,
    () => [],
  );
  const [disabledInstructions, setDisabledInstructions] =
    useLocalStorageState<boolean>(STORAGE_KEY_DISABLED_INSTRUCTIONS, false);
  const [agentBehavior, setAgentBehavior] =
    useLocalStorageState<AgentBehaviorSettings>(
      STORAGE_KEY_AGENT_BEHAVIOR,
      () => ({}),
    );
  const [providerApiKeys, setProviderApiKeys] = useState<ApiKeys>({});
  const [userContext, setUserContext] = useLocalStorageState<MyContext>(
    STORAGE_KEY_CONTEXT,
    (() => ({})) as () => MyContext,
  );
  const [toolsInitialized, setToolsInitialized] = useState(false);

  const allItems = useMemo(() => [...items, ...nextItems], [items, nextItems]);

  const abortControllerRef = useRef<AbortController | null>(null);
  useEffect(() => {
    const controller = abortControllerRef.current;
    if (controller) {
      controller.abort();
    }
  }, []);

  const fetchModels = useCallback(async (signal: AbortSignal) => {
    const response = await fetch(MODELS_URL, { signal });
    if (!response.ok) {
      throw new Error(`Failed to fetch models (${String(response.status)})`);
    }
    return (await response.json()) as ModelInfo[];
  }, []);

  const { data: modelsData, error: modelsError } =
    useFetchInitialData(fetchModels);

  useEffect(() => {
    if (!modelsData) {
      return;
    }

    const options = modelsData.map(mapModelInfo);
    setModelOptions(options);
    setProviderApiKeys((prev) => {
      const next = { ...prev };
      for (const option of options) {
        const storageKey = getProviderApiKeyStorageKey(option.provider);
        const storedValue = localStorage.getItem(storageKey);
        if (storedValue) {
          next[option.provider] = storedValue;
        } else {
          next[option.provider] = undefined;
        }
      }
      return next;
    });

    if (options.length > 0) {
      setModelSelection((current) => {
        if (current) {
          const stillValid = options.some(
            (option) =>
              option.provider === current.provider &&
              option.modelId === current.modelId,
          );
          if (stillValid) {
            return current;
          }
        }
        return options[0];
      });
    } else {
      setModelSelection(null);
    }
  }, [modelsData, setModelSelection]);

  useEffect(() => {
    if (!modelsError) {
      return;
    }
    setModelOptions([]);
    setModelSelection(null);
  }, [modelsError, setModelSelection]);

  const [hasStoredToolPreference] = useState(() => {
    if (typeof window === "undefined") {
      return false;
    }
    return window.localStorage.getItem(STORAGE_KEY_ENABLED_TOOLS) !== null;
  });

  const fetchTools = useCallback(async (signal: AbortSignal) => {
    const response = await fetch(TOOLS_URL, { signal });
    if (!response.ok) {
      throw new Error(`Failed to fetch tools (${String(response.status)})`);
    }
    return (await response.json()) as ToolInfo[];
  }, []);

  const { data: toolsData, error: toolsError } =
    useFetchInitialData(fetchTools);

  useEffect(() => {
    if (!toolsData) {
      return;
    }

    setToolOptions(toolsData);

    const toolNames = toolsData.map((tool) => tool.name);

    setEnabledTools((previous) => {
      const baseSelection =
        previous.length > 0 || hasStoredToolPreference ? previous : toolNames;
      const baseSet = new Set(baseSelection);
      const normalized = toolNames.filter((name) => baseSet.has(name));
      if (arraysEqual(previous, normalized)) {
        return previous;
      }
      return normalized;
    });

    setToolsInitialized(true);
  }, [hasStoredToolPreference, setEnabledTools, toolsData]);

  useEffect(() => {
    if (!toolsError) {
      return;
    }
    setToolOptions([]);
    setToolsInitialized(true);
  }, [toolsError]);

  const handleSaveProviderApiKey = useCallback(
    (provider: string, apiKey: string) => {
      const trimmed = apiKey.trim();
      setProviderApiKeys((prev) => {
        const next = { ...prev };
        const storageKey = getProviderApiKeyStorageKey(provider);
        if (trimmed) {
          next[provider] = trimmed;
          localStorage.setItem(storageKey, trimmed);
        } else {
          next[provider] = undefined;
          localStorage.removeItem(storageKey);
        }
        return next;
      });
    },
    [],
  );

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

  const { ensureStreamPlayer } = useAudio();

  const handleAudioDelta = useCallback(
    async (delta: AudioPartDelta) => {
      if (!delta.audio_data || delta.audio_data.length === 0) {
        return;
      }
      try {
        const player = await ensureStreamPlayer(delta.sample_rate ?? undefined);
        const buffer = base64ToArrayBuffer(delta.audio_data);
        player.add16BitPCM(buffer, delta.id ?? "default");
      } catch (err) {
        console.error("Failed to play streaming audio", err);
      }
    },
    [ensureStreamPlayer],
  );

  const runAgent = useCallback(
    async (conversation: AgentItem[], initialPendingItems: AgentItem[]) => {
      if (!modelSelection) {
        throw new Error("Model selection is required before sending a message");
      }
      const controller = new AbortController();
      abortControllerRef.current = controller;

      const pendingItems = [...initialPendingItems];

      setNextItems(initialPendingItems);
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
          } satisfies AgentRequest<MyContext>,
          enabled_tools: toolsInitialized ? enabledTools : undefined,
          disabled_instructions: disabledInstructions,
          temperature: agentBehavior.temperature,
          top_p: agentBehavior.top_p,
          top_k: agentBehavior.top_k,
          frequency_penalty: agentBehavior.frequency_penalty,
          presence_penalty: agentBehavior.presence_penalty,
        };

        logEvent("client", "request", inputPayload);

        const iterator = await eventStream(RUN_STREAM_URL, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            ...(providerApiKeys[modelSelection.provider]
              ? { Authorization: providerApiKeys[modelSelection.provider] }
              : {}),
          },
          body: JSON.stringify(inputPayload),
          signal: controller.signal,
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
              void handleAudioDelta(deltaPart);
            }
            setStreamingParts((prev) => reduceContentDelta(prev, parsed));
            continue;
          }

          if (parsed.event === "item") {
            // eslint-disable-next-line @typescript-eslint/no-unused-vars
            const { event: _event, ...item } = parsed;
            const newItem = item as AgentItem;
            setNextItems((prev) => [...prev, newItem]);
            pendingItems.push(newItem);
            setStreamingParts([]);
            continue;
          }
        }
        setItems((prev) => [...prev, ...pendingItems]);
        setNextItems([]);
      } catch (err) {
        if (err instanceof Response) {
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
      handleAudioDelta,
      logEvent,
      modelSelection,
      providerApiKeys,
      toolsInitialized,
      userContext,
    ],
  );

  const handleAbort = useCallback(() => {
    abortControllerRef.current?.abort();
  }, []);

  const handleSendMessage = useCallback(
    async (parts: Part[]) => {
      if (!modelSelection) {
        setError("Select a model before sending a message");
        return;
      }

      const userItem: AgentItemMessage = {
        type: "message",
        role: "user",
        content: parts,
      };

      const nextConversation = [...items, userItem];
      await runAgent(nextConversation, [userItem]);
    },
    [items, modelSelection, runAgent],
  );

  const handleEnabledToolsChange = useCallback(
    (next: string[]) => {
      const toolNames = toolOptions.map((tool) => tool.name);
      const toolNameSet = new Set(toolNames);
      const allowed = new Set(next.filter((name) => toolNameSet.has(name)));
      const normalized = toolNames.filter((name) => allowed.has(name));
      setEnabledTools(normalized);
      localStorage.setItem(
        STORAGE_KEY_ENABLED_TOOLS,
        JSON.stringify(normalized),
      );
    },
    [toolOptions],
  );

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-50 via-white to-rose-50 font-mono text-slate-900">
      <div className="flex h-screen">
        <section className="flex min-w-0 flex-1 flex-col border-slate-200/70 bg-white/75 backdrop-blur-sm">
          <header className="border-b border-slate-200/70 px-8 py-3">
            <div className="flex items-center justify-between">
              <div>
                <h1 className="text-md tracking-tight text-slate-900">
                  Chat Console
                </h1>
              </div>
              <div className="flex items-center gap-2 text-xs tracking-[0.25em] uppercase">
                <button
                  type="button"
                  className={`rounded-full border px-4 py-1.5 transition ${
                    activeTab === "chat"
                      ? "border-slate-900 bg-slate-900 text-white"
                      : "border-slate-300 bg-white/70 text-slate-600 hover:border-slate-400 hover:text-slate-900"
                  }`}
                  onClick={() => {
                    setActiveTab("chat");
                  }}
                >
                  Chat UI
                </button>
                <button
                  type="button"
                  className={`rounded-full border px-4 py-1.5 transition ${
                    activeTab === "events"
                      ? "border-slate-900 bg-slate-900 text-white"
                      : "border-slate-300 bg-white/70 text-slate-600 hover:border-slate-400 hover:text-slate-900"
                  }`}
                  onClick={() => {
                    setActiveTab("events");
                  }}
                >
                  Raw events
                </button>
              </div>
            </div>
          </header>
          {activeTab === "chat" ? (
            <ChatPane items={allItems} streamingParts={streamingParts} />
          ) : (
            <EventsPane events={eventLog} />
          )}
          {error ? (
            <div className="bg-rose-500 py-1 text-center text-xs text-white">
              {error}
            </div>
          ) : null}
          <Composer
            isStreaming={isStreaming}
            onError={setError}
            onSend={handleSendMessage}
            onAbort={handleAbort}
            disabled={!modelSelection}
          />
        </section>
        <Sidebar
          models={modelOptions}
          selection={modelSelection}
          onModelSelectionChange={setModelSelection}
          modelSelectionErrorMessage={modelsError}
          apiKeys={providerApiKeys}
          onSaveApiKey={handleSaveProviderApiKey}
          context={userContext}
          onContextChange={setUserContext}
          behavior={agentBehavior}
          onBehaviorChange={setAgentBehavior}
          tools={toolOptions}
          enabledTools={enabledTools}
          onEnabledToolsChange={handleEnabledToolsChange}
          toolErrorMessage={toolsError}
          disabledInstructions={disabledInstructions}
          onDisabledInstructionsChange={setDisabledInstructions}
          toolsInitialized={toolsInitialized}
        />
      </div>
    </div>
  );
}

function mapModelInfo(info: ModelInfo): ModelOption {
  return {
    provider: info.provider,
    modelId: info.model_id,
    label: `${formatProviderName(info.provider)} – ${info.model_id}`,
    metadata: info.metadata,
  };
}

function arraysEqual<T>(left: T[], right: T[]): boolean {
  if (left === right) {
    return true;
  }
  if (left.length !== right.length) {
    return false;
  }
  for (let index = 0; index < left.length; index += 1) {
    if (left[index] !== right[index]) {
      return false;
    }
  }
  return true;
}

function reduceContentDelta(
  parts: Part[],
  partial: AgentStreamEventPartial,
): Part[] {
  if (!partial.delta) {
    return parts;
  }
  const { index, part } = partial.delta;
  const next = [...parts];

  switch (part.type) {
    case "text": {
      const previousText = next[index]?.type === "text" ? next[index].text : "";
      next[index] = {
        type: "text",
        text: previousText + part.text,
      } satisfies TextPart;
      break;
    }
    case "image": {
      const previousImage: ImagePart =
        next[index]?.type === "image"
          ? next[index]
          : {
              type: "image",
              mime_type: part.mime_type ?? "image/png",
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
          ? next[index]
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
          ? next[index]
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
          ? next[index]
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

function formatProviderName(provider: string): string {
  const lookup: Record<string, string> = {
    openai: "OpenAI",
    "openai-chat-completion": "OpenAI (Chat Completions API)",
    anthropic: "Anthropic",
    google: "Google",
    cohere: "Cohere",
    mistral: "Mistral",
  };
  if (lookup[provider]) {
    return lookup[provider];
  }
  return provider.replace(/-/g, " ").replace(/\b\w/g, (ch) => ch.toUpperCase());
}

function getProviderApiKeyStorageKey(provider: string): string {
  return `${STORAGE_KEY_PROVIDER_PREFIX}${provider}`;
}
