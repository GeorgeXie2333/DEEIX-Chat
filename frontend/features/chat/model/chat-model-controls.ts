import type { ConversationOptions } from "../../../shared/api/conversation.types.ts";
import {
  isModelOptionPathFiltered,
  type ModelOptionPolicy,
} from "../../../shared/lib/model-option-policy.ts";
import type { ModelOptionControl } from "../types/chat-runtime.ts";
import { isReservedConversationOptionKey } from "./conversation-options.ts";

export type ServerOptionControlValueType = "number" | "select" | "text" | "boolean";

export type ResolvedServerOptionControl = {
  key: string;
  path: string[];
  valueType: ServerOptionControlValueType;
  value: number | string | boolean;
  defaultValue: number | string | boolean | undefined;
  options: string[];
  label: string;
  description: string;
  placeholder?: string;
  locked: boolean;
};

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

export function optionPathSegments(path: string): string[] {
  return path
    .split(".")
    .map((segment) => segment.trim())
    .filter(Boolean);
}

export function optionPathKey(path: string[]): string {
  return path.join(".");
}

export function getOptionAtPath(options: ConversationOptions, path: string[]): unknown {
  let current: unknown = options;
  for (const segment of path) {
    if (!isPlainObject(current)) {
      return undefined;
    }
    current = current[segment];
  }
  return current;
}

export function setOptionAtPath(
  options: ConversationOptions,
  path: string[],
  value: unknown,
): ConversationOptions {
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
    [segment]: setOptionAtPath(isPlainObject(current) ? current : {}, rest, value),
  };
}

export function removeOptionAtPath(options: ConversationOptions, path: string[]): ConversationOptions {
  if (path.length === 0) {
    return options;
  }
  const [segment, ...rest] = path;
  if (!Object.hasOwn(options, segment)) {
    return options;
  }
  if (rest.length === 0) {
    const { [segment]: _removed, ...remaining } = options;
    return remaining;
  }
  const current = options[segment];
  if (!isPlainObject(current)) {
    return options;
  }
  const nextChild = removeOptionAtPath(current, rest);
  if (Object.keys(nextChild).length === 0) {
    const { [segment]: _removed, ...remaining } = options;
    return remaining;
  }
  return { ...options, [segment]: nextChild };
}

function normalizeType(value: unknown): ServerOptionControlValueType | undefined {
  return value === "number" || value === "select" || value === "text" || value === "boolean"
    ? value
    : undefined;
}

function normalizeOptions(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return Array.from(new Set(value
    .map((item) => typeof item === "string" ? item.trim() : "")
    .filter(Boolean)));
}

function normalizeText(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function inferType(
  explicitType: ServerOptionControlValueType | undefined,
  options: string[],
  current: unknown,
  defaultValue: unknown,
): ServerOptionControlValueType {
  if (explicitType === "select" && options.length === 0) {
    return "text";
  }
  if (explicitType) {
    return explicitType;
  }
  if (options.length > 0) {
    return "select";
  }
  if (typeof current === "boolean" || typeof defaultValue === "boolean") {
    return "boolean";
  }
  if (typeof current === "number" || typeof defaultValue === "number") {
    return "number";
  }
  return "text";
}

function numericControlValue(value: unknown, fallback: unknown): number | string {
  if (value === "") {
    return "";
  }
  const parsed = typeof value === "number" ? value : Number(value);
  if (Number.isFinite(parsed)) {
    return parsed;
  }
  const fallbackNumber = typeof fallback === "number" ? fallback : Number(fallback);
  return Number.isFinite(fallbackNumber) ? fallbackNumber : 0;
}

function booleanControlValue(value: unknown, fallback: unknown): boolean {
  if (typeof value === "boolean") {
    return value;
  }
  if (typeof value === "string") {
    if (value.trim().toLowerCase() === "true") return true;
    if (value.trim().toLowerCase() === "false") return false;
  }
  if (typeof value === "number") {
    return value !== 0;
  }
  return fallback === true;
}

function selectControlValue(value: unknown, fallback: unknown, options: string[]): string {
  const preferred = value === undefined || value === null ? "" : String(value);
  if (preferred && options.includes(preferred)) {
    return preferred;
  }
  const fallbackValue = fallback === undefined || fallback === null ? "" : String(fallback);
  if (fallbackValue && options.includes(fallbackValue)) {
    return fallbackValue;
  }
  return options[0] ?? preferred;
}

export function resolveServerOptionControls({
  controls,
  options,
  defaultOptions,
  lockedOptionPaths = [],
  policy,
  protocol,
  excludedPaths = [],
}: {
  controls: ModelOptionControl[];
  options: ConversationOptions;
  defaultOptions: ConversationOptions;
  lockedOptionPaths?: string[];
  policy: ModelOptionPolicy | null;
  protocol: string;
  excludedPaths?: string[];
}): ResolvedServerOptionControl[] {
  const locked = new Set(lockedOptionPaths.map(optionPathSegments).map(optionPathKey));
  const excluded = new Set(excludedPaths.map(optionPathSegments).map(optionPathKey));
  const seen = new Set<string>();

  return controls.flatMap((control) => {
    const path = optionPathSegments(control.path);
    const key = optionPathKey(path);
    if (path.length === 0 || isReservedConversationOptionKey(path[0] ?? "") || seen.has(key) || excluded.has(key)) {
      return [];
    }
    if (policy && isModelOptionPathFiltered({ policy, protocol, path: key })) {
      return [];
    }

    seen.add(key);
    const explicitValue = getOptionAtPath(options, path);
    const defaultValue = getOptionAtPath(defaultOptions, path);
    const values = normalizeOptions(control.options);
    const valueType = inferType(normalizeType(control.type), values, explicitValue, defaultValue);
    const isLocked = control.locked === true || locked.has(key);
    const valueSource = isLocked && defaultValue !== undefined
      ? defaultValue
      : explicitValue !== undefined
        ? explicitValue
        : defaultValue;
    const value = valueType === "number"
      ? numericControlValue(valueSource, defaultValue)
      : valueType === "boolean"
        ? booleanControlValue(valueSource, defaultValue)
        : valueType === "select"
          ? selectControlValue(valueSource, defaultValue, values)
          : valueSource === undefined || valueSource === null ? "" : String(valueSource);
    const normalizedDefaultValue = defaultValue === undefined || defaultValue === null
      ? undefined
      : typeof defaultValue === "string" || typeof defaultValue === "number" || typeof defaultValue === "boolean"
        ? defaultValue
        : String(defaultValue);

    return [{
      key,
      path,
      valueType,
      value,
      defaultValue: normalizedDefaultValue,
      options: values,
      label: normalizeText(control.label) || key,
      description: normalizeText(control.description),
      placeholder: normalizeText(control.placeholder) || undefined,
      locked: isLocked,
    }];
  });
}
