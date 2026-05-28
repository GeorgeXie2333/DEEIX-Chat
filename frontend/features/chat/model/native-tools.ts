import type { ConversationOptions } from "@/shared/api/conversation.types";

export type NativeToolOption = {
  type: string;
  labelKey: string;
  descriptionKey: string;
  payload?: Record<string, unknown>;
};

export type NativeToolGroup = {
  key: "grok" | "openai" | "claude";
  options: NativeToolOption[];
};

const XAI_NATIVE_TOOL_OPTIONS: NativeToolOption[] = [
  {
    type: "web_search",
    labelKey: "webSearch",
    descriptionKey: "grokWebSearch",
  },
  {
    type: "x_search",
    labelKey: "xSearch",
    descriptionKey: "grokXSearch",
  },
  {
    type: "code_interpreter",
    labelKey: "codeInterpreter",
    descriptionKey: "grokCodeInterpreter",
  },
];

const OPENAI_RESPONSES_NATIVE_TOOL_OPTIONS: NativeToolOption[] = [
  {
    type: "web_search",
    labelKey: "webSearch",
    descriptionKey: "openaiWebSearch",
    payload: { type: "web_search" },
  },
  {
    type: "shell",
    labelKey: "shell",
    descriptionKey: "openaiShell",
    payload: {
      type: "shell",
      environment: { type: "container_auto" },
    },
  },
  {
    type: "image_generation",
    labelKey: "imageGeneration",
    descriptionKey: "openaiImageGeneration",
    payload: { type: "image_generation" },
  },
  {
    type: "code_interpreter",
    labelKey: "codeInterpreter",
    descriptionKey: "openaiCodeInterpreter",
    payload: {
      type: "code_interpreter",
      container: { type: "auto" },
    },
  },
];

const OPENAI_CHAT_NATIVE_TOOL_OPTIONS: NativeToolOption[] = [
  {
    type: "web_search",
    labelKey: "webSearch",
    descriptionKey: "openaiWebSearch",
    payload: { type: "web_search" },
  },
  {
    type: "web_search_preview",
    labelKey: "webSearchPreview",
    descriptionKey: "openaiWebSearchPreview",
    payload: { type: "web_search_preview" },
  },
];

const ANTHROPIC_NATIVE_TOOL_OPTIONS: NativeToolOption[] = [
  {
    type: "web_search_20260209",
    labelKey: "webSearch",
    descriptionKey: "claudeWebSearch",
    payload: { type: "web_search_20260209", name: "web_search", allowed_callers: ["direct"] },
  },
  {
    type: "web_fetch_20260209",
    labelKey: "webFetch",
    descriptionKey: "claudeWebFetch",
    payload: { type: "web_fetch_20260209", name: "web_fetch", allowed_callers: ["direct"] },
  },
  {
    type: "code_execution_20260120",
    labelKey: "codeExecution",
    descriptionKey: "claudeCodeExecution",
    payload: { type: "code_execution_20260120", name: "code_execution" },
  },
  {
    type: "advisor_20260301",
    labelKey: "advisor",
    descriptionKey: "claudeAdvisor",
    payload: { type: "advisor_20260301", name: "advisor", model: "claude-opus-4-7" },
  },
  {
    type: "tool_search_tool_regex_20251119",
    labelKey: "toolSearchRegex",
    descriptionKey: "claudeToolSearchRegex",
    payload: { type: "tool_search_tool_regex_20251119", name: "tool_search_tool_regex" },
  },
  {
    type: "tool_search_tool_bm25_20251119",
    labelKey: "toolSearchBm25",
    descriptionKey: "claudeToolSearchBm25",
    payload: { type: "tool_search_tool_bm25_20251119", name: "tool_search_tool_bm25" },
  },
];

const NATIVE_TOOL_TYPES = new Set(
  [
    ...XAI_NATIVE_TOOL_OPTIONS,
    ...OPENAI_RESPONSES_NATIVE_TOOL_OPTIONS,
    ...OPENAI_CHAT_NATIVE_TOOL_OPTIONS,
    ...ANTHROPIC_NATIVE_TOOL_OPTIONS,
  ].map((item) => item.type),
);

function isPlainOptionObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

export function resolveNativeToolGroup(protocol: string, isMediaMode: boolean): NativeToolGroup | null {
  if (isMediaMode) {
    return null;
  }
  switch (protocol) {
    case "xai_responses":
      return {
        key: "grok",
        options: XAI_NATIVE_TOOL_OPTIONS,
      };
    case "openai_chat_completions":
      return {
        key: "openai",
        options: OPENAI_CHAT_NATIVE_TOOL_OPTIONS,
      };
    case "openai_responses":
      return {
        key: "openai",
        options: OPENAI_RESPONSES_NATIVE_TOOL_OPTIONS,
      };
    case "anthropic_messages":
      return {
        key: "claude",
        options: ANTHROPIC_NATIVE_TOOL_OPTIONS,
      };
    default:
      return null;
  }
}

export function providerToolObjectsFromOptions(options: ConversationOptions): Record<string, unknown>[] {
  const rawTools = options.tools;
  if (!Array.isArray(rawTools)) {
    return [];
  }
  return rawTools.filter(isPlainOptionObject);
}

export function hasProviderTool(options: ConversationOptions, type: string): boolean {
  return providerToolObjectsFromOptions(options).some((tool) => tool.type === type);
}

export function countProviderTools(options: ConversationOptions, group: NativeToolGroup | null): number {
  if (!group) {
    return 0;
  }
  const groupTypes = new Set(group.options.map((tool) => tool.type));
  const selectedTypes = new Set<string>();
  providerToolObjectsFromOptions(options).forEach((tool) => {
    const type = typeof tool.type === "string" ? tool.type : "";
    if (groupTypes.has(type)) {
      selectedTypes.add(type);
    }
  });
  return selectedTypes.size;
}

export function shouldShowMCPToolsMenu(availableToolCount: number, isMediaMode: boolean): boolean {
  return availableToolCount > 0 && !isMediaMode;
}

export function setProviderToolEnabled(
  options: ConversationOptions,
  toolOption: NativeToolOption,
  enabled: boolean,
): ConversationOptions {
  const type = toolOption.type;
  if (!NATIVE_TOOL_TYPES.has(type)) {
    return options;
  }
  const tools = providerToolObjectsFromOptions(options);
  const hasTool = tools.some((tool) => tool.type === type);
  const nextTools = enabled
    ? hasTool
      ? tools
      : [...tools, { ...(toolOption.payload ?? { type }) }]
    : tools.filter((tool) => tool.type !== type);

  if (nextTools.length === 0) {
    const { tools: _tools, ...rest } = options;
    return rest;
  }

  return { ...options, tools: nextTools };
}
