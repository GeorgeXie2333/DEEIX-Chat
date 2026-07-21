export const MODEL_OPTION_POLICY_PROTOCOLS = [
  "default",
  "openai_chat_completions",
  "openrouter_chat_completions",
  "openai_responses",
  "openrouter_responses",
  "openai_image_generations",
  "openai_image_edits",
  "openai_video_generations",
  "anthropic_messages",
  "gemini_generate_content",
  "google_image_generation",
  "gemini_interactions",
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
  "google_image_generation": [
    "google_search"
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
  openrouter_chat_completions: "OpenRouter（Chat Completions）",
  openai_responses: "OpenAI（Responses）",
  openrouter_responses: "OpenRouter（Responses）",
  openai_image_generations: "OpenAI（Images Generations）",
  openai_image_edits: "OpenAI（Images Edits）",
  openai_video_generations: "OpenAI（Video Generations）",
  anthropic_messages: "Anthropic（Messages）",
  gemini_generate_content: "Google（Generate Content）",
  google_image_generation: "Google（Image Generation）",
  gemini_interactions: "Google（Interactions）",
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

const GEMINI_INTERACTIONS_ALLOWED_PATHS = [
  "generation_config.temperature",
  "generation_config.top_p",
  "generation_config.max_output_tokens",
  "generation_config.thinking_level",
  "response_format.type",
  "response_format.aspect_ratio",
  "response_format.image_size",
  "response_format.mime_type",
  "responseFormat.type",
  "responseFormat.aspectRatio",
  "responseFormat.imageSize",
  "responseFormat.mimeType",
  "generationConfig.videoConfig.task",
  "generation_config.video_config.task",
];

const LEGACY_GOOGLE_IMAGE_GENERATION_ALLOWED_PATHS = [
  "aspect_ratio",
  "aspectRatio",
  "image_size",
  "imageSize",
  "imageConfig.aspectRatio",
  "imageConfig.imageSize",
  "responseFormat.image.aspectRatio",
  "responseFormat.image.imageSize",
  "generationConfig.imageConfig.aspectRatio",
  "generationConfig.imageConfig.imageSize",
  "generationConfig.responseFormat.image.aspectRatio",
  "generationConfig.responseFormat.image.imageSize",
];

const LEGACY_BUILT_IN_POLICY_PATHS: Record<string, { required: string[]; optional?: string[] }> = {
  default: {
    required: ["temperature", "top_p", "max_tokens", "max_output_tokens", "max_completion_tokens", "stop", "response_format.type"],
    optional: ["tools"],
  },
  openai_chat_completions: {
    required: ["service_tier", "presence_penalty", "frequency_penalty", "reasoning_effort", "verbosity", "thinking.type", "stream_options.include_usage"],
    optional: ["reasoning_summary"],
  },
  openai_responses: {
    required: ["service_tier", "reasoning.effort", "reasoning.summary", "text.verbosity"],
    optional: ["store", "reasoning.mode", "reasoning_effort", "reasoning_summary"],
  },
  openai_image_generations: {
    required: ["background", "moderation", "n", "output_compression", "output_format", "partial_images", "quality", "response_format", "size", "style", "user"],
  },
  openai_image_edits: {
    required: ["background", "input_fidelity", "n", "output_compression", "output_format", "partial_images", "quality", "response_format", "size", "user"],
  },
  google_image_generation: {
    required: ["generationConfig.responseModalities", "generationConfig.imageConfig.aspectRatio", "generationConfig.imageConfig.imageSize"],
    optional: ["generationConfig.thinkingConfig.thinkingLevel"],
  },
  anthropic_messages: {
    required: ["speed", "top_k", "thinking.type", "thinking.budget_tokens"],
    optional: ["output_config.effort", "cache_control"],
  },
  xai_responses: {
    required: ["reasoning.effort"],
    optional: ["store"],
  },
  xai_image: {
    required: ["aspect_ratio", "n", "resolution", "response_format"],
  },
  xai_image_edits: {
    required: ["aspect_ratio", "n", "resolution", "response_format"],
  },
  gemini_generate_content: {
    required: ["generationConfig.temperature", "generationConfig.topP", "generationConfig.maxOutputTokens", "generationConfig.responseMimeType"],
    optional: ["thinkingConfig.includeThoughts", "thinkingConfig.thinkingLevel"],
  },
};

const LEGACY_BUILT_IN_OPTIONAL_PROTOCOL_PATHS: Record<string, { required: string[]; optional?: string[] }> = {
  openrouter_chat_completions: {
    required: ["presence_penalty", "frequency_penalty", "reasoning_effort", "reasoning.effort", "reasoning.summary", "verbosity", "thinking.type", "stream_options.include_usage"],
  },
  openrouter_responses: {
    required: ["reasoning.effort", "reasoning.summary"],
  },
  openai_video_generations: {
    required: ["seconds", "size"],
  },
};

function modelOptionPathSetMatches(paths: string[], required: string[], optional: string[] = []): boolean {
  const pathSet = new Set(paths);
  if (pathSet.size !== paths.length || required.some((path) => !pathSet.has(path))) {
    return false;
  }
  const allowed = new Set([...required, ...optional]);
  return paths.every((path) => allowed.has(path));
}

function builtInModelOptionPathSetMatches(
  protocol: string,
  paths: string[],
  required: string[],
  optional: string[] = [],
): boolean {
  if (modelOptionPathSetMatches(paths, required, optional)) {
    return true;
  }
  return protocol === "google_image_generation" && modelOptionPathSetMatches(
    paths,
    LEGACY_GOOGLE_IMAGE_GENERATION_ALLOWED_PATHS,
    ["generationConfig.thinkingConfig.thinkingLevel"],
  );
}

function matchesLegacyBuiltInPolicyWithoutGeminiInteractions(rules: ModelOptionRuleMap): boolean {
  if (rules.gemini_interactions) {
    return false;
  }
  if (
    !rules.openrouter_chat_completions &&
    !modelOptionPathSetMatches(
      rules.google_image_generation ?? [],
      LEGACY_GOOGLE_IMAGE_GENERATION_ALLOWED_PATHS,
      ["generationConfig.thinkingConfig.thinkingLevel"],
    )
  ) {
    return false;
  }
  let allowedProtocolCount = Object.keys(LEGACY_BUILT_IN_POLICY_PATHS).length;
  for (const [protocol, spec] of Object.entries(LEGACY_BUILT_IN_OPTIONAL_PROTOCOL_PATHS)) {
    const paths = rules[protocol];
    if (paths) {
      if (!builtInModelOptionPathSetMatches(protocol, paths, spec.required, spec.optional)) {
        return false;
      }
      allowedProtocolCount++;
    }
  }
  if (Object.keys(rules).length !== allowedProtocolCount) {
    return false;
  }
  return Object.entries(LEGACY_BUILT_IN_POLICY_PATHS).every(([protocol, spec]) => {
    const paths = rules[protocol];
    return Boolean(paths && builtInModelOptionPathSetMatches(protocol, paths, spec.required, spec.optional));
  });
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
  let changed = false;
  if (matchesLegacyBuiltInPolicyWithoutGeminiInteractions(next)) {
    next.gemini_interactions = [...GEMINI_INTERACTIONS_ALLOWED_PATHS];
    changed = true;
  }
  const legacyOpenAIChatPaths = [
    "service_tier",
    "presence_penalty",
    "frequency_penalty",
    "reasoning_effort",
    "verbosity",
    "thinking.type",
    "stream_options.include_usage",
  ];
  const openAIChatPaths = next.openai_chat_completions;
  if (
    openAIChatPaths?.length === legacyOpenAIChatPaths.length &&
    legacyOpenAIChatPaths.every((path) => openAIChatPaths.includes(path))
  ) {
    openAIChatPaths.push("reasoning_summary");
    changed = true;
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
      protocol: "openai_responses",
      anchors: ["service_tier", "reasoning.effort", "reasoning.summary", "text.verbosity"],
      minAnchors: 3,
      additions: ["reasoning.mode"],
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
    {
      protocol: "google_image_generation",
      anchors: [
        "generationConfig.responseModalities",
        "generationConfig.imageConfig.aspectRatio",
        "generationConfig.imageConfig.imageSize",
      ],
      minAnchors: 2,
      additions: ["generationConfig.thinkingConfig.thinkingLevel"],
    },
  ];
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
    case "openrouter_chat_completions":
      return "openrouter_chat_completions";
    case "openrouter":
    case "openrouter_responses":
      return "openrouter_responses";
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
    case "gemini_interactions":
      return "gemini_interactions";
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
