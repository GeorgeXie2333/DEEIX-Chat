import type { ConversationOptions } from "@/shared/api/conversation.types";
import { isGemini3PlusModel } from "./advanced-settings.ts";

export type NativeToolOption = {
  type: string;
  labelKey: string;
  descriptionKey: string;
  iconKind?: "x-logo";
  matchKey?: string;
  payload?: Record<string, unknown>;
};

export type NativeToolGroup = {
  key: "grok" | "openai" | "claude" | "gemini";
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
    iconKind: "x-logo",
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

const GEMINI_NATIVE_TOOL_OPTIONS: NativeToolOption[] = [
  {
    type: "google_search",
    labelKey: "webSearch",
    descriptionKey: "geminiWebSearch",
    matchKey: "google_search",
    payload: { google_search: {} },
  },
  {
    type: "code_execution",
    labelKey: "codeExecution",
    descriptionKey: "geminiCodeExecution",
    matchKey: "code_execution",
    payload: { code_execution: {} },
  },
];

const NATIVE_TOOL_TYPES = new Set(
  [
    ...XAI_NATIVE_TOOL_OPTIONS,
    ...OPENAI_RESPONSES_NATIVE_TOOL_OPTIONS,
    ...ANTHROPIC_NATIVE_TOOL_OPTIONS,
    ...GEMINI_NATIVE_TOOL_OPTIONS,
  ].map((item) => item.type),
);

const NATIVE_TOOL_OPTIONS_BY_TYPE = new Map(
  [
    ...XAI_NATIVE_TOOL_OPTIONS,
    ...OPENAI_RESPONSES_NATIVE_TOOL_OPTIONS,
    ...ANTHROPIC_NATIVE_TOOL_OPTIONS,
    ...GEMINI_NATIVE_TOOL_OPTIONS,
  ].map((item) => [item.type, item]),
);

function isPlainOptionObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

export function resolveNativeToolGroup(protocol: string, isMediaMode: boolean, modelName = ""): NativeToolGroup | null {
  if (isMediaMode) {
    return null;
  }
  switch (protocol) {
    case "xai_responses":
      return {
        key: "grok",
        options: XAI_NATIVE_TOOL_OPTIONS,
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
    case "gemini_generate_content":
    case "google_generate_content":
      return isGemini3PlusModel(modelName)
        ? {
            key: "gemini",
            options: GEMINI_NATIVE_TOOL_OPTIONS,
          }
        : null;
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

function toolMatchesOption(tool: Record<string, unknown>, option: NativeToolOption): boolean {
  if (tool.type === option.type) {
    return true;
  }
  return Boolean(option.matchKey && option.matchKey in tool);
}

export function hasProviderTool(options: ConversationOptions, type: string): boolean {
  const option = NATIVE_TOOL_OPTIONS_BY_TYPE.get(type);
  return providerToolObjectsFromOptions(options).some((tool) => (
    option ? toolMatchesOption(tool, option) : tool.type === type
  ));
}

export function countProviderTools(options: ConversationOptions, group: NativeToolGroup | null): number {
  if (!group) {
    return 0;
  }
  const selectedTypes = new Set<string>();
  providerToolObjectsFromOptions(options).forEach((tool) => {
    for (const option of group.options) {
      if (toolMatchesOption(tool, option)) {
        selectedTypes.add(option.type);
      }
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
  const hasTool = tools.some((tool) => toolMatchesOption(tool, toolOption));
  const nextTools = enabled
    ? hasTool
      ? tools
      : [...tools, { ...(toolOption.payload ?? { type }) }]
    : tools.filter((tool) => !toolMatchesOption(tool, toolOption));

  if (nextTools.length === 0) {
    const { tools: _tools, ...rest } = options;
    return rest;
  }

  return { ...options, tools: nextTools };
}
