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
  const [modelSelection, setModelSelection] =
    useLocalStorageState<ModelSelection | null>(STORAGE_KEY_MODEL, null);

  const [providerApiKeys, setProviderApiKeys] = useState<ApiKeys>({});
  useEffect(() => {
    setProviderApiKeys((prev) => {
      const next = { ...prev };
      for (const option of modelOptions) {
        const storageKey = getProviderApiKeyStorageKey(option.provider);
        const storedValue = localStorage.getItem(storageKey);
        next[option.provider] = storedValue ?? undefined;
      }
      return next;
    });

    if (modelOptions.length > 0) {
      setModelSelection((current) => {
        if (current) {
          const stillValid = modelOptions.some(
            (option) =>
              option.provider === current.provider &&
              option.modelId === current.modelId,
          );
          if (stillValid) {
            return current;
          }
        }
        return modelOptions[0];
      });
    } else {
      setModelSelection(null);
    }
  }, [modelOptions, setModelSelection]);

  const [toolOptions, setToolOptions] = useState<ToolInfo[]>([]);
  const [enabledTools, setEnabledTools] = useLocalStorageState<string[]>(
    STORAGE_KEY_ENABLED_TOOLS,
    () => [],
  );
  const [toolsInitialized, setToolsInitialized] = useState(false);
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
  const [modelAudio, setModelAudio] = useState<AudioOptions | undefined>(
    undefined,
  );
  const [modelReasoning, setModelReasoning] = useState<
    ReasoningOptions | undefined
  >(undefined);
  const [modelModalities, setModelModalities] = useState<
    Modality[] | undefined
  >(undefined);

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
    [toolOptions, setEnabledTools],
  );

  const handleMcpServersChange = useCallback(
    (servers: McpServerConfig[]) => {
      setMcpServers(servers);
    },
    [setMcpServers],
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

  useEffect(() => {
    if (selectedModelOption) {
      setModelAudio(selectedModelOption.audio);
      setModelReasoning(selectedModelOption.reasoning);
      setModelModalities(selectedModelOption.modalities);
    } else {
      setModelAudio(undefined);
      setModelReasoning(undefined);
      setModelModalities(undefined);
    }
  }, [selectedModelOption]);

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
  };
}
