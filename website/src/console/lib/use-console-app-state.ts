import type {
  AudioOptions,
  LanguageModelMetadata,
  Modality,
  ReasoningOptions,
} from "@hoangvvo/llm-sdk";
import {
  useCallback,
  useMemo,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";
import modelsJson from "../../../../models/models.json" with { type: "json" };
import {
  availableToolkits,
  availableTools,
} from "../../../../agent-js/examples/server/agent.ts";
import type {
  AgentBehaviorSettings,
  ApiKeys,
  McpServerConfig,
  ModelInfo,
  ToolkitInfo,
  ToolInfo,
  WebSearchSettings,
} from "../types.ts";
import { getCredentialProvider } from "../types.ts";
import { useLocalStorageState } from "./use-local-storage-state.ts";
export interface ModelSelection {
  provider: string;
  modelId: string;
}

export interface ModelOption extends ModelSelection {
  label: string;
  metadata?: LanguageModelMetadata;
  audio?: AudioOptions;
  reasoning?: ReasoningOptions;
  modalities?: Modality[];
}

export const STORAGE_KEY_MODEL = "console-selected-model";
export const STORAGE_KEY_PROVIDER_PREFIX = "console-provider-api-key:";
export const STORAGE_KEY_CONTEXT = "console-user-context";
export const STORAGE_KEY_ENABLED_TOOLS = "console-enabled-tools";
export const STORAGE_KEY_ENABLED_TOOLKITS = "console-enabled-toolkits";
export const STORAGE_KEY_WEB_SEARCH = "console-web-search";
export const STORAGE_KEY_AGENT_BEHAVIOR = "console-agent-behavior";
export const STORAGE_KEY_MCP_SERVERS = "console-mcp-servers";

const RAW_MODEL_LIST = (modelsJson.models ?? []) as ModelInfo[];
const MODEL_OPTIONS: ModelOption[] = RAW_MODEL_LIST.map(mapModelDefinition);
const TOOL_OPTIONS: ToolInfo[] = availableTools.flatMap((tool) =>
  tool.type === "function"
    ? [{ name: tool.name, description: tool.description }]
    : [],
);
const TOOLKIT_OPTIONS: ToolkitInfo[] = availableToolkits.map(
  ({ name, description }) => ({ name, description }),
);

function mapModelDefinition(info: ModelInfo): ModelOption {
  return {
    provider: info.provider,
    modelId: info.model_id,
    label: `${formatProviderName(info.provider)} – ${info.model_id}`,
    metadata: info.metadata,
    audio: info.default_audio,
    reasoning: info.default_reasoning,
    modalities: info.default_modalities,
  };
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
  return `${STORAGE_KEY_PROVIDER_PREFIX}${getCredentialProvider(provider)}`;
}

function resolveStateUpdate<T>(update: SetStateAction<T>, current: T): T {
  return typeof update === "function"
    ? (update as (previous: T) => T)(current)
    : update;
}

function readProviderApiKeys(modelOptions: ModelOption[]): ApiKeys {
  const next: ApiKeys = {};
  if (typeof window === "undefined") {
    return next;
  }
  for (const option of modelOptions) {
    const provider = getCredentialProvider(option.provider);
    const storageKey = getProviderApiKeyStorageKey(provider);
    next[provider] = window.localStorage.getItem(storageKey) ?? undefined;
  }
  return next;
}

function createInitialWebSearchSettings(): WebSearchSettings {
  if (typeof window === "undefined") {
    return { enabled: true };
  }

  const storedTools = window.localStorage.getItem(STORAGE_KEY_ENABLED_TOOLS);
  if (storedTools === null) {
    return { enabled: true };
  }

  try {
    const parsed = JSON.parse(storedTools) as unknown;
    return {
      enabled: Array.isArray(parsed) && parsed.includes("web_search"),
    };
  } catch {
    return { enabled: true };
  }
}

function getModelSelectionKey(
  selection: ModelSelection | null | undefined,
): string | null {
  if (!selection) {
    return null;
  }
  return `${selection.provider}:${selection.modelId}`;
}

function normalizeModelSelection(
  selection: ModelSelection | null,
  modelOptions: ModelOption[],
): ModelSelection | null {
  if (modelOptions.length === 0) {
    return null;
  }
  if (
    selection &&
    modelOptions.some(
      (option) =>
        option.provider === selection.provider &&
        option.modelId === selection.modelId,
    )
  ) {
    return selection;
  }
  return modelOptions[0];
}

function createModelFeatureState(selectedModelOption: ModelOption | undefined) {
  return {
    modelKey: getModelSelectionKey(selectedModelOption),
    audio: selectedModelOption?.audio,
    reasoning: selectedModelOption?.reasoning,
    modalities: selectedModelOption?.modalities,
  };
}

export interface ConsoleAppState<Context> {
  modelOptions: ModelOption[];
  modelSelection: ModelSelection | null;
  setModelSelection: Dispatch<SetStateAction<ModelSelection | null>>;
  selectedModelOption: ModelOption | undefined;
  providerApiKeys: ApiKeys;
  handleSaveProviderApiKey: (provider: string, apiKey: string) => void;
  toolOptions: ToolInfo[];
  enabledTools: string[];
  handleEnabledToolsChange: (tools: string[]) => void;
  toolkitOptions: ToolkitInfo[];
  enabledToolkits: string[];
  handleEnabledToolkitsChange: (toolkits: string[]) => void;
  webSearch: WebSearchSettings;
  setWebSearch: Dispatch<SetStateAction<WebSearchSettings>>;
  mcpServers: McpServerConfig[];
  handleMcpServersChange: (servers: McpServerConfig[]) => void;
  agentBehavior: AgentBehaviorSettings;
  setAgentBehavior: Dispatch<SetStateAction<AgentBehaviorSettings>>;
  userContext: Context;
  setUserContext: Dispatch<SetStateAction<Context>>;
  modelAudio: AudioOptions | undefined;
  setModelAudio: Dispatch<SetStateAction<AudioOptions | undefined>>;
  modelReasoning: ReasoningOptions | undefined;
  setModelReasoning: Dispatch<SetStateAction<ReasoningOptions | undefined>>;
  modelModalities: Modality[] | undefined;
  setModelModalities: Dispatch<SetStateAction<Modality[] | undefined>>;
}

export function useConsoleAppState<Context>(): ConsoleAppState<Context> {
  const modelOptions = MODEL_OPTIONS;
  const [storedModelSelection, setStoredModelSelection] =
    useLocalStorageState<ModelSelection | null>(STORAGE_KEY_MODEL, null);
  const modelSelection = useMemo(
    () => normalizeModelSelection(storedModelSelection, modelOptions),
    [modelOptions, storedModelSelection],
  );

  const [providerApiKeys, setProviderApiKeys] = useState<ApiKeys>(() =>
    readProviderApiKeys(modelOptions),
  );
  const [enabledTools, setEnabledTools] = useLocalStorageState<string[]>(
    STORAGE_KEY_ENABLED_TOOLS,
    () => [],
  );
  const [enabledToolkits, setEnabledToolkits] = useLocalStorageState<string[]>(
    STORAGE_KEY_ENABLED_TOOLKITS,
    () => ["artifacts"],
  );
  const [webSearch, setWebSearch] = useLocalStorageState<WebSearchSettings>(
    STORAGE_KEY_WEB_SEARCH,
    createInitialWebSearchSettings,
  );
  const [agentBehavior, setAgentBehavior] =
    useLocalStorageState<AgentBehaviorSettings>(
      STORAGE_KEY_AGENT_BEHAVIOR,
      () => ({}),
    );
  const [userContext, setUserContext] = useLocalStorageState<Context>(
    STORAGE_KEY_CONTEXT,
    (() => ({})) as () => Context,
  );
  const [mcpServers, setMcpServers] = useLocalStorageState<McpServerConfig[]>(
    STORAGE_KEY_MCP_SERVERS,
    () => [],
  );
  const supportedMcpServers = useMemo(
    () => mcpServers.filter((server) => server.type === "streamable-http"),
    [mcpServers],
  );

  const [hasStoredToolPreference, setHasStoredToolPreference] = useState(() => {
    if (typeof window === "undefined") {
      return false;
    }
    return window.localStorage.getItem(STORAGE_KEY_ENABLED_TOOLS) !== null;
  });
  const [hasStoredToolkitPreference, setHasStoredToolkitPreference] = useState(
    () => {
      if (typeof window === "undefined") {
        return false;
      }
      return window.localStorage.getItem(STORAGE_KEY_ENABLED_TOOLKITS) !== null;
    },
  );

  const handleSaveProviderApiKey = useCallback(
    (provider: string, apiKey: string) => {
      const credentialProvider = getCredentialProvider(provider);
      const trimmed = apiKey.trim();
      setProviderApiKeys((prev) => {
        const next = { ...prev };
        const storageKey = getProviderApiKeyStorageKey(credentialProvider);
        if (trimmed) {
          next[credentialProvider] = trimmed;
          localStorage.setItem(storageKey, trimmed);
        } else {
          next[credentialProvider] = undefined;
          localStorage.removeItem(storageKey);
        }
        return next;
      });
    },
    [],
  );

  const toolOptions = useMemo(
    () =>
      TOOL_OPTIONS.filter(
        (tool) =>
          !tool.providers ||
          !modelSelection ||
          tool.providers.includes(modelSelection.provider),
      ),
    [modelSelection],
  );
  const toolkitOptions = TOOLKIT_OPTIONS;

  const handleEnabledToolsChange = useCallback(
    (next: string[]) => {
      const allToolNames = TOOL_OPTIONS.map((tool) => tool.name);
      const visibleToolNames = new Set(toolOptions.map((tool) => tool.name));
      const nextVisibleTools = new Set(next);
      const selectedTools = new Set(
        enabledTools.length > 0 || hasStoredToolPreference
          ? enabledTools
          : allToolNames,
      );

      for (const name of visibleToolNames) {
        if (nextVisibleTools.has(name)) {
          selectedTools.add(name);
        } else {
          selectedTools.delete(name);
        }
      }

      const normalized = allToolNames.filter((name) => selectedTools.has(name));
      setEnabledTools(normalized);
      setHasStoredToolPreference(true);
      localStorage.setItem(
        STORAGE_KEY_ENABLED_TOOLS,
        JSON.stringify(normalized),
      );
    },
    [enabledTools, hasStoredToolPreference, setEnabledTools, toolOptions],
  );

  const handleEnabledToolkitsChange = useCallback(
    (next: string[]) => {
      const toolkitNames = TOOLKIT_OPTIONS.map((toolkit) => toolkit.name);
      const nextSet = new Set(next);
      const normalized = toolkitNames.filter((name) => nextSet.has(name));
      setEnabledToolkits(normalized);
      setHasStoredToolkitPreference(true);
    },
    [setEnabledToolkits],
  );

  const handleMcpServersChange = useCallback(
    (servers: McpServerConfig[]) => {
      setMcpServers(servers);
    },
    [setMcpServers],
  );

  const setModelSelection = useCallback<
    Dispatch<SetStateAction<ModelSelection | null>>
  >(
    (update) => {
      setStoredModelSelection((current) =>
        normalizeModelSelection(
          resolveStateUpdate(update, current),
          modelOptions,
        ),
      );
    },
    [modelOptions, setStoredModelSelection],
  );

  const selectedModelOption = useMemo(() => {
    if (!modelSelection) {
      return undefined;
    }
    return modelOptions.find(
      (option) =>
        option.provider === modelSelection.provider &&
        option.modelId === modelSelection.modelId,
    );
  }, [modelOptions, modelSelection]);

  const [modelFeatureState, setModelFeatureState] = useState(() =>
    createModelFeatureState(selectedModelOption),
  );
  const selectedModelKey = getModelSelectionKey(modelSelection);

  if (modelFeatureState.modelKey !== selectedModelKey) {
    setModelFeatureState(createModelFeatureState(selectedModelOption));
  }
  const normalizedEnabledTools = useMemo(() => {
    const toolNames = toolOptions.map((tool) => tool.name);
    if (toolNames.length === 0) {
      return [];
    }

    const baseSelection =
      enabledTools.length > 0 || hasStoredToolPreference
        ? enabledTools
        : toolNames;
    const allowed = new Set(
      baseSelection.filter((name) => toolNames.includes(name)),
    );
    return toolNames.filter((name) => allowed.has(name));
  }, [enabledTools, hasStoredToolPreference, toolOptions]);
  const normalizedEnabledToolkits = useMemo(() => {
    const toolkitNames = toolkitOptions.map((toolkit) => toolkit.name);
    if (toolkitNames.length === 0) {
      return enabledToolkits;
    }

    const baseSelection = hasStoredToolkitPreference
      ? enabledToolkits
      : toolkitNames;
    const allowed = new Set(baseSelection);
    return toolkitNames.filter((name) => allowed.has(name));
  }, [enabledToolkits, hasStoredToolkitPreference, toolkitOptions]);

  const setModelAudio = useCallback<
    Dispatch<SetStateAction<AudioOptions | undefined>>
  >((update) => {
    setModelFeatureState((current) => ({
      ...current,
      audio: resolveStateUpdate(update, current.audio),
    }));
  }, []);

  const setModelReasoning = useCallback<
    Dispatch<SetStateAction<ReasoningOptions | undefined>>
  >((update) => {
    setModelFeatureState((current) => ({
      ...current,
      reasoning: resolveStateUpdate(update, current.reasoning),
    }));
  }, []);

  const setModelModalities = useCallback<
    Dispatch<SetStateAction<Modality[] | undefined>>
  >((update) => {
    setModelFeatureState((current) => ({
      ...current,
      modalities: resolveStateUpdate(update, current.modalities),
    }));
  }, []);

  return {
    modelOptions,
    modelSelection,
    setModelSelection,
    selectedModelOption,
    providerApiKeys,
    handleSaveProviderApiKey,
    toolOptions,
    enabledTools: normalizedEnabledTools,
    handleEnabledToolsChange,
    toolkitOptions,
    enabledToolkits: normalizedEnabledToolkits,
    handleEnabledToolkitsChange,
    webSearch,
    setWebSearch,
    mcpServers: supportedMcpServers,
    handleMcpServersChange,
    agentBehavior,
    setAgentBehavior,
    userContext,
    setUserContext,
    modelAudio: modelFeatureState.audio,
    setModelAudio,
    modelReasoning: modelFeatureState.reasoning,
    setModelReasoning,
    modelModalities: modelFeatureState.modalities,
    setModelModalities,
  };
}
