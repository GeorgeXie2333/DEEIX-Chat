import type { ConversationOptions } from "@/shared/api/conversation.types";
import type { ModelOptionPolicy } from "../../../shared/lib/model-option-policy.ts";
import { isModelOptionPathFiltered } from "../../../shared/lib/model-option-policy.ts";

export type AdvancedSettingKind = "temperature" | "reasoningEffort" | "verbosity";

export type AdvancedSettingDefinition = {
  kind: AdvancedSettingKind;
  path: string[];
  valueType: "number" | "select";
  fallbackValue: number | string;
  values?: string[];
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
  values: ["low", "medium", "high"],
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

function selectValue(value: unknown, values: string[] | undefined): string | null {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  if (!trimmed) {
    return null;
  }
  if (values && !values.includes(trimmed)) {
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
  return selectValue(explicit, definition.values) ??
    selectValue(defaultValue, definition.values) ??
    String(definition.fallbackValue);
}

export function resolveAdvancedSettingDefinitions(protocol: string): AdvancedSettingDefinition[] {
  switch (protocol.trim()) {
    case "openai_chat_completions":
      return [TEMPERATURE_SETTING, CHAT_REASONING_EFFORT, CHAT_VERBOSITY];
    case "openai_responses":
      return [TEMPERATURE_SETTING, RESPONSES_REASONING_EFFORT, RESPONSES_VERBOSITY];
    case "xai_responses":
      return [TEMPERATURE_SETTING, RESPONSES_REASONING_EFFORT];
    case "anthropic_messages":
    case "gemini_generate_content":
    case "google_generate_content":
      return [TEMPERATURE_SETTING];
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
}: {
  protocol: string;
  options: ConversationOptions;
  defaultOptions: ConversationOptions;
  policy: ModelOptionPolicy | null;
}): AdvancedSettingItem[] {
  return resolveAdvancedSettingDefinitions(protocol)
    .filter((definition) => !isSettingFiltered(definition, policy, protocol))
    .map((definition) => ({
      ...definition,
      key: advancedSettingPathKey(definition.path),
      value: valueForDefinition(definition, options, defaultOptions),
    }));
}

export function setAdvancedSettingValue(
  options: ConversationOptions,
  setting: Pick<AdvancedSettingDefinition, "path" | "valueType" | "values" | "min" | "max">,
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
  const selected = selectValue(value, setting.values);
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
): ConversationOptions {
  return resolveAdvancedSettings({ protocol, options, defaultOptions, policy }).reduce((current, setting) => {
    const withoutCurrentValue = removeOptionAtPath(current, setting.path);
    const defaultValue = getOptionAtPath(defaultOptions, setting.path);
    if (setting.valueType === "number") {
      const numeric = numericValue(defaultValue);
      return numeric === null ? withoutCurrentValue : setOptionAtPath(withoutCurrentValue, setting.path, numeric);
    }
    const selected = selectValue(defaultValue, setting.values);
    return selected ? setOptionAtPath(withoutCurrentValue, setting.path, selected) : withoutCurrentValue;
  }, options);
}
