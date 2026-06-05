export const MODEL_OPTION_POLICY_PROTOCOLS = [
  "default",
  "openai_chat_completions",
  "openai_responses",
  "openai_image_generations",
  "openai_image_edits",
  "openai_video_generations",
  "anthropic_messages",
  "gemini_generate_content",
  "google_image_generation",
  "xai_responses",
  "xai_image",
  "xai_image_edits",
] as const;

export type ModelOptionPolicyProtocol = (typeof MODEL_OPTION_POLICY_PROTOCOLS)[number];
export type ModelOptionPolicyMode = "allowlist" | "denylist" | "disabled" | string;

export type ModelOptionPolicy = {
  mode: ModelOptionPolicyMode;
  allowedPathsJSON: string;
  deniedPathsJSON: string;
  nativeTools?: NativeToolDefinition[];
  nativeToolAllowedTypesJSON?: string;
};

export type ModelOptionRuleMap = Partial<Record<ModelOptionPolicyProtocol | string, string[]>>;

export const DEFAULT_NATIVE_TOOL_ALLOWED_TYPES = `{
  "openai_chat_completions": [
    "web_search",
    "web_search_preview"
  ],
  "openai_responses": [
    "web_search",
    "web_search_preview",
    "shell",
    "image_generation",
    "code_interpreter"
  ],
  "anthropic_messages": [
    "web_search_20250305",
    "web_search_20260209",
    "web_fetch_20250910",
    "web_fetch_20260209",
    "code_execution_20250825",
    "code_execution_20260120",
    "advisor_20260301",
    "tool_search_tool_regex_20251119",
    "tool_search_tool_bm25_20251119"
  ],
  "gemini_generate_content": [
    "google_search",
    "code_execution"
  ],
  "xai_responses": [
    "web_search",
    "x_search",
    "code_interpreter"
  ]
}`;

const DEFAULT_NATIVE_TOOL_ALLOWED_TYPES_MAP = parseModelOptionRuleMap(DEFAULT_NATIVE_TOOL_ALLOWED_TYPES).value;

export type NativeToolDefinition = {
  protocol: string;
  provider: string;
  type: string;
  toolKey: string;
  label: string;
  description: string;
  payload: Record<string, unknown>;
  defaultEnabled: boolean;
  billable: boolean;
  billingUnit: string;
  priceNanousd: number;
  priceLabel: string;
  riskLevel: string;
  usageAliases: string[];
};

export type ModelNativeToolConfig = {
  id: string;
  key: string;
  protocol: string;
  protocols: string[];
  provider?: string;
  type: string;
  label: string;
  description?: string;
  enabled: boolean;
  defaultEnabled: boolean;
  payload: Record<string, unknown>;
};

export const MODEL_OPTION_POLICY_PROTOCOL_LABELS: Record<ModelOptionPolicyProtocol, string> = {
  default: "Default",
  openai_chat_completions: "OpenAI（Chat Completions）",
  openai_responses: "OpenAI（Responses）",
  openai_image_generations: "OpenAI（Images Generations）",
  openai_image_edits: "OpenAI（Images Edits）",
  openai_video_generations: "OpenAI（Video Generations）",
  anthropic_messages: "Anthropic（Messages）",
  gemini_generate_content: "Google（Generate Content）",
  google_image_generation: "Google（Image Generation）",
  xai_responses: "xAI（Responses）",
  xai_image: "xAI（Images Generations）",
  xai_image_edits: "xAI（Images Edits）",
};

export const HARD_DENIED_MODEL_OPTION_PATHS = [
  "model",
  "messages",
  "input",
  "instructions",
  "prompt",
  "system",
  "systemInstruction",
  "headers",
  "api_key",
  "apiKey",
  "base_url",
  "baseURL",
  "stream",
  "previous_response_id",
];

export function parseModelOptionRuleMap(raw: string): { value: ModelOptionRuleMap; error: string } {
  try {
    const parsed = JSON.parse(raw || "{}") as unknown;
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return { value: {}, error: "Configuration must be a JSON object" };
    }
    const value: ModelOptionRuleMap = {};
    for (const [key, paths] of Object.entries(parsed as Record<string, unknown>)) {
      if (!Array.isArray(paths)) {
        return { value: {}, error: `${key} must be a string array` };
      }
      value[key] = paths.map((path) => (typeof path === "string" ? path.trim() : "")).filter(Boolean);
    }
    return { value, error: "" };
  } catch {
    return { value: {}, error: "Invalid JSON format" };
  }
}

export function uniqueModelOptionPaths(paths: string[]): string[] {
  return [...new Set(paths.map((path) => path.trim()).filter(Boolean))];
}

