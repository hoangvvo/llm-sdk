import type { AudioPart } from "@hoangvvo/llm-sdk";
import { useMicVAD, utils as vadUtils } from "@ricky0123/vad-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { ChatPane } from "./components/chat-pane.tsx";
import { ResponsiveSidebar } from "./components/sidebar.tsx";
import { reduceArtifactsFromToolParts } from "./lib/artifacts.ts";
import { useAgent } from "./lib/use-agent.ts";
import { useAudio } from "./lib/use-audio.ts";
import { useConsoleAppState } from "./lib/use-console-app-state.ts";
import { base64ToArrayBuffer } from "./lib/utils.ts";
import type { MyContext } from "./types.ts";

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

const REALTIME_HISTORY_STORAGE_KEY = "console-realtime-history";

export function RealtimeApp() {
  const [visualVolume, setVisualVolume] = useState(0.12);
  const [turnCount, setTurnCount] = useState(0);
  const {
    serverOptions,
    serverUrl,
    handleServerUrlChange,
    runStreamUrl,
    modelOptions,
    modelSelection,
    setModelSelection,
    selectedModelOption,
    providerApiKeys,
    handleSaveProviderApiKey,
    toolOptions,
    toolsError,
    toolsInitialized,
    enabledTools,
    handleEnabledToolsChange,
    mcpServers,
    handleMcpServersChange,
    agentBehavior,
    setAgentBehavior,
    userContext,
    setUserContext,
    modelAudio,
    setModelAudio,
    modelReasoning,
    setModelReasoning,
    modelModalities,
    setModelModalities,
  } = useConsoleAppState<MyContext>();

  const { add16BitPCM, interruptPlayback, isPlaying } = useAudio();

  const hasServerOptions = serverOptions.length > 0;

  const {
    isStreaming: agentIsStreaming,
    error: agentError,
    setError: setAgentError,
    sendUserMessage,
    abort: abortAgent,
    items,
    nextItems,
    streamingParts,
    resetConversation,
  } = useAgent<MyContext>(
    {
      runStreamUrl,
      modelSelection,
      model: selectedModelOption ?? null,
      providerApiKeys,
      userContext,
      enabledTools,
      mcpServers,
      agentBehavior,
      toolsInitialized,
      audio: modelAudio,
      reasoning: modelReasoning,
      modalities: modelModalities,
      historyStorageKey: REALTIME_HISTORY_STORAGE_KEY,
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
  }, [abortAgent, agentIsStreaming, isPlaying, interruptPlayback]);

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

  const handleNewChat = useCallback(() => {
    resetConversation();
    setAgentError(null);
    setTurnCount(0);
    setSpeechStarted(false);
  }, [resetConversation, setAgentError]);

  return (
    <div className="flex h-full w-full flex-col-reverse bg-gradient-to-br from-slate-50 via-white to-rose-50 font-mono text-slate-900 lg:flex-row">
      <section className="flex min-w-0 flex-none items-center justify-center lg:flex-1">
        <div
          className="flex flex-col items-center gap-10 px-6 py-8 text-center"
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
            {!selectedModelOption?.modalities?.includes("audio") ? (
              <p className="max-w-sm text-xs text-amber-500">
                Selected model does not support audio input. Please select a
                different model in the sidebar.
              </p>
            ) : null}
          </div>
        </div>
      </section>
      <section className="flex min-h-0 min-w-0 flex-1 flex-col border-slate-200/70 lg:border-l">
        <div className="flex items-center justify-between border-b border-slate-200/70 px-4 py-2">
          <p className="text-xs font-semibold tracking-[0.25em] text-slate-700 uppercase">
            Transcript
          </p>
          <button
            type="button"
            className="console-button"
            onClick={handleNewChat}
          >
            New Chat
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto">
          <ChatPane items={allItems} streamingParts={streamingParts} />
        </div>
      </section>
      <ResponsiveSidebar
        serverOptions={hasServerOptions ? serverOptions : undefined}
        serverUrl={hasServerOptions ? serverUrl : undefined}
        onServerUrlChange={hasServerOptions ? handleServerUrlChange : undefined}
        models={modelOptions}
        selection={modelSelection}
        onModelSelectionChange={setModelSelection}
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
        mcpServers={mcpServers}
        onMcpServersChange={handleMcpServersChange}
        toolsInitialized={toolsInitialized}
        modelAudio={modelAudio}
        onModelAudioChange={setModelAudio}
        modelReasoning={modelReasoning}
        onModelReasoningChange={setModelReasoning}
        modelModalities={modelModalities}
        onModelModalitiesChange={setModelModalities}
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
