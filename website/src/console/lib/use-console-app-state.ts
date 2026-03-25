import type {
  AudioOptions,
  LanguageModelMetadata,
  Modality,
  ReasoningOptions,
} from "@hoangvvo/llm-sdk";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";
import modelsJson from "../../../../models/models.json" with { type: "json" };
import type {
  AgentBehaviorSettings,
  ApiKeys,
  McpServerConfig,
  ModelInfo,
  ToolInfo,
} from "../types.ts";
import { normalizeBaseUrl, parseExampleServerUrls } from "./example-server.ts";
import { useFetchInitialData } from "./use-fetch-initial-data.ts";
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

const env = import.meta.env as Record<string, string | undefined>;

export const DEFAULT_API_BASE_URL = normalizeBaseUrl("http://localhost:4000");
export const EXAMPLE_SERVER_URL_OPTIONS = parseExampleServerUrls(
  env.EXAMPLE_SERVER_URLS ?? env.PUBLIC_EXAMPLE_SERVER_URLS,
);

export const STORAGE_KEY_SERVER_URL = "console-example-server-url";
export const STORAGE_KEY_MODEL = "console-selected-model";
export const STORAGE_KEY_PROVIDER_PREFIX = "console-provider-api-key:";
export const STORAGE_KEY_CONTEXT = "console-user-context";
export const STORAGE_KEY_ENABLED_TOOLS = "console-enabled-tools";
export const STORAGE_KEY_AGENT_BEHAVIOR = "console-agent-behavior";
export const STORAGE_KEY_MCP_SERVERS = "console-mcp-servers";

const RAW_MODEL_LIST = (modelsJson.models ?? []) as ModelInfo[];
const MODEL_OPTIONS: ModelOption[] = RAW_MODEL_LIST.map(mapModelDefinition);

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
  return `${STORAGE_KEY_PROVIDER_PREFIX}${provider}`;
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
    const storageKey = getProviderApiKeyStorageKey(option.provider);
    next[option.provider] =
      window.localStorage.getItem(storageKey) ?? undefined;
  }
  return next;
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
  serverOptions: string[];
  serverUrl: string;
  handleServerUrlChange: (value: string) => void;
  runStreamUrl: string;
  modelOptions: ModelOption[];
  modelSelection: ModelSelection | null;
  setModelSelection: Dispatch<SetStateAction<ModelSelection | null>>;
  selectedModelOption: ModelOption | undefined;
  providerApiKeys: ApiKeys;
  handleSaveProviderApiKey: (provider: string, apiKey: string) => void;
  toolOptions: ToolInfo[];
  toolsError: string | null;
  toolsInitialized: boolean;
  enabledTools: string[];
  handleEnabledToolsChange: (tools: string[]) => void;
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
  const serverOptions = EXAMPLE_SERVER_URL_OPTIONS;
  const hasServerOptions = serverOptions.length > 0;

  const [apiBaseUrl, setApiBaseUrl] = useLocalStorageState<string>(
    STORAGE_KEY_SERVER_URL,
    () => serverOptions[0] ?? DEFAULT_API_BASE_URL,
  );
  const normalizedApiBaseUrl = useMemo(
    () => normalizeBaseUrl(apiBaseUrl),
    [apiBaseUrl],
  );
  const effectiveApiBaseUrl = hasServerOptions
    ? normalizedApiBaseUrl
    : DEFAULT_API_BASE_URL;

  useEffect(() => {
    if (!hasServerOptions) {
      return;
    }
    if (!serverOptions.includes(normalizedApiBaseUrl)) {
      setApiBaseUrl(serverOptions[0]);
    }
  }, [hasServerOptions, normalizedApiBaseUrl, serverOptions, setApiBaseUrl]);

  const handleServerUrlChange = useCallback(
    (value: string) => {
      setApiBaseUrl(normalizeBaseUrl(value));
    },
    [setApiBaseUrl],
  );

  const runStreamUrl = `${effectiveApiBaseUrl}/run-stream`;
  const toolsUrl = `${effectiveApiBaseUrl}/tools`;

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

  const [hasStoredToolPreference] = useState(() => {
    if (typeof window === "undefined") {
      return false;
    }
    return window.localStorage.getItem(STORAGE_KEY_ENABLED_TOOLS) !== null;
  });

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

  const toolOptions = useMemo(() => toolsData ?? [], [toolsData]);
  const toolsInitialized = toolsData !== null || toolsError !== null;

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
    [toolOptions, setEnabledTools],
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
    serverOptions,
    serverUrl: normalizedApiBaseUrl,
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
    enabledTools: normalizedEnabledTools,
    handleEnabledToolsChange,
    mcpServers,
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
