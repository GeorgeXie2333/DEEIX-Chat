import type { ConversationOptions } from "@/shared/api/conversation.types";
import type { ModelOptionPolicy } from "../../../shared/lib/model-option-policy.ts";
import { isModelOptionPathFiltered } from "../../../shared/lib/model-option-policy.ts";

export type AdvancedSettingKind =
  | "temperature"
  | "reasoningEffort"
  | "verbosity"
  | "imageQuality"
  | "imageResolution";
export type AdvancedSettingCustomValueKind = "openaiImage2Resolution";

export type AdvancedSettingDefinition = {
  kind: AdvancedSettingKind;
  path: string[];
  valueType: "number" | "select";
  fallbackValue: number | string;
  values?: string[];
  customValueKind?: AdvancedSettingCustomValueKind;
  min?: number;
  max?: number;
  step?: number;
};

export type AdvancedSettingItem = AdvancedSettingDefinition & {
  key: string;
  value: number | string;
};

const TEMPERATURE_SETTING: AdvancedSettingDefinition = {
  kind: "temperature",
  path: ["temperature"],
  valueType: "number",
  fallbackValue: 1,
  min: 0,
  max: 2,
  step: 0.1,
};

const CHAT_REASONING_EFFORT: AdvancedSettingDefinition = {
  kind: "reasoningEffort",
  path: ["reasoning_effort"],
  valueType: "select",
  fallbackValue: "medium",
  values: ["minimal", "low", "medium", "high", "xhigh"],
};

const RESPONSES_REASONING_EFFORT: AdvancedSettingDefinition = {
  kind: "reasoningEffort",
  path: ["reasoning", "effort"],
  valueType: "select",
  fallbackValue: "medium",
  values: ["none", "low", "medium", "high", "xhigh"],
};

const XAI_RESPONSES_REASONING_EFFORT: AdvancedSettingDefinition = {
  kind: "reasoningEffort",
  path: ["reasoning", "effort"],
  valueType: "select",
  fallbackValue: "medium",
  values: ["none", "low", "medium", "high"],
};

const ANTHROPIC_EFFORT: AdvancedSettingDefinition = {
  kind: "reasoningEffort",
  path: ["output_config", "effort"],
  valueType: "select",
  fallbackValue: "medium",
  values: ["low", "medium", "high", "xhigh", "max"],
};

const GEMINI_THINKING_LEVEL: AdvancedSettingDefinition = {
  kind: "reasoningEffort",
  path: ["thinkingConfig", "thinkingLevel"],
  valueType: "select",
  fallbackValue: "medium",
  values: ["minimal", "low", "medium", "high"],
};

const CHAT_VERBOSITY: AdvancedSettingDefinition = {
  kind: "verbosity",
  path: ["verbosity"],
  valueType: "select",
  fallbackValue: "medium",
  values: ["low", "medium", "high"],
};

const RESPONSES_VERBOSITY: AdvancedSettingDefinition = {
  kind: "verbosity",
  path: ["text", "verbosity"],
  valueType: "select",
  fallbackValue: "medium",
  values: ["low", "medium", "high"],
};

const OPENAI_GPT_IMAGE_QUALITY_VALUES = ["auto", "low", "medium", "high"];
const OPENAI_GPT_IMAGE_2_SIZE_VALUES = [
  "auto",
  "1024x1024",
  "1536x1024",
  "1024x1536",
  "2048x2048",
  "2048x1152",
  "3840x2160",
  "2160x3840",
];

const OPENAI_IMAGE_QUALITY: AdvancedSettingDefinition = {
  kind: "imageQuality",
  path: ["quality"],
  valueType: "select",
  fallbackValue: "auto",
  values: OPENAI_GPT_IMAGE_QUALITY_VALUES,
};

const OPENAI_IMAGE_2_RESOLUTION: AdvancedSettingDefinition = {
  kind: "imageResolution",
  path: ["size"],
  valueType: "select",
  fallbackValue: "auto",
  values: OPENAI_GPT_IMAGE_2_SIZE_VALUES,
  customValueKind: "openaiImage2Resolution",
};

function isPlainOptionObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

export function advancedSettingPathKey(path: string[]): string {
  return path.join(".");
}

function getOptionAtPath(options: ConversationOptions, path: string[]): unknown {
  let current: unknown = options;
  for (const segment of path) {
    if (!isPlainOptionObject(current)) {
      return undefined;
    }
    current = current[segment];
  }
  return current;
}

function setOptionAtPath(options: ConversationOptions, path: string[], value: unknown): ConversationOptions {
  if (path.length === 0) {
    return options;
  }
  const [segment, ...rest] = path;
  if (rest.length === 0) {
    return { ...options, [segment]: value };
  }
  const current = options[segment];
  return {
    ...options,
    [segment]: setOptionAtPath(isPlainOptionObject(current) ? current : {}, rest, value),
  };
}

function removeOptionAtPath(options: ConversationOptions, path: string[]): ConversationOptions {
  if (path.length === 0) {
    return options;
  }
  const [segment, ...rest] = path;
  if (!(segment in options)) {
    return options;
  }
  if (rest.length === 0) {
    const { [segment]: _removed, ...remaining } = options;
    return remaining;
  }
  const current = options[segment];
  if (!isPlainOptionObject(current)) {
    return options;
  }
  const nextChild = removeOptionAtPath(current, rest);
  if (Object.keys(nextChild).length === 0) {
    const { [segment]: _removed, ...remaining } = options;
    return remaining;
  }
  return { ...options, [segment]: nextChild };
}

function numericValue(value: unknown): number | null {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
}

export function isGemini3PlusModel(modelName: string): boolean {
  const normalized = modelName.trim().toLowerCase();
  const match = normalized.match(/gemini[^0-9]*(\d+)(?:[._-](\d+))?/);
  if (!match) {
    return false;
  }
  return Number(match[1]) >= 3;
}

export function isValidOpenAIImage2Resolution(value: string): boolean {
  const match = value.trim().match(/^(\d+)x(\d+)$/i);
  if (!match) {
    return false;
  }
  const width = Number(match[1]);
  const height = Number(match[2]);
  if (!Number.isInteger(width) || !Number.isInteger(height) || width <= 0 || height <= 0) {
    return false;
  }
  if (width > 3840 || height > 3840 || width % 16 !== 0 || height % 16 !== 0) {
    return false;
  }
  const longEdge = Math.max(width, height);
  const shortEdge = Math.min(width, height);
  const pixels = width * height;
  return longEdge / shortEdge <= 3 && pixels >= 655360 && pixels <= 8294400;
}

function isCustomValueValid(kind: AdvancedSettingCustomValueKind, value: string): boolean {
  switch (kind) {
    case "openaiImage2Resolution":
      return isValidOpenAIImage2Resolution(value);
  }
}

export function isAdvancedSettingCustomValueValid(
  setting: Pick<AdvancedSettingDefinition, "customValueKind">,
  value: string,
): boolean {
  return setting.customValueKind ? isCustomValueValid(setting.customValueKind, value) : true;
}

function selectValue(
  value: unknown,
  values: string[] | undefined,
  customValueKind?: AdvancedSettingCustomValueKind,
): string | null {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  if (!trimmed) {
    return null;
  }
  if (values?.includes(trimmed)) {
    return trimmed;
  }
  if (customValueKind && isCustomValueValid(customValueKind, trimmed)) {
    return trimmed;
  }
  if (values) {
    return null;
  }
  return trimmed;
}

function valueForDefinition(
  definition: AdvancedSettingDefinition,
  options: ConversationOptions,
  defaultOptions: ConversationOptions,
): number | string {
  const explicit = getOptionAtPath(options, definition.path);
  const defaultValue = getOptionAtPath(defaultOptions, definition.path);
  if (definition.valueType === "number") {
    return numericValue(explicit) ?? numericValue(defaultValue) ?? Number(definition.fallbackValue);
  }
  return selectValue(explicit, definition.values, definition.customValueKind) ??
    selectValue(defaultValue, definition.values, definition.customValueKind) ??
    String(definition.fallbackValue);
}

