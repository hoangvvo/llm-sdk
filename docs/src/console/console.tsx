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
import { base64ToArrayBuffer, createId } from "./lib/utils.ts";
import { WavStreamPlayer } from "./lib/wavtools/wav_stream_player.ts";
import type { ApiKeys, LoggedEvent, ModelInfo, MyContext } from "./types.ts";

const API_BASE_URL = "http://localhost:4000";
const RUN_STREAM_URL = `${API_BASE_URL}/run-stream`;
const MODELS_URL = `${API_BASE_URL}/models`;

const STORAGE_KEY_MODEL = "console-selected-model";
const STORAGE_KEY_PROVIDER_PREFIX = "console-provider-api-key:";

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
  const [modelSelection, setModelSelection] = useState<ModelSelection | null>(
    null,
  );
  const [modelFetchError, setModelFetchError] = useState<string | null>(null);
  const [providerApiKeys, setProviderApiKeys] = useState<ApiKeys>({});
  const [userContext, setUserContext] = useState<MyContext>({});

  const allItems = useMemo(() => [...items, ...nextItems], [items, nextItems]);

  const abortControllerRef = useRef<AbortController | null>(null);
  const streamPlayerRef = useRef<WavStreamPlayer | null>(null);
  const streamPlayerSampleRateRef = useRef<number | null>(null);
  const streamPlayerConnectedRef = useRef(false);

  useEffect(() => {
    return () => {
      const controller = abortControllerRef.current;
      if (controller) {
        controller.abort();
      }
      const context = streamPlayerRef.current?.context;
      if (context && context.state !== "closed") {
        context.close().catch(() => {
          /* ignore */
        });
      }
    };
  }, []);

  useEffect(() => {
    let cancelled = false;

    async function loadModels() {
      try {
        const response = await fetch(MODELS_URL);
        if (!response.ok) {
          throw new Error(
            `Failed to fetch models (${String(response.status)})`,
          );
        }
        const data = (await response.json()) as ModelInfo[];
        if (cancelled) return;
        const options = data.map(mapModelInfo);
        setModelOptions(options);
        setModelFetchError(null);
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
          const stored = localStorage.getItem(STORAGE_KEY_MODEL);
          if (stored) {
            try {
              const parsed = JSON.parse(stored) as ModelSelection;
              const found = options.find(
                (option) =>
                  option.provider === parsed.provider &&
                  option.modelId === parsed.modelId,
              );
              if (found) {
                setModelSelection(found);
                return;
              }
            } catch (err) {
              console.error("Failed to parse stored model selection", err);
            }
          } else {
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
          }
        } else {
          setModelSelection(null);
        }
      } catch (err) {
        if (cancelled) return;
        setModelOptions([]);
        setModelSelection(null);
        setModelFetchError(
          err instanceof Error ? err.message : "Failed to fetch models",
        );
      }
    }

    void loadModels();
    return () => {
      cancelled = true;
    };
  }, []);

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

  const handleUpdateModelSelection = useCallback<
    React.Dispatch<React.SetStateAction<ModelSelection | null>>
  >((selection) => {
    setModelSelection(selection);
    if (typeof selection === "function") {
      setModelSelection((current) => {
        const next = selection(current);
        if (next) {
          localStorage.setItem(STORAGE_KEY_MODEL, JSON.stringify(next));
        } else {
          localStorage.removeItem(STORAGE_KEY_MODEL);
        }
        return next;
      });
    } else {
      if (selection) {
        localStorage.setItem(STORAGE_KEY_MODEL, JSON.stringify(selection));
      } else {
        localStorage.removeItem(STORAGE_KEY_MODEL);
      }
      setModelSelection(selection);
    }
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

  const ensureStreamPlayer = useCallback(
    async (sampleRate?: number): Promise<WavStreamPlayer> => {
      const desiredRate =
        sampleRate ?? streamPlayerSampleRateRef.current ?? 44100;

      if (
        streamPlayerRef.current &&
        streamPlayerSampleRateRef.current !== null &&
        streamPlayerSampleRateRef.current !== desiredRate
      ) {
        const context = streamPlayerRef.current.context;
        if (context && context.state !== "closed") {
          await context.close().catch(() => {
            /* ignore */
          });
        }
        streamPlayerRef.current = null;
        streamPlayerConnectedRef.current = false;
      }

      if (!streamPlayerRef.current) {
        streamPlayerRef.current = new WavStreamPlayer({
          sampleRate: desiredRate,
        });
        streamPlayerSampleRateRef.current = desiredRate;
        streamPlayerConnectedRef.current = false;
      }

      if (!streamPlayerConnectedRef.current) {
        await streamPlayerRef.current.connect();
        streamPlayerConnectedRef.current = true;
      }

      return streamPlayerRef.current;
    },
    [],
  );

  const handleAudioDelta = useCallback(
    async (delta: AudioPartDelta) => {
      if (!delta.audio_data || delta.audio_data.length === 0) {
        return;
      }
      try {
        const player = await ensureStreamPlayer(delta.sample_rate ?? undefined);
        const buffer = base64ToArrayBuffer(delta.audio_data);
        player.add16BitPCM(buffer, delta.audio_id ?? "default");
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
        const requestContext: MyContext = {};
        const name = userContext.name?.trim();
        if (name) {
          requestContext.name = name;
        }
        const location = userContext.location?.trim();
        if (location) {
          requestContext.location = location;
        }

        const inputPayload = {
          provider: modelSelection.provider,
          model_id: modelSelection.modelId,
          input: {
            input: conversation,
            context: requestContext,
          } satisfies AgentRequest<MyContext>,
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
    [handleAudioDelta, logEvent, modelSelection, providerApiKeys, userContext],
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
          onModelSelectionChange={handleUpdateModelSelection}
          modelSelectionErrorMessage={modelFetchError}
          apiKeys={providerApiKeys}
          onSaveApiKey={handleSaveProviderApiKey}
          context={userContext}
          onContextChange={setUserContext}
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
        audio_id: part.audio_id ?? previousAudio.audio_id,
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
      } satisfies ReasoningPart;
      break;
    }
    case "tool-call": {
      const previousText = next[index]?.type === "text" ? next[index].text : "";
      const segments = [previousText];
      if (part.tool_name) {
        segments.push(`Tool: ${part.tool_name}\n`);
      }
      if (part.args) {
        segments.push(part.args);
      }
      next[index] = {
        type: "text",
        text: segments.join(""),
      } satisfies TextPart;
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
