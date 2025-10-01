import type { AudioPartDelta, Part } from "@hoangvvo/llm-sdk";
import { useCallback, useMemo, useState } from "react";
import { ArtifactsPane } from "./components/artifacts-pane.tsx";
import { ChatPane } from "./components/chat-pane.tsx";
import { Composer } from "./components/composer.tsx";
import { EventsPane } from "./components/events-pane.tsx";
import { ResponsiveSidebar } from "./components/sidebar.tsx";
import { reduceArtifactsFromToolParts } from "./lib/artifacts.ts";
import { useAgent } from "./lib/use-agent.ts";
import { useAudio } from "./lib/use-audio.ts";
import { useConsoleAppState } from "./lib/use-console-app-state.ts";
import { base64ToArrayBuffer } from "./lib/utils.ts";
import type { MyContext } from "./types.ts";

const CHAT_HISTORY_STORAGE_KEY = "console-chat-history";

export function ChatApp() {
  const [activeTab, setActiveTab] = useState<"chat" | "events">("chat");
  const [isArtifactsOpen, setIsArtifactsOpen] = useState(false);

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

  const { add16BitPCM } = useAudio();

  const handleAudioDelta = useCallback(
    (delta: AudioPartDelta) => {
      if (!delta.data || delta.data.length === 0) {
        return;
      }
      const buffer = base64ToArrayBuffer(delta.data);
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
      historyStorageKey: CHAT_HISTORY_STORAGE_KEY,
    },
    {
      onAudioDelta: handleAudioDelta,
      onToolResult: (_toolName, parts) => {
        setUserContext((prev) => reduceArtifactsFromToolParts(prev, parts));
      },
    },
  );

  const allItems = useMemo(() => [...items, ...nextItems], [items, nextItems]);

  const handleSendMessage = useCallback(
    async (parts: Part[]) => {
      await sendUserMessage({ parts });
    },
    [sendUserMessage],
  );

  const handleArtifactDelete = useCallback(
    (id: string) => {
      setUserContext((prev) => ({
        ...prev,
        artifacts: (prev.artifacts ?? []).filter(
          (artifact) => artifact.id !== id,
        ),
      }));
    },
    [setUserContext],
  );

  const hasServerOptions = serverOptions.length > 0;
  const artifacts = userContext.artifacts ?? [];

  const handleNewChat = useCallback(() => {
    resetConversation();
    setError(null);
  }, [resetConversation, setError]);

  return (
    <div className="relative h-full w-full bg-gradient-to-br from-slate-50 via-white to-rose-50 font-mono text-slate-900">
      <div className="flex h-full flex-col lg:flex-row">
        <section className="flex min-h-0 min-w-0 flex-1 flex-col border-slate-200/70 bg-white/75 backdrop-blur">
          <header className="flex items-center justify-between border-b border-slate-200/70 px-4 py-2">
            <h1 className="text-md truncate tracking-tight text-slate-900">
              Chat Console
            </h1>
            <div className="flex items-center gap-3">
              <div className="flex items-center gap-2 text-xs tracking-[0.25em] uppercase">
                <TabButton
                  isActive={activeTab === "chat"}
                  onClick={() => {
                    setActiveTab("chat");
                  }}
                >
                  UI
                </TabButton>
                <TabButton
                  isActive={activeTab === "events"}
                  onClick={() => {
                    setActiveTab("events");
                  }}
                >
                  Events
                </TabButton>
              </div>
              <button
                type="button"
                className="console-button truncate"
                onClick={handleNewChat}
              >
                New Chat
              </button>
            </div>
          </header>
          {activeTab === "chat" ? (
            <div className="grid min-h-0 flex-1 grid-cols-1 grid-rows-1 gap-3 p-2 lg:grid-cols-3">
              <div className="col-span-1 flex h-full min-h-0 min-w-0 lg:col-span-2">
                <ChatPane items={allItems} streamingParts={streamingParts} />
              </div>
              <div className="col-span-1 hidden h-full min-w-0 flex-col lg:flex">
                <ArtifactsPane
                  artifacts={userContext.artifacts}
                  onDelete={handleArtifactDelete}
                />
              </div>
            </div>
          ) : (
            <EventsPane events={eventLog} />
          )}
          {error ? (
            <div className="bg-rose-500 py-1 text-center text-xs text-white">
              {error}
            </div>
          ) : null}
          {activeTab === "chat" ? (
            <div className="border-t border-slate-200/70 bg-white/80 px-4 py-3 backdrop-blur-sm lg:hidden">
              <button
                type="button"
                className="flex w-full items-center justify-between text-[11px] font-semibold tracking-[0.3em] text-slate-700 uppercase"
                onClick={() => {
                  setIsArtifactsOpen(true);
                }}
              >
                <span>Artifacts</span>
                <span className="rounded-full border border-slate-300 px-2 py-0.5 text-[10px] tracking-[0.25em] text-slate-500">
                  {artifacts.length}
                </span>
              </button>
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
        <ResponsiveSidebar
          serverOptions={hasServerOptions ? serverOptions : undefined}
          serverUrl={hasServerOptions ? serverUrl : undefined}
          onServerUrlChange={
            hasServerOptions ? handleServerUrlChange : undefined
          }
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
      {isArtifactsOpen ? (
        <div className="fixed inset-0 z-50 flex flex-col bg-white/95 backdrop-blur lg:hidden">
          <div className="flex items-center justify-between border-b border-slate-200/70 px-6 py-4">
            <p className="text-xs font-semibold tracking-[0.3em] text-slate-700 uppercase">
              Artifacts
            </p>
            <button
              type="button"
              className="rounded-full border border-slate-300 px-3 py-1 text-[11px] font-semibold tracking-[0.2em] text-slate-700 uppercase transition hover:border-slate-400 hover:text-slate-900"
              onClick={() => {
                setIsArtifactsOpen(false);
              }}
            >
              Close
            </button>
          </div>
          <div className="flex flex-1 flex-col overflow-y-auto">
            <ArtifactsPane
              artifacts={userContext.artifacts}
              onDelete={handleArtifactDelete}
            />
          </div>
        </div>
      ) : null}
    </div>
  );
}

function TabButton({
  isActive,
  onClick,
  children,
}: {
  isActive: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      className={`flex-1 rounded px-3 py-1 text-sm font-medium transition ${
        isActive
          ? "bg-slate-800 text-white"
          : "border border-slate-200 text-slate-600 hover:bg-slate-100 hover:text-slate-900"
      }`}
      onClick={onClick}
    >
      {children}
    </button>
  );
}
