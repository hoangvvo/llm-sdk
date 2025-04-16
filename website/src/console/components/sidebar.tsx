import type { LanguageModelMetadata } from "@hoangvvo/llm-sdk";
import type { ChangeEvent, Dispatch, SetStateAction } from "react";
import { useEffect, useMemo, useState } from "react";
import type {
  AgentBehaviorSettings,
  ApiKeys,
  MyContext,
  ToolInfo,
} from "../types";

export interface ModelSelection {
  provider: string;
  modelId: string;
}

export interface ModelOption extends ModelSelection {
  label: string;
  metadata?: LanguageModelMetadata;
}

interface SidebarProps {
  models: ModelOption[];
  selection: ModelSelection | null;
  onModelSelectionChange: Dispatch<SetStateAction<ModelSelection | null>>;
  modelSelectionErrorMessage?: string | null;
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
  disabledInstructions: boolean;
  onDisabledInstructionsChange: (value: boolean) => void;
  toolsInitialized: boolean;
}

export function Sidebar({
  models,
  selection,
  onModelSelectionChange,
  modelSelectionErrorMessage,
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
  disabledInstructions,
  onDisabledInstructionsChange,
  toolsInitialized,
}: SidebarProps) {
  return (
    <aside className="flex h-full w-96 shrink-0 flex-col overflow-auto border-l border-slate-200/70 bg-white/60 px-6 py-6 backdrop-blur-sm">
      <div className="space-y-6">
        <ModelSelectionSection
          models={models}
          selection={selection}
          onChange={onModelSelectionChange}
          errorMessage={modelSelectionErrorMessage}
          apiKeys={apiKeys}
          onSaveApiKey={onSaveApiKey}
        />
        <AgentBehaviorSection
          behavior={behavior}
          onChange={onBehaviorChange}
          disabledInstructions={disabledInstructions}
          onDisabledInstructionsChange={onDisabledInstructionsChange}
        />
        <ToolSection
          tools={tools}
          enabledTools={enabledTools}
          onEnabledToolsChange={onEnabledToolsChange}
          errorMessage={toolErrorMessage}
          initialized={toolsInitialized}
        />
        <ContextSection context={context} onChange={onContextChange} />
      </div>
    </aside>
  );
}

interface ModelSelectionSectionProps {
  models: ModelOption[];
  selection: ModelSelection | null;
  onChange: Dispatch<SetStateAction<ModelSelection | null>>;
  errorMessage?: string | null;
  apiKeys: ApiKeys;
  onSaveApiKey: (provider: string, apiKey: string) => void;
}

function ModelSelectionSection({
  models,
  selection,
  onChange,
  errorMessage,
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
          Choose which LLM model the agent should use.
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
            {errorMessage ?? "Loading models…"}
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
  disabledInstructions: boolean;
  onDisabledInstructionsChange: (value: boolean) => void;
}

function AgentBehaviorSection({
  behavior,
  onChange,
  disabledInstructions,
  onDisabledInstructionsChange,
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
      <h2 className="console-section-title">Agent Behavior</h2>
      <p className="text-xs text-slate-500">
        Tune sampling parameters for the agent.
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
      <div className="border-t border-slate-200/70 pt-3">
        <label className="flex items-start gap-3 text-xs text-slate-600">
          <input
            type="checkbox"
            className="mt-1 h-4 w-4 rounded border-slate-300 text-slate-900 focus:ring-2 focus:ring-slate-500"
            checked={disabledInstructions}
            onChange={(event) => {
              onDisabledInstructionsChange(event.target.checked);
            }}
          />
          <span>
            <span className="font-semibold text-slate-700">
              Disable system instructions
            </span>
            <p className="text-[11px] leading-snug text-slate-500">
              Prevents default instructions from being sent to the model
              (workaround for some legacy models)
            </p>
          </span>
        </label>
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
      <h2 className="console-section-title">Tools</h2>
      <p className="text-xs text-slate-500">
        Toggle which tools the agent can invoke during a run.
      </p>
      {tools.length > 0 ? (
        <ul className="mt-2 space-y-2 text-xs text-slate-600">
          {tools.map((tool) => {
            const checked = enabledTools.includes(tool.name);
            return (
              <li key={tool.name} className="flex items-start gap-2">
                <input
                  type="checkbox"
                  className="mt-1 h-4 w-4 rounded border-slate-300 text-slate-900 focus:ring-2 focus:ring-slate-500"
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
  const handleNameChange = (value: string) => {
    onChange((prev) => {
      const next = { ...prev };
      if (value.trim().length === 0) {
        delete next.name;
      } else {
        next.name = value;
      }
      return next;
    });
  };

  const handleLocationChange = (value: string) => {
    onChange((prev) => {
      const next = { ...prev };
      if (value.trim().length === 0) {
        delete next.location;
      } else {
        next.location = value;
      }
      return next;
    });
  };

  const handleUnitsChange = (value: string) => {
    onChange((prev) => {
      const next = { ...prev };
      if (value.trim().length === 0) {
        delete next.units;
      } else {
        next.units = value;
      }
      return next;
    });
  };

  return (
    <div className="console-surface space-y-3">
      <h2 className="console-section-title">Context</h2>
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
            handleNameChange(event.target.value);
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
            handleLocationChange(event.target.value);
          }}
        />
      </label>
      <label className="console-label">
        Preferred units
        <input
          type="text"
          className="console-field mt-2 w-full"
          placeholder="e.g. metric, imperial"
          value={context.units ?? ""}
          onChange={(event) => {
            handleUnitsChange(event.target.value);
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
