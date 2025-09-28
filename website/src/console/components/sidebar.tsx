import type {
  AudioOptions,
  LanguageModelMetadata,
  Modality,
  ReasoningOptions,
} from "@hoangvvo/llm-sdk";
import type { ChangeEvent, Dispatch, SetStateAction } from "react";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import type {
  ModelOption,
  ModelSelection,
} from "../lib/use-console-app-state.ts";
import type {
  AgentBehaviorSettings,
  ApiKeys,
  McpServerConfig,
  MyContext,
  ToolInfo,
} from "../types";

const MODALITY_OPTIONS: Modality[] = ["text", "image", "audio"];

interface SidebarProps {
  serverOptions?: string[];
  serverUrl?: string;
  onServerUrlChange?: (value: string) => void;
  models: ModelOption[];
  selection: ModelSelection | null;
  onModelSelectionChange: Dispatch<SetStateAction<ModelSelection | null>>;
  apiKeys: ApiKeys;
  onSaveApiKey: (provider: string, apiKey: string) => void;
  context: MyContext;
  onContextChange: Dispatch<SetStateAction<MyContext>>;
  behavior: AgentBehaviorSettings;
  onBehaviorChange: Dispatch<SetStateAction<AgentBehaviorSettings>>;
  tools: ToolInfo[];
  enabledTools: string[];
  onEnabledToolsChange: (tools: string[]) => void;
  toolErrorMessage?: string | null;
  mcpServers: McpServerConfig[];
  onMcpServersChange: (servers: McpServerConfig[]) => void;
  toolsInitialized: boolean;
  modelAudio: AudioOptions | undefined;
  onModelAudioChange: (audio: AudioOptions | undefined) => void;
  modelReasoning: ReasoningOptions | undefined;
  onModelReasoningChange: (reasoning: ReasoningOptions | undefined) => void;
  modelModalities: Modality[] | undefined;
  onModelModalitiesChange: (modalities: Modality[] | undefined) => void;
}

export function ResponsiveSidebar(props: SidebarProps) {
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);

  return (
    <>
      <div className="hidden lg:block">
        <Sidebar {...props} />
      </div>
      <button
        type="button"
        aria-label="Open settings"
        className="fixed right-0 bottom-1/2 z-20 flex items-center justify-center rounded-l-full border border-r-0 border-slate-300 bg-white/80 px-3 py-2 text-[11px] font-semibold tracking-[0.3em] text-slate-900 uppercase backdrop-blur transition hover:bg-white/80 lg:hidden"
        onClick={() => {
          setIsSidebarOpen(true);
        }}
      >
        Menu
      </button>
      {isSidebarOpen ? (
        <div className="fixed inset-x-0 top-0 z-40 flex h-full flex-col overflow-hidden lg:hidden">
          <div className="flex items-center justify-between bg-white px-6 py-4">
            <p className="text-xs font-semibold tracking-[0.3em] text-slate-700 uppercase">
              Console Settings
            </p>
            <button
              type="button"
              className="rounded-full border border-slate-300 px-3 py-1 text-[11px] font-semibold tracking-[0.2em] text-slate-700 uppercase transition hover:border-slate-400 hover:text-slate-900"
              onClick={() => {
                setIsSidebarOpen(false);
              }}
            >
              Close
            </button>
          </div>
          <div className="flex-1 overflow-y-auto border-t border-slate-200/70">
            <Sidebar {...props} />
          </div>
        </div>
      ) : null}
    </>
  );
}