export function resolveAdvancedSettingDefinitions(protocol: string, modelName = ""): AdvancedSettingDefinition[] {
  switch (protocol.trim()) {
    case "openai_chat_completions":
      return [TEMPERATURE_SETTING, CHAT_REASONING_EFFORT, CHAT_VERBOSITY];
    case "openai_responses":
      return [TEMPERATURE_SETTING, RESPONSES_REASONING_EFFORT, RESPONSES_VERBOSITY];
    case "xai_responses":
      return [TEMPERATURE_SETTING, XAI_RESPONSES_REASONING_EFFORT];
    case "anthropic_messages":
      return [TEMPERATURE_SETTING, ANTHROPIC_EFFORT];
    case "openai_image_generations":
    case "openai_image_edits":
      return [OPENAI_IMAGE_QUALITY, OPENAI_IMAGE_2_RESOLUTION];
    case "gemini_generate_content":
    case "google_generate_content":
      return isGemini3PlusModel(modelName) ? [TEMPERATURE_SETTING, GEMINI_THINKING_LEVEL] : [];
    default:
      return [];
  }
}

function isSettingFiltered(
  definition: AdvancedSettingDefinition,
  policy: ModelOptionPolicy | null,
  protocol: string,
): boolean {
  if (!policy) {
    return false;
  }
  return isModelOptionPathFiltered({
    policy,
    protocol,
    path: advancedSettingPathKey(definition.path),
  });
}

export function resolveAdvancedSettings({
  protocol,
  options,
  defaultOptions,
  policy,
  modelName = "",
}: {
  protocol: string;
  modelName?: string;
  options: ConversationOptions;
  defaultOptions: ConversationOptions;
  policy: ModelOptionPolicy | null;
}): AdvancedSettingItem[] {
  return resolveAdvancedSettingDefinitions(protocol, modelName)
    .filter((definition) => !isSettingFiltered(definition, policy, protocol))
    .map((definition) => ({
      ...definition,
      key: advancedSettingPathKey(definition.path),
      value: valueForDefinition(definition, options, defaultOptions),
    }));
}

export function setAdvancedSettingValue(
  options: ConversationOptions,
  setting: Pick<AdvancedSettingDefinition, "path" | "valueType" | "values" | "customValueKind" | "min" | "max">,
  value: number | string,
): ConversationOptions {
  if (setting.valueType === "number") {
    const numeric = numericValue(value);
    if (numeric === null) {
      return options;
    }
    const min = setting.min ?? Number.NEGATIVE_INFINITY;
    const max = setting.max ?? Number.POSITIVE_INFINITY;
    return setOptionAtPath(options, setting.path, Math.min(max, Math.max(min, numeric)));
  }
  const selected = selectValue(value, setting.values, setting.customValueKind);
  if (!selected) {
    return options;
  }
  return setOptionAtPath(options, setting.path, selected);
}

export function resetAdvancedSettings(
  options: ConversationOptions,
  defaultOptions: ConversationOptions,
  protocol: string,
  policy: ModelOptionPolicy | null,
  modelName = "",
): ConversationOptions {
  return resolveAdvancedSettings({ protocol, modelName, options, defaultOptions, policy }).reduce((current, setting) => {
    const withoutCurrentValue = removeOptionAtPath(current, setting.path);
    const defaultValue = getOptionAtPath(defaultOptions, setting.path);
    if (setting.valueType === "number") {
      const numeric = numericValue(defaultValue);
      return numeric === null ? withoutCurrentValue : setOptionAtPath(withoutCurrentValue, setting.path, numeric);
    }
    const selected = selectValue(defaultValue, setting.values, setting.customValueKind);
    return selected ? setOptionAtPath(withoutCurrentValue, setting.path, selected) : withoutCurrentValue;
  }, options);
}
