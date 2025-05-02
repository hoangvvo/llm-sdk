import type { AudioPartDelta, Part } from "@hoangvvo/llm-sdk";
import { useCallback, useEffect, useMemo, useState } from "react";
import { ChatPane } from "./components/chat-pane.tsx";
import { Composer } from "./components/composer.tsx";
import { EventsPane } from "./components/events-pane.tsx";
import {
  Sidebar,
  type ModelOption,
  type ModelSelection,
} from "./components/sidebar.tsx";
import {
  normalizeBaseUrl,
  parseExampleServerUrls,
} from "./lib/example-server.ts";
import { useAgent } from "./lib/use-agent.ts";
import { useAudio } from "./lib/use-audio.ts";
import { useFetchInitialData } from "./lib/use-fetch-initial-data.ts";
import { useLocalStorageState } from "./lib/use-local-storage-state.ts";
import { base64ToArrayBuffer } from "./lib/utils.ts";
import type {
  AgentBehaviorSettings,
  ApiKeys,
  ModelInfo,
  MyContext,
  ToolInfo,
} from "./types.ts";

const env = import.meta.env as Record<string, string | undefined>;

const DEFAULT_API_BASE_URL = normalizeBaseUrl("http://localhost:4000");
const STORAGE_KEY_SERVER_URL = "console-example-server-url";
const EXAMPLE_SERVER_URL_OPTIONS = parseExampleServerUrls(
  env.EXAMPLE_SERVER_URLS ?? env.PUBLIC_EXAMPLE_SERVER_URLS,
);

const STORAGE_KEY_MODEL = "console-selected-model";
const STORAGE_KEY_PROVIDER_PREFIX = "console-provider-api-key:";
const STORAGE_KEY_CONTEXT = "console-user-context";
const STORAGE_KEY_ENABLED_TOOLS = "console-enabled-tools";
const STORAGE_KEY_DISABLED_INSTRUCTIONS = "console-disabled-instructions";
const STORAGE_KEY_AGENT_BEHAVIOR = "console-agent-behavior";

export function ChatApp() {
  const [activeTab, setActiveTab] = useState<"chat" | "events">("chat");
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

  const hasExampleServerOptions = EXAMPLE_SERVER_URL_OPTIONS.length > 0;
  const [apiBaseUrl, setApiBaseUrl] = useLocalStorageState<string>(
    STORAGE_KEY_SERVER_URL,
    () => EXAMPLE_SERVER_URL_OPTIONS[0] ?? DEFAULT_API_BASE_URL,
  );
  const normalizedApiBaseUrl = useMemo(
    () => normalizeBaseUrl(apiBaseUrl),
    [apiBaseUrl],
  );
  const effectiveApiBaseUrl = hasExampleServerOptions
    ? normalizedApiBaseUrl
    : DEFAULT_API_BASE_URL;

  useEffect(() => {
    if (!hasExampleServerOptions) {
      return;
    }
    if (!EXAMPLE_SERVER_URL_OPTIONS.includes(normalizedApiBaseUrl)) {
      setApiBaseUrl(EXAMPLE_SERVER_URL_OPTIONS[0]);
    }
  }, [hasExampleServerOptions, normalizedApiBaseUrl, setApiBaseUrl]);

  const runStreamUrl = `${effectiveApiBaseUrl}/run-stream`;
  const modelsUrl = `${effectiveApiBaseUrl}/models`;
  const toolsUrl = `${effectiveApiBaseUrl}/tools`;

  const handleServerUrlChange = useCallback(
    (value: string) => {
      setApiBaseUrl(normalizeBaseUrl(value));
    },
    [setApiBaseUrl],
  );

  const [hasStoredToolPreference] = useState(() => {
    if (typeof window === "undefined") {
      return false;
    }
    return window.localStorage.getItem(STORAGE_KEY_ENABLED_TOOLS) !== null;
  });

  const fetchModels = useCallback(
    async (signal: AbortSignal) => {
      const response = await fetch(modelsUrl, { signal });
      if (!response.ok) {
        throw new Error(`Failed to fetch models (${String(response.status)})`);
      }
      return (await response.json()) as ModelInfo[];
    },
    [modelsUrl],
  );

  const {
    data: modelsData,
    error: modelsError,
    refetch: refetchModels,
  } = useFetchInitialData(fetchModels);
  useEffect(() => {
    refetchModels();
  }, [modelsUrl, refetchModels]);

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
        next[option.provider] = storedValue ?? undefined;
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

  const fetchTools = useCallback(
    async (signal: AbortSignal) => {
      const response = await fetch(toolsUrl, { signal });
      if (!response.ok) {
        throw new Error(`Failed to fetch tools (${String(response.status)})`);
      }
      return (await response.json()) as ToolInfo[];
    },
    [toolsUrl],
  );

  const {
    data: toolsData,
    error: toolsError,
    refetch: refetchTools,
  } = useFetchInitialData(fetchTools);
  useEffect(() => {
    refetchTools();
  }, [refetchTools, toolsUrl]);

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

  const { add16BitPCM } = useAudio();

  const handleAudioDelta = useCallback(
    (delta: AudioPartDelta) => {
      if (!delta.audio_data || delta.audio_data.length === 0) {
        return;
      }
      const buffer = base64ToArrayBuffer(delta.audio_data);
      void add16BitPCM(buffer, delta.id ?? "default");
    },
    [add16BitPCM],
  );

  const {
    items,
    nextItems,
    streamingParts,
    isStreaming,
    error,
    setError,
    eventLog,
    sendUserMessage,
    abort,
  } = useAgent<MyContext>(
    {
      runStreamUrl,
      modelSelection,
      providerApiKeys,
      userContext,
      enabledTools,
      disabledInstructions,
      agentBehavior,
      toolsInitialized,
    },
    { onAudioDelta: handleAudioDelta },
  );

  const allItems = useMemo(() => [...items, ...nextItems], [items, nextItems]);

  const handleSendMessage = useCallback(
    async (parts: Part[]) => {
      await sendUserMessage({ parts });
    },
    [sendUserMessage],
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
            onAbort={abort}
            disabled={!modelSelection}
          />
        </section>
        <Sidebar
          serverOptions={
            hasExampleServerOptions ? EXAMPLE_SERVER_URL_OPTIONS : []
          }
          serverUrl={hasExampleServerOptions ? normalizedApiBaseUrl : undefined}
          onServerUrlChange={
            hasExampleServerOptions ? handleServerUrlChange : undefined
          }
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