function Sidebar({
  serverOptions,
  serverUrl,
  onServerUrlChange,
  models,
  selection,
  onModelSelectionChange,
  apiKeys,
  onSaveApiKey,
  context,
  onContextChange,
  behavior,
  onBehaviorChange,
  tools,
  enabledTools,
  onEnabledToolsChange,
  toolErrorMessage,
  mcpServers,
  onMcpServersChange,
  toolsInitialized,
  modelAudio,
  onModelAudioChange,
  modelReasoning,
  onModelReasoningChange,
  modelModalities,
  onModelModalitiesChange,
}: SidebarProps) {
  const selectedModel = useMemo(() => {
    if (!selection) {
      return null;
    }
    return (
      models.find(
        (item) =>
          item.provider === selection.provider &&
          item.modelId === selection.modelId,
      ) ?? null
    );
  }, [models, selection]);
  const [toolsOpen, setToolsOpen] = useState(false);
  const [mcpOpen, setMcpOpen] = useState(false);
  const [behaviorOpen, setBehaviorOpen] = useState(false);
  const [contextOpen, setContextOpen] = useState(false);
  const [featuresOpen, setFeaturesOpen] = useState(false);

  return (
    <aside className="flex h-full w-full flex-col overflow-auto bg-white/60 px-6 py-6 backdrop-blur-sm lg:w-[360px] lg:shrink-0 lg:border-l lg:border-slate-200/70">
      <div className="space-y-6">
        <ServerSelectionSection
          options={serverOptions}
          value={serverUrl}
          onChange={onServerUrlChange}
        />
        <ModelSelectionSection
          models={models}
          selection={selection}
          onChange={onModelSelectionChange}
          apiKeys={apiKeys}
          onSaveApiKey={onSaveApiKey}
        />
        <CollapsibleSection
          title="Model Features"
          isOpen={featuresOpen}
          onToggle={() => {
            setFeaturesOpen((prev) => !prev);
          }}
        >
          <ModelFeaturesSection
            selectedModel={selectedModel}
            audio={modelAudio}
            onAudioChange={onModelAudioChange}
            reasoning={modelReasoning}
            onReasoningChange={onModelReasoningChange}
            modalities={modelModalities}
            onModalitiesChange={onModelModalitiesChange}
          />
        </CollapsibleSection>
        <CollapsibleSection
          title={`Tools (${tools.length})`}
          isOpen={toolsOpen}
          onToggle={() => {
            setToolsOpen((prev) => !prev);
          }}
        >
          <ToolSection
            tools={tools}
            enabledTools={enabledTools}
            onEnabledToolsChange={onEnabledToolsChange}
            errorMessage={toolErrorMessage}
            initialized={toolsInitialized}
          />
        </CollapsibleSection>
        <CollapsibleSection
          title={`MCP Servers (${mcpServers.length})`}
          isOpen={mcpOpen}
          onToggle={() => {
            setMcpOpen((prev) => !prev);
          }}
        >
          <McpServerSection
            servers={mcpServers}
            onChange={onMcpServersChange}
          />
        </CollapsibleSection>
        <CollapsibleSection
          title="Sampling Parameters"
          isOpen={behaviorOpen}
          onToggle={() => {
            setBehaviorOpen((prev) => !prev);
          }}
        >
          <AgentBehaviorSection
            behavior={behavior}
            onChange={onBehaviorChange}
          />
        </CollapsibleSection>
        <CollapsibleSection
          title="Context"
          isOpen={contextOpen}
          onToggle={() => {
            setContextOpen((prev) => !prev);
          }}
        >
          <ContextSection context={context} onChange={onContextChange} />
        </CollapsibleSection>
      </div>
    </aside>
  );
}

interface CollapsibleSectionProps {
  title: string;
  isOpen: boolean;
  onToggle: () => void;
  children: ReactNode;
}

function CollapsibleSection({
  title,
  isOpen,
  onToggle,
  children,
}: CollapsibleSectionProps) {
  return (
    <div className="space-y-3">
      <button
        type="button"
        className={`flex w-full items-center justify-between rounded-md border border-slate-300 bg-white px-4 py-2 text-left text-xs font-semibold tracking-[0.2em] uppercase transition focus:ring-2 focus:ring-slate-500 focus:outline-none ${
          isOpen
            ? "text-slate-900 shadow-sm"
            : "text-slate-600 hover:text-slate-900"
        }`}
        onClick={onToggle}
        aria-expanded={isOpen}
      >
        <span>{title}</span>
        <span
          className={`text-base leading-none ${
            isOpen ? "text-slate-900" : "text-slate-400"
          }`}
        >
          {isOpen ? "▾" : "▸"}
        </span>
      </button>
      {isOpen ? children : null}
    </div>
  );
}

interface ModelFeaturesSectionProps {
  selectedModel: ModelOption | null;
  audio: AudioOptions | undefined;
  onAudioChange: (audio: AudioOptions | undefined) => void;
  reasoning: ReasoningOptions | undefined;
  onReasoningChange: (reasoning: ReasoningOptions | undefined) => void;
  modalities: Modality[] | undefined;
  onModalitiesChange: (modalities: Modality[] | undefined) => void;
}

