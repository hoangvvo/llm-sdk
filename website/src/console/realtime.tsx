import type { AudioPart } from "@hoangvvo/llm-sdk";
import { useMicVAD, utils as vadUtils } from "@ricky0123/vad-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { ChatPane } from "./components/chat-pane.tsx";
import {
  Sidebar,
  type ModelOption,
  type ModelSelection,
} from "./components/sidebar.tsx";
import { reduceArtifactsFromToolParts } from "./lib/artifacts.ts";
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

type SessionPhase =
  | "idle"
  | "capturing"
  | "listening"
  | "processing"
  | "responding";
type SpeakerState = "idle" | "user" | "assistant";
const SESSION_THEMES: Record<
  SessionPhase,
  { label: string; description: string; accent: string }
> = {
  idle: {
    label: "Mic Muted",
    description: "The capture channel is muted.",
    accent: "text-slate-400",
  },
  listening: {
    label: "Waiting",
    description: "Awaiting voice activity detection to seal the turn.",
    accent: "text-rose-400",
  },
  capturing: {
    label: "Listening",
    description: "Recording your current turn.",
    accent: "text-rose-500",
  },
  processing: {
    label: "Processing",
    description: "Packaging audio for the agent.",
    accent: "text-amber-400",
  },
  responding: {
    label: "Responding",
    description: "The agent is streaming audio back in realtime.",
    accent: "text-sky-400",
  },
};

const CAPTURE_SAMPLE_RATE = 16_000;
const CAPTURE_CHANNEL_COUNT = 1;
const VAD_MODEL_BASE_URL = "/src/vad-web/";
const ORT_WASM_BASE_URL = "/src/onnxruntime-web/";

interface MicVadState {
  listening: boolean;
  errored: string | false;
  loading: boolean;
  userSpeaking: boolean;
  pause: () => void;
  start: () => void;
  toggle: () => void;
}

