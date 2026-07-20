import type { ConversationOptions } from "@/shared/api/conversation.types";
import type { ModelOptionPolicy } from "../../../shared/lib/model-option-policy.ts";
import { isModelOptionPathFiltered } from "../../../shared/lib/model-option-policy.ts";
import type { ChatSubmitTask } from "./chat-task.ts";

export type AdvancedSettingKind =
  | "temperature"
  | "reasoningMode"
  | "reasoningEffort"
  | "reasoningSummary"
  | "verbosity"
  | "outputFormat"
  | "speed"
  | "imageQuality"
  | "imageResolution"
  | "imageAspectRatio"
  | "imageSize"
  | "responseFormat"
  | "thinkingLevel"
  | "videoResolution"
  | "videoSeconds"
  | "videoTask";
export type AdvancedSettingCustomValueKind = "openaiImage2Resolution";

export type AdvancedSettingDefinition = {
  kind: AdvancedSettingKind;
  path: string[];
  valueType: "number" | "select";
  fallbackValue: number | string;
  values?: string[];
  omitValues?: string[];
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

const CHAT_REASONING_EFFORT_VALUES = ["none", "low", "medium", "high", "xhigh", "max"];
const CHAT_REASONING_SUMMARY_VALUES = ["none", "auto", "concise", "detailed"];
const CHAT_VERBOSITY_VALUES = ["none", "low", "medium", "high"];

const OPENAI_CHAT_REASONING_EFFORT: AdvancedSettingDefinition = {
  kind: "reasoningEffort",
  path: ["reasoning_effort"],
  valueType: "select",
  fallbackValue: "none",
  values: CHAT_REASONING_EFFORT_VALUES,
  omitValues: ["none"],
};

const OPENAI_CHAT_REASONING_SUMMARY: AdvancedSettingDefinition = {
  kind: "reasoningSummary",
  path: ["reasoning_summary"],
  valueType: "select",
  fallbackValue: "none",
  values: CHAT_REASONING_SUMMARY_VALUES,
  omitValues: ["none"],
};

const OPENROUTER_CHAT_REASONING_EFFORT: AdvancedSettingDefinition = {
  kind: "reasoningEffort",
  path: ["reasoning", "effort"],
  valueType: "select",
  fallbackValue: "none",
  values: CHAT_REASONING_EFFORT_VALUES,
  omitValues: ["none"],
};

const OPENROUTER_CHAT_REASONING_SUMMARY: AdvancedSettingDefinition = {
  kind: "reasoningSummary",
  path: ["reasoning", "summary"],
  valueType: "select",
  fallbackValue: "none",
  values: CHAT_REASONING_SUMMARY_VALUES,
  omitValues: ["none"],
};

const CHAT_VERBOSITY: AdvancedSettingDefinition = {
  kind: "verbosity",
  path: ["verbosity"],
  valueType: "select",
  fallbackValue: "none",
  values: CHAT_VERBOSITY_VALUES,
  omitValues: ["none"],
};

const RESPONSES_REASONING_EFFORT: AdvancedSettingDefinition = {
  kind: "reasoningEffort",
  path: ["reasoning", "effort"],
  valueType: "select",
  fallbackValue: "medium",
  values: ["none", "low", "medium", "high", "xhigh", "max"],
};

const RESPONSES_REASONING_MODE: AdvancedSettingDefinition = {
  kind: "reasoningMode",
  path: ["reasoning", "mode"],
  valueType: "select",
  fallbackValue: "standard",
  values: ["standard", "pro"],
  omitValues: ["standard"],
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
  fallbackValue: "high",
  values: ["low", "medium", "high", "xhigh", "max"],
};

const ANTHROPIC_SPEED: AdvancedSettingDefinition = {
  kind: "speed",
  path: ["speed"],
  valueType: "select",
  fallbackValue: "standard",
  values: ["standard", "fast"],
  omitValues: ["standard"],
};

const GEMINI_THINKING_LEVEL: AdvancedSettingDefinition = {
  kind: "reasoningEffort",
  path: ["thinkingConfig", "thinkingLevel"],
  valueType: "select",
  fallbackValue: "medium",
  values: ["minimal", "low", "medium", "high"],
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

const OPENAI_IMAGE_OUTPUT_FORMAT: AdvancedSettingDefinition = {
  kind: "outputFormat",
  path: ["output_format"],
  valueType: "select",
  fallbackValue: "png",
  values: ["png", "jpeg", "webp"],
};

const OPENAI_IMAGE_2_RESOLUTION: AdvancedSettingDefinition = {
  kind: "imageResolution",
  path: ["size"],
  valueType: "select",
  fallbackValue: "auto",
  values: OPENAI_GPT_IMAGE_2_SIZE_VALUES,
};

const GEMINI_IMAGE_RESOLUTION: AdvancedSettingDefinition = {
  kind: "imageResolution",
  path: ["generationConfig", "imageConfig", "imageSize"],
  valueType: "select",
  fallbackValue: "1K",
  values: ["512", "1K", "2K", "4K"],
};

const GEMINI_IMAGE_ASPECT_RATIO: AdvancedSettingDefinition = {
  kind: "imageAspectRatio",
  path: ["generationConfig", "imageConfig", "aspectRatio"],
  valueType: "select",
  fallbackValue: "auto",
  values: ["auto", "1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9", "1:4", "4:1", "1:8", "8:1"],
  omitValues: ["auto"],
};

const GEMINI_IMAGE_THINKING_LEVEL: AdvancedSettingDefinition = {
  kind: "reasoningEffort",
  path: ["generationConfig", "thinkingConfig", "thinkingLevel"],
  valueType: "select",
  fallbackValue: "high",
  values: ["minimal", "high"],
};

const XAI_IMAGE_ASPECT_RATIO: AdvancedSettingDefinition = {
  kind: "imageAspectRatio",
  path: ["aspect_ratio"],
  valueType: "select",
  fallbackValue: "auto",
  values: ["auto", "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "2:1", "1:2", "19.5:9", "9:19.5", "20:9", "9:20"],
  omitValues: ["auto"],
};

const XAI_IMAGE_RESOLUTION: AdvancedSettingDefinition = {
  kind: "imageResolution",
  path: ["resolution"],
  valueType: "select",
  fallbackValue: "1k",
  values: ["1k", "2k"],
};

const XAI_IMAGE_RESPONSE_FORMAT: AdvancedSettingDefinition = {
  kind: "responseFormat",
  path: ["response_format"],
  valueType: "select",
  fallbackValue: "b64_json",
  values: ["url", "b64_json"],
};

const GEMINI_INTERACTIONS_THINKING_LEVEL: AdvancedSettingDefinition = {
  kind: "thinkingLevel",
  path: ["generation_config", "thinking_level"],
  valueType: "select",
  fallbackValue: "high",
  values: ["minimal", "low", "medium", "high"],
};

const GEMINI_INTERACTIONS_IMAGE_SIZE: AdvancedSettingDefinition = {
  kind: "imageSize",
  path: ["response_format", "image_size"],
  valueType: "select",
  fallbackValue: "1K",
  values: ["512", "1K", "2K", "4K"],
};

const GEMINI_INTERACTIONS_VIDEO_TASK: AdvancedSettingDefinition = {
  kind: "videoTask",
  path: ["generation_config", "video_config", "task"],
  valueType: "select",
  fallbackValue: "auto",
  values: ["auto", "text_to_video", "image_to_video", "reference_to_video", "edit"],
  omitValues: ["auto"],
};

const OPENAI_VIDEO_SECONDS: AdvancedSettingDefinition = {
  kind: "videoSeconds",
  path: ["seconds"],
  valueType: "select",
  fallbackValue: "4",
  values: ["4", "8", "12"],
};

const OPENAI_VIDEO_SIZE_BASE_VALUES = ["720x1280", "1280x720"];
const OPENAI_VIDEO_SIZE_PRO_VALUES = [
  "720x1280",
  "1280x720",
  "1024x1792",
  "1792x1024",
  "1080x1920",
  "1920x1080",
];

function openAIVideoResolutionSetting(modelName: string): AdvancedSettingDefinition {
  return {
    kind: "videoResolution",
    path: ["size"],
    valueType: "select",
    fallbackValue: "1280x720",
    values: modelName.trim().toLowerCase().includes("sora-2-pro")
      ? OPENAI_VIDEO_SIZE_PRO_VALUES
      : OPENAI_VIDEO_SIZE_BASE_VALUES,
  };
}

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

export function resolveAdvancedSettingDefinitions(
  protocol: string,
  modelName = "",
  submitTask: ChatSubmitTask = "chat",
): AdvancedSettingDefinition[] {
  switch (protocol.trim()) {
    case "openai_chat_completions":
      return [
        TEMPERATURE_SETTING,
        OPENAI_CHAT_REASONING_EFFORT,
        OPENAI_CHAT_REASONING_SUMMARY,
        CHAT_VERBOSITY,
      ];
    case "openrouter_chat_completions":
      return [
        TEMPERATURE_SETTING,
        OPENROUTER_CHAT_REASONING_EFFORT,
        OPENROUTER_CHAT_REASONING_SUMMARY,
        CHAT_VERBOSITY,
      ];
    case "openai_responses":
      return [TEMPERATURE_SETTING, RESPONSES_REASONING_MODE, RESPONSES_REASONING_EFFORT, RESPONSES_VERBOSITY];
    case "openrouter_responses":
      return [TEMPERATURE_SETTING, RESPONSES_REASONING_EFFORT];
    case "xai_responses":
      return [TEMPERATURE_SETTING, XAI_RESPONSES_REASONING_EFFORT];
    case "anthropic_messages":
      return [TEMPERATURE_SETTING, ANTHROPIC_EFFORT, ANTHROPIC_SPEED];
    case "openai_image_generations":
      return [OPENAI_IMAGE_QUALITY, OPENAI_IMAGE_2_RESOLUTION, OPENAI_IMAGE_OUTPUT_FORMAT];
    case "openai_image_edits":
      return [OPENAI_IMAGE_QUALITY, OPENAI_IMAGE_2_RESOLUTION];
    case "openai_video_generations":
      return [openAIVideoResolutionSetting(modelName), OPENAI_VIDEO_SECONDS];
    case "google_image_generation":
      return [GEMINI_IMAGE_RESOLUTION, GEMINI_IMAGE_ASPECT_RATIO, GEMINI_IMAGE_THINKING_LEVEL];
    case "gemini_generate_content":
    case "google_generate_content":
      return isGemini3PlusModel(modelName) ? [TEMPERATURE_SETTING, GEMINI_THINKING_LEVEL] : [];
    case "xai_image":
    case "xai_image_edits":
      return [XAI_IMAGE_ASPECT_RATIO, XAI_IMAGE_RESOLUTION, XAI_IMAGE_RESPONSE_FORMAT];
    case "gemini_interactions":
      if (submitTask === "image_generation" || submitTask === "image_edit") {
        return [GEMINI_INTERACTIONS_THINKING_LEVEL, GEMINI_INTERACTIONS_IMAGE_SIZE];
      }
      if (submitTask === "video_generation") {
        return [GEMINI_INTERACTIONS_THINKING_LEVEL, GEMINI_INTERACTIONS_VIDEO_TASK];
      }
      return [GEMINI_INTERACTIONS_THINKING_LEVEL];
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
  submitTask = "chat",
}: {
  protocol: string;
  modelName?: string;
  submitTask?: ChatSubmitTask;
  options: ConversationOptions;
  defaultOptions: ConversationOptions;
  policy: ModelOptionPolicy | null;
}): AdvancedSettingItem[] {
  return resolveAdvancedSettingDefinitions(protocol, modelName, submitTask)
    .filter((definition) => !isSettingFiltered(definition, policy, protocol))
    .map((definition) => ({
      ...definition,
      key: advancedSettingPathKey(definition.path),
      value: valueForDefinition(definition, options, defaultOptions),
    }));
}

export function setAdvancedSettingValue(
  options: ConversationOptions,
  setting: Pick<AdvancedSettingDefinition, "path" | "valueType" | "values" | "omitValues" | "customValueKind" | "min" | "max">,
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
  if (setting.omitValues?.includes(selected)) {
    return removeOptionAtPath(options, setting.path);
  }
  return setOptionAtPath(options, setting.path, selected);
}

export function resetAdvancedSettings(
  options: ConversationOptions,
  defaultOptions: ConversationOptions,
  protocol: string,
  policy: ModelOptionPolicy | null,
  modelName = "",
  submitTask: ChatSubmitTask = "chat",
): ConversationOptions {
  return resolveAdvancedSettings({ protocol, modelName, submitTask, options, defaultOptions, policy }).reduce((current, setting) => {
    const withoutCurrentValue = removeOptionAtPath(current, setting.path);
    const defaultValue = getOptionAtPath(defaultOptions, setting.path);
    if (setting.valueType === "number") {
      const numeric = numericValue(defaultValue);
      return numeric === null ? withoutCurrentValue : setOptionAtPath(withoutCurrentValue, setting.path, numeric);
    }
    const selected = selectValue(defaultValue, setting.values, setting.customValueKind);
    return selected && !setting.omitValues?.includes(selected)
      ? setOptionAtPath(withoutCurrentValue, setting.path, selected)
      : withoutCurrentValue;
  }, options);
}