function ModelFeaturesSection({
  selectedModel,
  audio,
  onAudioChange,
  reasoning,
  onReasoningChange,
  modalities,
  onModalitiesChange,
}: ModelFeaturesSectionProps) {
  const audioEnabled = audio !== undefined;
  const reasoningEnabled = reasoning?.enabled ?? false;
  const currentModalities = modalities ?? selectedModel?.modalities ?? [];

  const handleAudioToggle = (enabled: boolean) => {
    if (enabled) {
      onAudioChange(audio ?? selectedModel?.audio ?? { format: "linear16" });
    } else {
      onAudioChange(undefined);
    }
  };

  const updateAudioField = (field: keyof AudioOptions, value: string) => {
    const base = audio ?? selectedModel?.audio ?? { format: "" };
    const next = { ...base, [field]: value } as AudioOptions;
    onAudioChange(next);
  };

  const handleReasoningToggle = (enabled: boolean) => {
    if (enabled) {
      const base = reasoning ?? selectedModel?.reasoning ?? { enabled: true };
      onReasoningChange({ ...base, enabled: true });
    } else {
      onReasoningChange(undefined);
    }
  };

  const handleReasoningBudgetChange = (value: string) => {
    if (value.trim() === "") {
      if (!reasoning) {
        return;
      }
      onReasoningChange({ ...reasoning, budget_tokens: undefined });
      return;
    }
    const parsed = Number(value);
    if (Number.isNaN(parsed)) {
      return;
    }
    onReasoningChange({
      ...(reasoning ?? { enabled: true }),
      budget_tokens: parsed,
    });
  };

  const handleModalitiesToggle = (modality: Modality, enabled: boolean) => {
    const nextSet = new Set(currentModalities);
    if (enabled) {
      nextSet.add(modality);
    } else {
      nextSet.delete(modality);
    }
    const ordered = MODALITY_OPTIONS.filter((item) =>
      nextSet.has(item),
    ) as Modality[];
    onModalitiesChange(ordered);
  };

  const handleResetDefaults = () => {
    onAudioChange(selectedModel?.audio ?? undefined);
    onReasoningChange(selectedModel?.reasoning ?? undefined);
    onModalitiesChange(selectedModel?.modalities ?? undefined);
  };

  return (
    <div className="console-surface space-y-4">
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <label className="console-label flex! items-center gap-2 text-xs">
            <input
              type="checkbox"
              className="h-4 w-4 rounded border-slate-300 text-slate-900 focus:ring-2 focus:ring-slate-500"
              checked={audioEnabled}
              onChange={(event) => {
                handleAudioToggle(event.target.checked);
              }}
            />
            Enable audio output
          </label>
        </div>
        <p className="text-[11px] text-slate-500">
          Configure audio generation parameters for supported models.
        </p>
        <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
          <label className="console-label text-[11px]">
            Audio format
            <input
              type="text"
              className="console-field mt-1 w-full"
              value={audio?.format ?? ""}
              onChange={(event) => {
                updateAudioField("format", event.target.value);
              }}
              disabled={!audioEnabled}
              placeholder="e.g. linear16"
            />
          </label>
          <label className="console-label text-[11px]">
            Voice
            <input
              type="text"
              className="console-field mt-1 w-full"
              value={(audio as { voice?: string })?.voice ?? ""}
              onChange={(event) => {
                updateAudioField("voice", event.target.value);
              }}
              disabled={!audioEnabled}
              placeholder="e.g. alloy"
            />
          </label>
        </div>
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <label className="console-label flex! items-center gap-2 text-xs">
            <input
              type="checkbox"
              className="h-4 w-4 rounded border-slate-300 text-slate-900 focus:ring-2 focus:ring-slate-500"
              checked={reasoningEnabled}
              onChange={(event) => {
                handleReasoningToggle(event.target.checked);
              }}
            />
            Enable reasoning
          </label>
        </div>
        <p className="text-[11px] text-slate-500">
          Enable reasoning tokens on reasoning models.
        </p>
        <label className="console-label text-[11px]">
          Budget tokens
          <input
            type="number"
            className="console-field mt-1 w-full"
            value={
              (reasoning as { budget_tokens?: number })?.budget_tokens ?? ""
            }
            onChange={(event) => {
              handleReasoningBudgetChange(event.target.value);
            }}
            disabled={!reasoningEnabled}
            min="0"
            step="1"
          />
        </label>
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <span className="console-label text-xs">Modalities</span>
        </div>
        <p className="text-[11px] text-slate-500">
          Select which content modalities the model may generate.
        </p>
        <div className="space-y-1">
          {MODALITY_OPTIONS.map((modality) => {
            const checked = currentModalities.includes(modality);
            return (
              <label
                key={modality}
                className="flex items-center gap-2 text-[11px] text-slate-600"
              >
                <input
                  type="checkbox"
                  className="h-4 w-4 rounded border-slate-300 text-slate-900 focus:ring-2 focus:ring-slate-500"
                  checked={checked}
                  onChange={(event) => {
                    handleModalitiesToggle(modality, event.target.checked);
                  }}
                />
                {modality}
              </label>
            );
          })}
        </div>
      </div>

      <button
        type="button"
        className="console-button console-button-quiet w-full text-[11px]"
        onClick={handleResetDefaults}
        disabled={!selectedModel}
      >
        Reset to defaults
      </button>
    </div>
  );
}