export function RealtimeApp() {
  const [visualVolume, setVisualVolume] = useState(0.12);
  const [turnCount, setTurnCount] = useState(0);

  const [modelOptions, setModelOptions] = useState<ModelOption[]>([]);
  const [modelSelection, setModelSelection] =
    useLocalStorageState<ModelSelection | null>(STORAGE_KEY_MODEL, null);
  const [providerApiKeys, setProviderApiKeys] = useState<ApiKeys>({});
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

  const handleServerUrlChange = useCallback(
    (value: string) => {
      setApiBaseUrl(normalizeBaseUrl(value));
    },
    [setApiBaseUrl],
  );

  const runStreamUrl = `${effectiveApiBaseUrl}/run-stream`;
  const modelsUrl = `${effectiveApiBaseUrl}/models`;
  const toolsUrl = `${effectiveApiBaseUrl}/tools`;

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

  const { add16BitPCM, interruptPlayback, isPlaying } = useAudio();

  const {
    isStreaming: agentIsStreaming,
    error: agentError,
    sendUserMessage,
    abort: abortAgent,
    items,
    nextItems,
    streamingParts,
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
    {
      onAudioDelta: (delta) => {
        if (!delta.audio_data || delta.audio_data.length === 0) {
          return;
        }
        try {
          const buffer = base64ToArrayBuffer(delta.audio_data);
          const pcm = new Int16Array(buffer);
          void add16BitPCM(pcm);
        } catch (err) {
          console.error("Failed to play streaming audio", err);
        }
      },
      onToolResult: (_toolName, parts) => {
        setUserContext((prev) => reduceArtifactsFromToolParts(prev, parts));
      },
    },
  );

  const allItems = useMemo(() => {
    return [...items, ...nextItems];
  }, [items, nextItems]);

  const [speechStarted, setSpeechStarted] = useState(false);

  const handleSpeechRealStart = useCallback(() => {
    if (agentIsStreaming) {
      abortAgent();
    }
    if (isPlaying) {
      void interruptPlayback();
    }
    setSpeechStarted(true);
  }, [abortAgent, agentIsStreaming, isPlaying]);

  const handleSpeechEnd = useCallback(
    (audio: Float32Array) => {
      setSpeechStarted(false);
      if (audio.length === 0) {
        return;
      }
      const copied = audio.slice();
      const wavBuffer = vadUtils.encodeWAV(
        copied,
        3,
        CAPTURE_SAMPLE_RATE,
        CAPTURE_CHANNEL_COUNT,
      );
      const base64 = vadUtils.arrayBufferToBase64(wavBuffer);
      const audioPart: AudioPart = {
        type: "audio",
        format: "wav",
        audio_data: base64,
        sample_rate: CAPTURE_SAMPLE_RATE,
        channels: CAPTURE_CHANNEL_COUNT,
      };
      setTurnCount((prev) => prev + 1);
      void sendUserMessage({ parts: [audioPart] }).catch((err: unknown) => {
        console.error("Failed to send audio turn", err);
      });
    },
    [sendUserMessage],
  );

  const vad = useMicVAD({
    model: "v5",
    startOnLoad: false,
    baseAssetPath: VAD_MODEL_BASE_URL,
    onnxWASMBasePath: ORT_WASM_BASE_URL,
    workletOptions: {
      processorOptions: {
        channelCount: CAPTURE_CHANNEL_COUNT,
      },
    },
    onSpeechRealStart: handleSpeechRealStart,
    onSpeechEnd: handleSpeechEnd,
    getStream: () =>
      navigator.mediaDevices.getUserMedia({
        audio: {
          sampleRate: CAPTURE_SAMPLE_RATE,
          channelCount: CAPTURE_CHANNEL_COUNT,
          echoCancellation: true,
          autoGainControl: true,
          noiseSuppression: true,
        },
        video: false,
      }),
    // positiveSpeechThreshold: 0.3,
    // negativeSpeechThreshold: 0.25,
    // redemptionMs: 4000,
    // preSpeechPadMs: 800,
    // minSpeechMs: 1500,
  }) as MicVadState;

  const { listening, loading, errored, toggle } = vad;

  const sessionPhase = useMemo<SessionPhase>(() => {
    if (agentIsStreaming || isPlaying) {
      if (speechStarted) {
        return "capturing";
      }
      return "responding";
    }
    if (speechStarted) {
      return "capturing";
    }
    if (listening) {
      return "listening";
    }
    return "idle";
  }, [agentIsStreaming, listening, speechStarted, isPlaying]);

  useEffect(() => {
    let animationFrame = 0;
    const animate = () => {
      const isActive =
        sessionPhase === "capturing" || sessionPhase === "responding";
      const jitter = isActive ? 0.35 + Math.random() * 0.65 : 0;
      const target = Math.min(0.95, jitter);
      setVisualVolume(target);
      animationFrame = requestAnimationFrame(animate);
    };
    animationFrame = requestAnimationFrame(animate);
    return () => {
      cancelAnimationFrame(animationFrame);
    };
  }, [sessionPhase]);

  const currentSpeaker: SpeakerState =
    sessionPhase === "responding"
      ? "assistant"
      : sessionPhase === "capturing" ||
          sessionPhase === "listening" ||
          sessionPhase === "processing"
        ? "user"
        : "idle";

  const sessionTheme = useMemo(
    () => SESSION_THEMES[sessionPhase],
    [sessionPhase],
  );
  const statusAccent = loading
    ? "text-slate-400"
    : errored || agentError
      ? "text-rose-500"
      : sessionTheme.accent;
  const statusLabel = loading
    ? "Loading VAD"
    : errored
      ? "Mic Error"
      : agentError
        ? "Agent Error"
        : sessionTheme.label;

  const statusAnnouncement = useMemo(() => {
    if (loading) {
      return "Voice activity detector is initializing.";
    }
    if (errored) {
      return `Microphone error: ${errored}`;
    }
    if (agentError) {
      return `Agent error: ${agentError}`;
    }
    if (turnCount > 0) {
      const plural = turnCount === 1 ? "" : "s";
      const countText = String(turnCount);
      return `${sessionTheme.description} · ${countText} buffered turn${plural}.`;
    }
    return sessionTheme.description;
  }, [agentError, errored, loading, sessionTheme, turnCount]);

  const handleMicrophoneToggle = useCallback(() => {
    if (loading) {
      return;
    }
    toggle();
  }, [loading, toggle]);

  return (
    <div className="flex h-screen bg-gradient-to-br from-slate-50 via-white to-rose-50 font-mono text-slate-900">
      <section className="flex min-w-0 flex-1 items-center justify-center border-slate-200/60 bg-white/60 backdrop-blur">
        <div
          className="flex flex-col items-center gap-10 px-6 text-center"
          aria-live="polite"
          aria-label={statusAnnouncement}
        >
          <VolumeVisualizer
            speaker={currentSpeaker}
            volume={visualVolume}
            microphoneActive={listening}
          />
          <div className="flex flex-col items-center gap-3">
            <MicrophoneButton
              active={listening}
              onClick={handleMicrophoneToggle}
            />
            <p
              className={`text-[11px] tracking-[0.3em] uppercase ${statusAccent}`}
            >
              {statusLabel}
            </p>
            {turnCount > 0 ? (
              <p className="text-[10px] tracking-[0.25em] text-slate-300 uppercase">
                {turnCount} buffered turn{turnCount === 1 ? "" : "s"}
              </p>
            ) : null}
            {errored ? (
              <p className="text-xs text-rose-500">{errored}</p>
            ) : null}
            {agentError ? (
              <p className="text-xs text-rose-500">{agentError}</p>
            ) : null}
            {!errored && loading ? (
              <p className="text-xs text-slate-500">
                Loading voice activity detector…
              </p>
            ) : null}
          </div>
        </div>
      </section>
      <section className="flex min-h-0 min-w-0 flex-1 flex-col overflow-x-hidden overflow-y-auto">
        <ChatPane items={allItems} streamingParts={streamingParts} />
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
  );
}