export function normalizeModelOptionAllowedPathsJSON(raw: string): string {
  const parsed = parseModelOptionRuleMap(raw);
  if (parsed.error) {
    return raw;
  }
  const next: ModelOptionRuleMap = {};
  for (const [protocol, paths] of Object.entries(parsed.value)) {
    next[protocol] = [...(paths ?? [])];
  }
  const upgrades: Array<{
    protocol: string;
    anchors: string[];
    minAnchors: number;
    additions: string[];
  }> = [
    {
      protocol: "openai_video_generations",
      anchors: [],
      minAnchors: 0,
      additions: ["seconds", "size"],
    },
    {
      protocol: "anthropic_messages",
      anchors: ["speed", "top_k", "thinking.type"],
      minAnchors: 2,
      additions: ["output_config.effort"],
    },
    {
      protocol: "gemini_generate_content",
      anchors: [
        "generationConfig.temperature",
        "generationConfig.topP",
        "generationConfig.maxOutputTokens",
        "generationConfig.responseMimeType",
      ],
      minAnchors: 2,
      additions: ["thinkingConfig.thinkingLevel"],
    },
  ];
  let changed = false;
  for (const upgrade of upgrades) {
    const paths = next[upgrade.protocol];
    if (!paths) {
      continue;
    }
    const pathSet = new Set(paths);
    const anchorMatches = upgrade.anchors.filter((anchor) => pathSet.has(anchor)).length;
    if (anchorMatches < upgrade.minAnchors) {
      continue;
    }
    for (const addition of upgrade.additions) {
      if (pathSet.has(addition)) {
        continue;
      }
      paths.push(addition);
      pathSet.add(addition);
      changed = true;
    }
  }
  if (!next.openai_video_generations && next.openai_image_generations && next.openai_image_edits) {
    next.openai_video_generations = ["seconds", "size"];
    changed = true;
  }
  return changed ? JSON.stringify(next) : raw;
}

export function resolveModelOptionPolicyProtocol(protocol: string): ModelOptionPolicyProtocol {
  switch (protocol.trim().toLowerCase()) {
    case "openai":
    case "openai_responses":
      return "openai_responses";
    case "openai_chat_completions":
      return "openai_chat_completions";
    case "openai_image_generations":
      return "openai_image_generations";
    case "openai_image_edits":
      return "openai_image_edits";
    case "openai_video_generations":
      return "openai_video_generations";
    case "anthropic":
    case "claude":
    case "anthropic_messages":
      return "anthropic_messages";
    case "xai":
    case "grok":
    case "xai_responses":
      return "xai_responses";
    case "xai_image":
      return "xai_image";
    case "xai_image_edits":
      return "xai_image_edits";
    case "google":
    case "gemini":
    case "google_generate_content":
    case "gemini_generate_content":
      return "gemini_generate_content";
    case "google_image_generation":
      return "google_image_generation";
    default:
      return "openai_responses";
  }
}

export function effectiveModelOptionPaths(rules: ModelOptionRuleMap, protocol: string): string[] {
  if (protocol === "default") {
    return uniqueModelOptionPaths(rules.default ?? []);
  }
  return uniqueModelOptionPaths([...(rules.default ?? []), ...(rules[protocol] ?? [])]);
}

export function isNativeToolTypeAllowed(
  policy: ModelOptionPolicy,
  protocol: string,
  toolType: string,
  nativeToolKeys: string[] = [],
): boolean {
  const policyProtocol = resolveModelOptionPolicyProtocol(protocol);
  const normalizedToolType = toolType.trim();
  if (!normalizedToolType) {
    return false;
  }

  const catalogTools = policy.nativeTools ?? [];
  if (catalogTools.length > 0) {
    const matchingTools = catalogTools.filter((tool) => (
      tool.protocol.trim() === policyProtocol && tool.type.trim() === normalizedToolType
    ));
    if (matchingTools.length > 0) {
      const keySet = new Set(nativeToolKeys.map((key) => key.trim()).filter(Boolean));
      if (keySet.size > 0) {
        return matchingTools.some((tool) => keySet.has(tool.toolKey.trim()));
      }
      return matchingTools.some((tool) => tool.defaultEnabled);
    }
  }

  const configuredRules = parseModelOptionRuleMap(policy.nativeToolAllowedTypesJSON ?? "").value;
  const configuredTypes = configuredRules[policyProtocol];
  const allowedTypes = configuredTypes && configuredTypes.length > 0
    ? configuredTypes
    : DEFAULT_NATIVE_TOOL_ALLOWED_TYPES_MAP[policyProtocol] ?? [];
  return allowedTypes.includes(normalizedToolType);
}

function pathSegments(path: string): string[] {
  return path.split(".").map((item) => item.trim()).filter(Boolean);
}

function ruleAffectsPath(rule: string, path: string): boolean {
  const ruleParts = pathSegments(rule);
  const pathParts = pathSegments(path);
  if (ruleParts.length === 0 || pathParts.length === 0 || ruleParts.length > pathParts.length) {
    return false;
  }
  return ruleParts.every((part, index) => part === pathParts[index]);
}

export function isModelOptionPathFiltered({
  policy,
  protocol,
  path,
}: {
  policy: ModelOptionPolicy;
  protocol: string;
  path: string;
}): boolean {
  const mode = policy.mode?.trim() || "allowlist";
  if (mode === "disabled") {
    return true;
  }

  const policyProtocol = resolveModelOptionPolicyProtocol(protocol);
  const allowed = parseModelOptionRuleMap(normalizeModelOptionAllowedPathsJSON(policy.allowedPathsJSON)).value;
  const denied = parseModelOptionRuleMap(policy.deniedPathsJSON).value;
  const deniedPaths = uniqueModelOptionPaths([
    ...HARD_DENIED_MODEL_OPTION_PATHS,
    ...(mode === "denylist" ? effectiveModelOptionPaths(denied, policyProtocol) : []),
  ]);
  if (deniedPaths.some((rule) => ruleAffectsPath(rule, path))) {
    return true;
  }

  if (mode === "denylist") {
    return false;
  }

  const allowedPaths = effectiveModelOptionPaths(allowed, policyProtocol);
  return !allowedPaths.some((rule) => ruleAffectsPath(rule, path));
}