interface ServerSelectionSectionProps {
  options?: string[];
  value?: string;
  onChange?: (value: string) => void;
}

function ServerSelectionSection({
  options,
  value,
  onChange,
}: ServerSelectionSectionProps) {
  if (!options || options.length === 0 || !value || !onChange) {
    return null;
  }
  return (
    <div>
      <h2 className="console-section-title">Example Server</h2>
      <p className="mt-2 text-xs text-slate-500">
        Select which example server to use when running the console.
      </p>
      <select
        className="console-field mt-3 w-full"
        value={value}
        onChange={(event) => {
          onChange(event.target.value);
        }}
      >
        {options.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    </div>
  );
}

interface ModelSelectionSectionProps {
  models: ModelOption[];
  selection: ModelSelection | null;
  onChange: Dispatch<SetStateAction<ModelSelection | null>>;
  apiKeys: ApiKeys;
  onSaveApiKey: (provider: string, apiKey: string) => void;
}

function ModelSelectionSection({
  models,
  selection,
  onChange,
  apiKeys,
  onSaveApiKey,
}: ModelSelectionSectionProps) {
  const selected = selection
    ? models.find(
        (item) =>
          item.provider === selection.provider &&
          item.modelId === selection.modelId,
      )
    : null;
  const providers = useMemo(
    () => Array.from(new Set(models.map((item) => item.provider))).sort(),
    [models],
  );

  const [showApiKeyManager, setShowApiKeyManager] = useState(false);
  const [apiKeyDrafts, setApiKeyDrafts] = useState<ApiKeys>({});

  useEffect(() => {
    if (!showApiKeyManager) {
      return;
    }
    setApiKeyDrafts((prev) => {
      const next: Record<string, string> = {};
      for (const provider of providers) {
        next[provider] = prev[provider] ?? apiKeys[provider] ?? "";
      }
      return next;
    });
  }, [apiKeys, providers, showApiKeyManager]);

  const handleToggleApiKeyManager = () => {
    if (!showApiKeyManager) {
      const drafts: Record<string, string> = {};
      for (const provider of providers) {
        drafts[provider] = apiKeys[provider] ?? "";
      }
      setApiKeyDrafts(drafts);
      setShowApiKeyManager(true);
      return;
    }
    setShowApiKeyManager(false);
  };

  const handleDraftChange = (provider: string, value: string) => {
    setApiKeyDrafts((prev) => ({ ...prev, [provider]: value }));
  };

  const handleSave = (provider: string) => {
    const value = (apiKeyDrafts[provider] ?? "").trim();
    onSaveApiKey(provider, value);
    setApiKeyDrafts((prev) => ({ ...prev, [provider]: value }));
  };

  const handleClear = (provider: string) => {
    onSaveApiKey(provider, "");
    setApiKeyDrafts((prev) => ({ ...prev, [provider]: "" }));
  };

  return (
    <div className="space-y-4">
      <div>
        <h2 className="console-section-title">Model</h2>
        <p className="mt-2 text-xs text-slate-500">
          Choose which model the agent should use.
        </p>
        {models.length > 0 ? (
          <div className="mt-3 flex gap-2">
            <select
              className="console-field min-w-0 flex-1"
              value={selected ? `${selected.provider}:${selected.modelId}` : ""}
              onChange={(event) => {
                const [provider, modelId] = event.target.value.split(":");
                onChange({ provider, modelId });
              }}
            >
              <option value="" disabled>
                Select a model
              </option>
              {models.map((option) => (
                <option
                  key={`${option.provider}:${option.modelId}`}
                  value={`${option.provider}:${option.modelId}`}
                >
                  {option.label}
                </option>
              ))}
            </select>
            <button
              type="button"
              className="console-button"
              onClick={handleToggleApiKeyManager}
            >
              {showApiKeyManager ? "Close" : "API Keys"}
            </button>
          </div>
        ) : (
          <div className="console-surface mt-3 text-sm text-slate-500">
            Loading models…
          </div>
        )}
      </div>
      {showApiKeyManager ? (
        <div className="console-surface text-xs text-slate-600">
          <h3 className="console-subheading">Provider API Keys</h3>
          {providers.length > 0 ? (
            <div className="mt-3 space-y-3">
              {providers.map((provider) => {
                const savedValue = apiKeys[provider];
                const draftValue = apiKeyDrafts[provider] ?? "";
                return (
                  <div key={provider} className="console-surface p-3">
                    <div className="console-microcaps flex items-center justify-between text-slate-500">
                      <span>{formatProviderLabel(provider)}</span>
                      <span
                        className={`text-[10px] ${
                          savedValue ? "text-emerald-600" : "text-slate-400"
                        }`}
                      >
                        {savedValue ? "Saved" : "Not set"}
                      </span>
                    </div>
                    <input
                      type="text"
                      className="console-field mt-2 w-full"
                      placeholder={`Enter your ${formatProviderLabel(
                        provider,
                      )} API key`}
                      value={draftValue}
                      onChange={(event) => {
                        handleDraftChange(provider, event.target.value);
                      }}
                    />
                    <div className="mt-2 flex items-center gap-2">
                      <button
                        type="button"
                        className="console-button"
                        onClick={() => {
                          handleSave(provider);
                        }}
                      >
                        Save
                      </button>
                      <button
                        type="button"
                        className="console-button console-button-quiet"
                        onClick={() => {
                          handleClear(provider);
                        }}
                      >
                        Clear
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          ) : (
            <p className="mt-3 text-slate-400">No providers available.</p>
          )}
        </div>
      ) : null}
      {selected ? <ModelDetails option={selected} /> : null}
    </div>
  );
}

interface AgentBehaviorSectionProps {
  behavior: AgentBehaviorSettings;
  onChange: Dispatch<SetStateAction<AgentBehaviorSettings>>;
}

function AgentBehaviorSection({
  behavior,
  onChange,
}: AgentBehaviorSectionProps) {
  const handleNumberChange =
    (key: keyof AgentBehaviorSettings, parser: (value: string) => number) =>
    (event: ChangeEvent<HTMLInputElement>) => {
      const { value } = event.target;
      onChange((prev) => {
        if (value.trim() === "") {
          const { [key]: _omit, ...rest } = prev;
          void _omit;
          return rest;
        }
        const parsed = parser(value);
        if (Number.isNaN(parsed)) {
          return prev;
        }
        return { ...prev, [key]: parsed };
      });
    };

  return (
    <div className="console-surface space-y-3">
      <p className="text-xs text-slate-500">
        Tune sampling parameters for the model.
      </p>
      <div className="grid grid-cols-1 gap-3">
        <NumberField
          label="Temperature"
          placeholder="e.g. 0.7"
          value={behavior.temperature}
          step="0.1"
          min="0"
          max="2"
          onChange={handleNumberChange("temperature", (value) =>
            parseFloat(value),
          )}
        />
        <NumberField
          label="Top P"
          placeholder="e.g. 0.9"
          value={behavior.top_p}
          step="0.05"
          min="0"
          max="1"
          onChange={handleNumberChange("top_p", (value) => parseFloat(value))}
        />
        <NumberField
          label="Top K"
          placeholder="e.g. 40"
          value={behavior.top_k}
          step="1"
          min="0"
          onChange={handleNumberChange("top_k", (value) =>
            Math.max(0, Math.floor(Number(value))),
          )}
        />
        <NumberField
          label="Presence penalty"
          placeholder="e.g. 0.5"
          value={behavior.presence_penalty}
          step="0.1"
          onChange={handleNumberChange("presence_penalty", (value) =>
            parseFloat(value),
          )}
        />
        <NumberField
          label="Frequency penalty"
          placeholder="e.g. 0.5"
          value={behavior.frequency_penalty}
          step="0.1"
          onChange={handleNumberChange("frequency_penalty", (value) =>
            parseFloat(value),
          )}
        />
      </div>
    </div>
  );
}

interface ToolSectionProps {
  tools: ToolInfo[];
  enabledTools: string[];
  onEnabledToolsChange: (tools: string[]) => void;
  errorMessage?: string | null;
  initialized: boolean;
}

function ToolSection({
  tools,
  enabledTools,
  onEnabledToolsChange,
  errorMessage,
  initialized,
}: ToolSectionProps) {
  const orderedToolNames = useMemo(
    () => tools.map((tool) => tool.name),
    [tools],
  );

  const handleToggleTool = (toolName: string, isChecked: boolean) => {
    if (isChecked) {
      const nextSet = new Set([...enabledTools, toolName]);
      const normalized = orderedToolNames.filter((name) => nextSet.has(name));
      onEnabledToolsChange(normalized);
    } else {
      onEnabledToolsChange(enabledTools.filter((name) => name !== toolName));
    }
  };

  return (
    <div className="console-surface space-y-3">
      <p className="text-xs text-slate-500">
        Toggle which tools the agent can invoke during a run.
      </p>
      {tools.length > 0 ? (
        <ul className="mt-2 space-y-2 text-xs text-slate-600">
          {tools.map((tool) => {
            const checked = enabledTools.includes(tool.name);
            return (
              <li key={tool.name} className="flex items-center gap-2">
                <input
                  type="checkbox"
                  className="h-4 w-4 rounded border-slate-300 text-slate-900 focus:ring-2 focus:ring-slate-500"
                  checked={checked}
                  onChange={(event) => {
                    handleToggleTool(tool.name, event.target.checked);
                  }}
                />
                <div>
                  <p className="font-semibold text-slate-700">{tool.name}</p>
                  {tool.description ? (
                    <p className="text-[11px] leading-snug text-slate-500">
                      {tool.description}
                    </p>
                  ) : null}
                </div>
              </li>
            );
          })}
        </ul>
      ) : (
        <p className="mt-2 text-xs text-slate-500">
          {errorMessage ??
            (initialized ? "No tools available." : "Loading tools…")}
        </p>
      )}
    </div>
  );
}

interface McpServerSectionProps {
  servers: McpServerConfig[];
  onChange: (servers: McpServerConfig[]) => void;
}

function McpServerSection({ servers, onChange }: McpServerSectionProps) {
  const updateServer = (
    index: number,
    updater: (current: McpServerConfig) => McpServerConfig,
  ) => {
    onChange(
      servers.map((server, currentIndex) =>
        currentIndex === index ? updater(server) : server,
      ),
    );
  };

  const handleAddServer = () => {
    onChange([
      ...servers,
      { type: "streamable-http", url: "", authorization: "" },
    ]);
  };

  const handleRemoveServer = (index: number) => {
    onChange(servers.filter((_, currentIndex) => currentIndex !== index));
  };

  const handleTypeChange = (index: number, type: McpServerConfig["type"]) => {
    updateServer(index, (server) => {
      if (type === server.type) {
        return server;
      }
      if (type === "streamable-http") {
        return { type: "streamable-http", url: "", authorization: "" };
      }
      return { type: "stdio", command: "", args: [] };
    });
  };

  const handleStreamableHttpChange = (
    index: number,
    field: "url" | "authorization",
    value: string,
  ) => {
    updateServer(index, (server) => {
      if (server.type !== "streamable-http") {
        return server;
      }
      if (field === "url") {
        return { ...server, url: value };
      }
      return { ...server, authorization: value };
    });
  };

  const handleStdioCommandChange = (index: number, value: string) => {
    updateServer(index, (server) => {
      if (server.type !== "stdio") {
        return server;
      }
      return { ...server, command: value };
    });
  };

  const handleStdioArgsChange = (index: number, value: string) => {
    const parts = value
      .split(/\s+/)
      .map((part) => part.trim())
      .filter((part) => part.length > 0);
    updateServer(index, (server) => {
      if (server.type !== "stdio") {
        return server;
      }
      return { ...server, args: parts };
    });
  };

  return (
    <div className="console-surface space-y-3">
      <p className="text-xs text-slate-500">
        Connect Model Context Protocol servers to access additional tools.
      </p>
      {servers.length > 0 ? (
        <div className="space-y-3">
          {servers.map((server, index) => {
            const headerLabel = `Server ${String(index + 1)}`;
            return (
              <div
                key={`mcp-server-${String(index)}`}
                className="console-surface p-3"
              >
                <div className="flex items-center justify-between gap-3">
                  <span className="console-microcaps text-slate-500">
                    {headerLabel}
                  </span>
                  <button
                    type="button"
                    className="console-button console-button-quiet"
                    onClick={() => {
                      handleRemoveServer(index);
                    }}
                  >
                    Remove
                  </button>
                </div>
                <label className="console-label mt-3 block">
                  Connection type
                  <select
                    className="console-field mt-2 w-full"
                    value={server.type}
                    onChange={(event) => {
                      handleTypeChange(
                        index,
                        event.target.value as McpServerConfig["type"],
                      );
                    }}
                  >
                    <option value="streamable-http">Streamable HTTP</option>
                    <option value="stdio">Stdio</option>
                  </select>
                </label>
                {server.type === "streamable-http" ? (
                  <div className="mt-3 space-y-3">
                    <label className="console-label">
                      Server URL
                      <input
                        type="text"
                        className="console-field mt-2 w-full"
                        placeholder="e.g. https://example.com/mcp"
                        value={server.url}
                        onChange={(event) => {
                          handleStreamableHttpChange(
                            index,
                            "url",
                            event.currentTarget.value,
                          );
                        }}
                      />
                    </label>
                    <label className="console-label">
                      Authorization header (optional)
                      <input
                        type="text"
                        className="console-field mt-2 w-full"
                        placeholder="Bearer token or other header value"
                        value={server.authorization ?? ""}
                        onChange={(event) => {
                          handleStreamableHttpChange(
                            index,
                            "authorization",
                            event.currentTarget.value,
                          );
                        }}
                      />
                    </label>
                  </div>
                ) : (
                  <div className="mt-3 space-y-3">
                    <label className="console-label">
                      Command
                      <input
                        type="text"
                        className="console-field mt-2 w-full"
                        placeholder="Executable for the MCP server"
                        value={server.command}
                        onChange={(event) => {
                          handleStdioCommandChange(
                            index,
                            event.currentTarget.value,
                          );
                        }}
                      />
                    </label>
                    <label className="console-label">
                      Arguments (optional)
                      <input
                        type="text"
                        className="console-field mt-2 w-full"
                        placeholder="Space-separated arguments"
                        value={(server.args ?? []).join(" ")}
                        onChange={(event) => {
                          handleStdioArgsChange(
                            index,
                            event.currentTarget.value,
                          );
                        }}
                      />
                    </label>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      ) : (
        <p className="mt-2 text-xs text-slate-500">
          No MCP servers configured. Add one to expose remote tools.
        </p>
      )}
      <button
        type="button"
        className="console-button"
        onClick={handleAddServer}
      >
        Add MCP Server
      </button>
    </div>
  );
}

interface NumberFieldProps {
  label: string;
  value?: number;
  placeholder?: string;
  step?: string;
  min?: string;
  max?: string;
  onChange: (event: ChangeEvent<HTMLInputElement>) => void;
}

function NumberField({
  label,
  value,
  placeholder,
  step,
  min,
  max,
  onChange,
}: NumberFieldProps) {
  return (
    <label className="console-label">
      {label}
      <input
        type="number"
        className="console-field mt-2 w-full"
        value={value ?? ""}
        placeholder={placeholder}
        step={step}
        min={min}
        max={max}
        onChange={onChange}
      />
    </label>
  );
}

interface ContextSectionProps {
  context: MyContext;
  onChange: Dispatch<SetStateAction<MyContext>>;
}

function ContextSection({ context, onChange }: ContextSectionProps) {
  return (
    <div className="console-surface space-y-3">
      <p className="text-xs text-slate-500">
        Provide optional details the agent can use for its instructions and
        tools.
      </p>
      <label className="console-label">
        Name
        <input
          type="text"
          className="console-field mt-2 w-full"
          placeholder="e.g. Ada Lovelace"
          value={context.name ?? ""}
          onChange={(event) => {
            onChange((prev) => ({ ...prev, name: event.target.value }));
          }}
        />
      </label>
      <label className="console-label">
        Location
        <input
          type="text"
          className="console-field mt-2 w-full"
          placeholder="e.g. San Francisco, CA"
          value={context.location ?? ""}
          onChange={(event) => {
            onChange((prev) => ({ ...prev, location: event.target.value }));
          }}
        />
      </label>
      <label className="console-label">
        Language
        <input
          type="text"
          className="console-field mt-2 w-full"
          placeholder="e.g. English, Spanish, French"
          value={context.language ?? ""}
          onChange={(event) => {
            onChange((prev) => ({ ...prev, language: event.target.value }));
          }}
        />
      </label>
      <label className="console-label">
        Tomorrow.io API key
        <input
          type="text"
          className="console-field mt-2 w-full"
          placeholder="API key for the Tomorrow.io weather tool"
          value={context.tomorrow_api_key ?? ""}
          onChange={(event) => {
            onChange((prev) => ({
              ...prev,
              tomorrow_api_key: event.target.value,
            }));
          }}
        />
      </label>
      <label className="console-label">
        Geocode.maps.co API key
        <input
          type="text"
          className="console-field mt-2 w-full"
          placeholder="API key for the geocoding tool"
          value={context.geo_api_key ?? ""}
          onChange={(event) => {
            onChange((prev) => ({
              ...prev,
              geo_api_key: event.target.value,
            }));
          }}
        />
      </label>
      <label className="console-label">
        newsapi.org API key
        <input
          type="text"
          className="console-field mt-2 w-full"
          placeholder="API key for the news tool"
          value={context.news_api_key ?? ""}
          onChange={(event) => {
            onChange((prev) => ({
              ...prev,
              news_api_key: event.target.value,
            }));
          }}
        />
      </label>
    </div>
  );
}

function ModelDetails({ option }: { option: ModelOption }) {
  const pricing = option.metadata?.pricing;
  const capabilities = option.metadata?.capabilities ?? [];
  const pricingEntries: [string, number | null | undefined][] = pricing
    ? [
        ["Input text token", pricing.input_cost_per_text_token],
        ["Input cached text token", pricing.input_cost_per_cached_text_token],
        ["Output text token", pricing.output_cost_per_text_token],
        ["Input audio token", pricing.input_cost_per_audio_token],
        ["Input cached audio token", pricing.input_cost_per_cached_audio_token],
        ["Output audio token", pricing.output_cost_per_audio_token],
        ["Input image token", pricing.input_cost_per_image_token],
        ["Input cached image token", pricing.input_cost_per_cached_image_token],
        ["Output image token", pricing.output_cost_per_image_token],
      ]
    : [];
  const hasPricing = pricing ? hasAnyPricing(pricing) : false;

  return (
    <div className="console-surface text-xs text-slate-600">
      <h3 className="console-subheading">Capabilities</h3>
      {capabilities.length > 0 ? (
        <ul className="mt-2 flex flex-wrap gap-2">
          {capabilities.map((capability) => (
            <li key={capability} className="console-chip">
              {formatCapability(capability)}
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-2 text-slate-400">No capability data.</p>
      )}

      <h3 className="console-subheading mt-4">Pricing (USD/M tokens)</h3>
      {pricing ? (
        <div className="mt-2 space-y-1 text-slate-500">
          {pricingEntries.map(([label, value]) => {
            if (value === undefined || value === null) {
              return null;
            }
            return (
              <p key={label}>
                <span className="font-semibold text-slate-600">{label}:</span>{" "}
                <span className="text-slate-700">
                  ${(value * 1_000_000).toFixed(2)}
                </span>
              </p>
            );
          })}
          {hasPricing ? null : (
            <p className="text-slate-400">No pricing data.</p>
          )}
        </div>
      ) : (
        <p className="mt-2 text-slate-400">No pricing data.</p>
      )}
    </div>
  );
}

function formatCapability(capability: string): string {
  return capability
    .split("-")
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

function formatProviderLabel(provider: string): string {
  return provider
    .split("-")
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

function hasAnyPricing(pricing: LanguageModelMetadata["pricing"]): boolean {
  if (!pricing) return false;
  return [
    pricing.input_cost_per_text_token,
    pricing.input_cost_per_cached_text_token,
    pricing.output_cost_per_text_token,
    pricing.input_cost_per_audio_token,
    pricing.input_cost_per_cached_audio_token,
    pricing.output_cost_per_audio_token,
    pricing.input_cost_per_image_token,
    pricing.input_cost_per_cached_image_token,
    pricing.output_cost_per_image_token,
  ].some((value) => value !== undefined);
}