function VolumeVisualizer({
  speaker,
  volume,
  microphoneActive,
}: {
  speaker: SpeakerState;
  volume: number;
  microphoneActive: boolean;
}) {
  const intensity = Math.max(0.4, Math.min(volume, 1));
  const baseCircle =
    speaker === "assistant"
      ? "bg-sky-50/70 border-sky-200"
      : speaker === "user"
        ? "bg-rose-50/80 border-rose-200"
        : "bg-white/70 border-slate-200/70";
  const aura =
    speaker === "assistant"
      ? "bg-sky-100/40"
      : speaker === "user"
        ? "bg-rose-100/40"
        : "bg-slate-100/30";
  const iconColor =
    speaker === "assistant"
      ? "text-sky-500"
      : speaker === "user"
        ? "text-rose-500"
        : microphoneActive
          ? "text-rose-500"
          : "text-slate-500";
  const scalePrimary = (0.5 + intensity * 0.6).toFixed(3);
  const scaleSecondary = (0.5 + intensity * 1).toFixed(3);

  return (
    <div className="relative flex h-44 w-44 items-center justify-center">
      <div
        className={`absolute h-40 w-40 rounded-full ${aura} blur-3xl transition-all duration-300`}
        style={{ opacity: 0.35 + intensity * 0.35 }}
      />
      <div
        className={`absolute h-40 w-40 rounded-full border ${baseCircle} transition-all duration-300`}
        style={{ transform: `scale(${scalePrimary})` }}
      />
      <div
        className={`absolute h-48 w-48 rounded-full border ${baseCircle} opacity-50 transition-all duration-500`}
        style={{
          transform: `scale(${scaleSecondary})`,
          opacity: 0.18 + intensity * 0.4,
        }}
      />
      <div className="relative flex h-20 w-20 items-center justify-center rounded-full border border-slate-200/60 bg-white/80">
        <MicIcon className={`h-7 w-7 ${iconColor}`} />
      </div>
    </div>
  );
}

function MicrophoneButton({
  active,
  onClick,
}: {
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`group inline-flex items-center gap-4 rounded-full border px-5 py-3 text-sm font-semibold transition ${
        active
          ? "border-rose-300 bg-rose-100/80 text-rose-700 hover:bg-rose-100"
          : "border-slate-200 bg-white/80 text-slate-700 hover:border-slate-300 hover:bg-white"
      }`}
    >
      <span
        className={`flex h-10 w-10 items-center justify-center rounded-full border text-base transition ${
          active
            ? "border-rose-300 bg-white/80 text-rose-500"
            : "border-slate-200 bg-white/80 text-slate-500 group-hover:border-slate-300 group-hover:text-slate-700"
        }`}
      >
        {active ? (
          <StopIcon className="h-4 w-4" />
        ) : (
          <MicIcon className="h-5 w-5" />
        )}
      </span>
      <div className="flex flex-col text-left">
        <span
          className={`text-[10px] tracking-[0.3em] uppercase ${
            active ? "text-rose-500" : "text-slate-400"
          }`}
        >
          {active ? "mic live" : "mic muted"}
        </span>
        <span>{active ? "Mute microphone" : "Unmute microphone"}</span>
      </div>
    </button>
  );
}

function MicIcon({ className }: { className?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="currentColor"
      className={className}
    >
      <path d="M12 14a3 3 0 0 0 3-3V7a3 3 0 1 0-6 0v4a3 3 0 0 0 3 3Zm5-3a5 5 0 1 1-10 0H5a7 7 0 0 0 6 6.93V21h2v-3.07A7 7 0 0 0 19 11h-2Z" />
    </svg>
  );
}

function StopIcon({ className }: { className?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="currentColor"
      className={className}
    >
      <rect x="7" y="7" width="10" height="10" rx="2" />
    </svg>
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
