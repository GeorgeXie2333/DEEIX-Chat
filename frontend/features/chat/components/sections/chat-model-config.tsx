"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { Cog } from "@/components/animate-ui/icons/cog";
import { Input } from "@/components/ui/input";
import { InputGroupButton } from "@/components/ui/input-group";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import {
  isAdvancedSettingCustomValueValid,
  resetAdvancedSettings,
  resolveAdvancedSettings,
  setAdvancedSettingValue,
  type AdvancedSettingItem,
} from "@/features/chat/model/advanced-settings";
import type { ConversationOptions } from "@/shared/api/conversation.types";
import type { ModelOptionPolicy } from "@/shared/lib/model-option-policy";

type ChatModelConfigProps = {
  disabled: boolean;
  options: ConversationOptions;
  defaultOptions: ConversationOptions;
  optionControls?: unknown[];
  nativeToolKeys?: string[];
  nativeTools?: unknown[];
  modelOptionPolicy: ModelOptionPolicy | null;
  selectedProtocol: string;
  selectedModelName: string;
  onOptionsChange: React.Dispatch<React.SetStateAction<ConversationOptions>>;
  onOptionsReset: (defaults?: ConversationOptions) => void;
  onDefaultOptionsRestore?: () => Promise<ConversationOptions | null>;
};

const CUSTOM_SELECT_VALUE = "__custom__";

function formatTemperature(value: number | string): string {
  const numeric = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(numeric)) {
    return "1";
  }
  return Number(numeric.toFixed(1)).toString();
}

function labelKeyForSetting(setting: AdvancedSettingItem): string {
  switch (setting.kind) {
    case "temperature":
      return "temperature";
    case "reasoningMode":
      return "reasoning_mode";
    case "reasoningEffort":
      return "reasoning_effort";
    case "verbosity":
      return "verbosity";
    case "imageQuality":
      return "quality";
    case "imageResolution":
      return "resolution";
    case "imageAspectRatio":
      return "aspect_ratio";
    case "videoResolution":
      return "videoResolution";
    case "videoSeconds":
      return "videoSeconds";
  }
}

function collectOptionValueEntries(value: unknown, path: string[]): OptionValueEntry[] {
  if (path.length === 0) {
    return [];
  }
  if (!isPlainOptionObject(value)) {
    return [{ key: optionPathKey(path), path, value }];
  }
  const entries = Object.entries(value).flatMap(([key, nestedValue]) => collectOptionValueEntries(nestedValue, [...path, key]));
  if (entries.length === 0) {
    return [{ key: optionPathKey(path), path, value }];
  }
  return entries;
}

function optionValueEntriesFromOptions(options: ConversationOptions): OptionValueEntry[] {
  return Object.entries(options).flatMap(([key, value]) => {
    if (key === "tools") {
      return [{ key, path: [key], value }];
    }
    return collectOptionValueEntries(value, [key]);
  });
}

function formatVisualOptionValue(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean" || value === null) {
    return String(value);
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function isEditableOptionValue(value: unknown): value is EditableOptionValue {
  return value === null || ["string", "number", "boolean"].includes(typeof value);
}

function isPlainOptionObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function providerToolObjectsFromOptions(options: ConversationOptions): Record<string, unknown>[] {
  const rawTools = options.tools;
  if (!Array.isArray(rawTools)) {
    return [];
  }
  return rawTools.filter(isPlainOptionObject);
}

function providerToolMatchesDefinition(tool: Record<string, unknown>, definition: NativeToolDefinition): boolean {
  const toolType = typeof tool.type === "string" ? tool.type.trim() : "";
  if (toolType) {
    return toolType === definition.type;
  }
  return Object.keys(definition.payload ?? {}).some((key) => key !== "type" && Object.hasOwn(tool, key));
}

function nativeToolDefinitionsFromKeys(
  toolKeys: string[],
  catalog: NativeToolDefinition[],
): NativeToolDefinition[] {
  const allowedKeys = new Set(toolKeys.map((key) => key.trim()).filter(Boolean));
  return catalog.filter((tool) => allowedKeys.has(tool.toolKey.trim()));
}

function nativeToolConfigPayloadType(config: ModelNativeToolConfig): string {
  return typeof config.payload.type === "string" ? config.payload.type.trim() : "";
}

function nativeToolDefinitionFromConfig(
  config: ModelNativeToolConfig,
  catalog: NativeToolDefinition[],
  selectedProtocol: string,
): NativeToolDefinition | null {
  const key = config.key.trim();
  const protocols = config.protocols.length > 0 ? config.protocols : (config.protocol.trim() ? [config.protocol.trim()] : []);
  const type = config.type.trim() || nativeToolConfigPayloadType(config);
  const policyProtocol = selectedProtocol ? resolveModelOptionPolicyProtocol(selectedProtocol) : "";
  const matched = (key && policyProtocol && protocols.includes(policyProtocol) ? catalog.find((tool) => tool.toolKey === key && tool.protocol === policyProtocol) : undefined)
    ?? (key && protocols.length > 0 ? catalog.find((tool) => tool.toolKey === key && protocols.includes(tool.protocol)) : undefined)
    ?? (key && policyProtocol ? catalog.find((tool) => tool.toolKey === key && tool.protocol === policyProtocol) : undefined)
    ?? catalog.find((tool) => tool.toolKey === key)
    ?? (type && policyProtocol && (protocols.length === 0 || protocols.includes(policyProtocol)) ? catalog.find((tool) => tool.protocol === policyProtocol && tool.type === type) : undefined)
    ?? (type && protocols.length > 0 ? catalog.find((tool) => protocols.includes(tool.protocol) && tool.type === type) : undefined)
    ?? (!policyProtocol && type ? catalog.find((tool) => tool.type === type) : undefined);
  if (!matched && !key && !type && Object.keys(config.payload).length === 0) {
    return null;
  }
  return {
    protocol: matched?.protocol || protocols[0] || selectedProtocol,
    provider: config.provider || matched?.provider || "Provider",
    type: type || matched?.type || key,
    toolKey: key || matched?.toolKey || type,
    label: config.label || matched?.label || type || key,
    description: config.description || matched?.description || type || key,
    payload: Object.keys(config.payload).length > 0 ? config.payload : (matched?.payload ?? {}),
    defaultEnabled: config.defaultEnabled,
    billable: matched?.billable ?? false,
    billingUnit: matched?.billingUnit ?? "",
    priceNanousd: matched?.priceNanousd ?? 0,
    priceLabel: matched?.priceLabel ?? "",
    riskLevel: matched?.riskLevel ?? "",
    usageAliases: matched?.usageAliases ?? [],
  };
}

function nativeToolDefinitionsFromConfigs(
  configs: ModelNativeToolConfig[],
  fallbackToolKeys: string[],
  catalog: NativeToolDefinition[],
  selectedProtocol: string,
): NativeToolVisualOption[] {
  const sourceConfigs = configs.length > 0
    ? configs
    : nativeToolDefinitionsFromKeys(fallbackToolKeys, catalog).map((tool): ModelNativeToolConfig => ({
      id: `${tool.protocol}:${tool.toolKey}:${tool.type}`,
      key: tool.toolKey,
      protocol: tool.protocol,
      protocols: [tool.protocol],
      provider: tool.provider,
      type: tool.type,
      label: tool.label,
      description: tool.description,
      enabled: true,
      defaultEnabled: false,
      payload: tool.payload,
    }));
  return sourceConfigs.flatMap((config): NativeToolVisualOption[] => {
    if (!config.enabled) {
      return [];
    }
    const definition = nativeToolDefinitionFromConfig(config, catalog, selectedProtocol);
    if (!definition) {
      return [];
    }
    const matchingDefinitions = catalog.filter((tool) => tool.toolKey === definition.toolKey);
    const protocols = config.protocols.length > 0
      ? config.protocols
      : Array.from(new Set([
        config.protocol,
        definition.protocol,
        ...matchingDefinitions.map((tool) => tool.protocol).filter(Boolean),
      ].filter(Boolean)));
    return [{
      definition,
      protocols,
      protocolMatched: !selectedProtocol || protocols.includes(resolveModelOptionPolicyProtocol(selectedProtocol)),
    }];
  });
}

function providerToolMatchesAnyDefinition(
  value: unknown,
  definitions: NativeToolDefinition[],
): boolean {
  if (!isPlainOptionObject(value)) {
    return false;
  }
  return definitions.some((definition) => providerToolMatchesDefinition(value, definition));
}

function ignoredProviderToolValues(
  value: unknown,
  definitions: NativeToolDefinition[],
): unknown[] {
  if (value === undefined) {
    return [];
  }
  if (!Array.isArray(value)) {
    return [value];
  }
  return value.filter((item) => !providerToolMatchesAnyDefinition(item, definitions));
}

function hasProviderTool(options: ConversationOptions, definition: NativeToolDefinition): boolean {
  return providerToolObjectsFromOptions(options).some((tool) => providerToolMatchesDefinition(tool, definition));
}

function setProviderToolEnabled(
  options: ConversationOptions,
  toolOption: NativeToolDefinition,
  enabled: boolean,
): ConversationOptions {
  const type = toolOption.type;
  const tools = providerToolObjectsFromOptions(options);
  const hasTool = tools.some((tool) => providerToolMatchesDefinition(tool, toolOption));
  const nextTools = enabled
    ? hasTool
      ? tools
      : [...tools, { ...(toolOption.payload ?? { type }) }]
    : tools.filter((tool) => !providerToolMatchesDefinition(tool, toolOption));

  if (nextTools.length === 0) {
    const { tools: _tools, ...rest } = options;
    return rest;
  }

  return { ...options, tools: nextTools };
}

function optionPathKey(path: string[]): string {
  return path.join(".");
}

function isIgnoredOptionPath(
  policy: ModelOptionPolicy | null,
  protocol: string,
  key: string,
  path: string[],
): boolean {
  if (isReservedConversationOptionKey(path[0] ?? "")) {
    return true;
  }
  return Boolean(policy && isModelOptionPathFiltered({ policy, protocol, path: key }));
}

function ignoredVisualOption(entry: OptionValueEntry, value: unknown): VisualOption {
  return {
    key: entry.key,
    path: entry.path,
    value,
    active: true,
    editable: false,
    forcedFilterStatus: "filtered",
  };
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

function hasOptionAtPath(options: ConversationOptions, path: string[]): boolean {
  let current: unknown = options;
  for (const segment of path) {
    if (!isPlainOptionObject(current) || !Object.hasOwn(current, segment)) {
      return false;
    }
    current = current[segment];
  }
  return true;
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

function applyLockedDefaultOptions(
  options: ConversationOptions,
  defaults: ConversationOptions,
  lockedPaths: string[],
): ConversationOptions {
  if (lockedPaths.length === 0) {
    return options;
  }
  return lockedPaths.reduce((nextOptions, key) => {
    const path = optionPathFromControl(key);
    if (path.length === 0) {
      return nextOptions;
    }
    const defaultValue = getOptionAtPath(defaults, path);
    return defaultValue === undefined ? nextOptions : setOptionAtPath(nextOptions, path, defaultValue);
  }, options);
}

function visualOptionsFromOptions(
  options: ConversationOptions,
  policy: ModelOptionPolicy | null,
  protocol: string,
  nativeToolDefinitions: NativeToolDefinition[],
): VisualOption[] {
  const nestedOptions = NESTED_VISUAL_OPTION_PATHS.flatMap((path): VisualOption[] => {
    if (isReservedConversationOptionKey(path[0] ?? "")) {
      return [];
    }
    const value = getOptionAtPath(options, path);
    if (!isEditableOptionValue(value)) {
      return [];
    }
    return [{ key: optionPathKey(path), path, value, active: true, editable: true }];
  });
  const topLevelOptions = Object.entries(options).flatMap(([key, value]): VisualOption[] => {
    if (isReservedConversationOptionKey(key)) {
      return [];
    }
    if (isEditableOptionValue(value)) {
      return [{ key, path: [key], value, active: true, editable: true }];
    }
    return [];
  });
  const editableOptions = [...nestedOptions, ...topLevelOptions]
    .filter((item, index, items) => items.findIndex((candidate) => candidate.key === item.key) === index);
  const visibleKeys = new Set(editableOptions.map((item) => item.key));
  const ignoredOptions = optionValueEntriesFromOptions(options).flatMap((entry): VisualOption[] => {
    if (visibleKeys.has(entry.key)) {
      return [];
    }
    if (entry.key === "tools") {
      const ignoredTools = ignoredProviderToolValues(entry.value, nativeToolDefinitions);
      return ignoredTools.length > 0 ? [ignoredVisualOption(entry, ignoredTools)] : [];
    }
    if (!isIgnoredOptionPath(policy, protocol, entry.key, entry.path)) {
      return [];
    }
    return [ignoredVisualOption(entry, entry.value)];
  });
  return [...editableOptions, ...ignoredOptions]
    .filter((item, index, items) => items.findIndex((candidate) => candidate.key === item.key) === index)
    .sort((left, right) => compareOptionKeys(left.key, right.key));
}

function optionPathFromControl(path: string): string[] {
  return path
    .split(".")
    .map((segment) => segment.trim())
    .filter(Boolean);
}

function resolveControlEditableValue(options: ConversationOptions, path: string[]): EditableOptionValue {
  const currentValue = getOptionAtPath(options, path);
  if (isEditableOptionValue(currentValue)) {
    return currentValue;
  }
  return null;
}

function normalizeControlSelectValues(values: string[] | undefined): string[] {
  if (!values) {
    return [];
  }
  return Array.from(new Set(values.map((item) => item.trim()).filter(Boolean)));
}

function resolveControlKind(control: ModelOptionControl): VisualOptionKind | undefined {
  if (control.type) {
    return control.type;
  }
  if (normalizeControlSelectValues(control.options).length > 0) {
    return "select";
  }
  return undefined;
}

function visualOptionsFromControls(
  controls: ModelOptionControl[],
  options: ConversationOptions,
  defaultOptions: ConversationOptions = {},
): VisualOption[] {
  return controls.flatMap((control): VisualOption[] => {
    const path = optionPathFromControl(control.path);
    if (path.length === 0 || isReservedConversationOptionKey(path[0] ?? "")) {
      return [];
    }
    const key = optionPathKey(path);
    const hasLockedDefault = Boolean(control.locked && hasOptionAtPath(defaultOptions, path));
    const value = hasLockedDefault
      ? resolveControlEditableValue(defaultOptions, path)
      : resolveControlEditableValue(options, path);
    const active = hasOptionAtPath(options, path) || hasLockedDefault;
    const selectValues = normalizeControlSelectValues(control.options);
    return [{
      key,
      path,
      value,
      active,
      label: control.label,
      description: control.description,
      kind: resolveControlKind(control),
      selectValues,
      placeholder: control.placeholder,
      editable: !control.locked,
      locked: control.locked,
    }];
  });
}

function hasVisualConfigurationContent({
  nativeToolDefinitions,
  optionControls,
  options,
  policy,
  protocol,
}: {
  nativeToolDefinitions: NativeToolDefinition[];
  optionControls: ModelOptionControl[];
  options: ConversationOptions;
  policy: ModelOptionPolicy | null;
  protocol: string;
}): boolean {
  if (nativeToolDefinitions.length > 0) {
    return true;
  }
  const configuredOptions = visualOptionsFromControls(optionControls, options);
  if (configuredOptions.length > 0) {
    return true;
  }
  const configuredKeys = new Set(configuredOptions.map((item) => item.key));
  return visualOptionsFromOptions(options, policy, protocol, nativeToolDefinitions)
    .some((item) => !configuredKeys.has(item.key));
}

function resolveOptionTitle(key: string, configuredLabel: string | undefined, translate: OptionTranslationResolver): string {
  const translationKey = key.replaceAll(".", "__");
  if (OPTION_LABEL_KEYS.has(key) && translate.has?.(translationKey)) {
    return translate(translationKey);
  }
  return configuredLabel?.trim() || key;
}

function resolveOptionDescription(key: string, description: string | undefined, translate: OptionTranslationResolver): string {
  const translationKey = key.replaceAll(".", "__");
  if (OPTION_DESCRIPTION_KEYS.has(key) && translate.has?.(translationKey)) {
    return translate(translationKey);
  }
  return description?.trim() || "";
}

function compareOptionKeys(a: string, b: string): number {
  const aIndex = OPTION_ORDER.indexOf(a);
  const bIndex = OPTION_ORDER.indexOf(b);
  if (aIndex >= 0 && bIndex >= 0) {
    return aIndex - bIndex;
  }
  if (aIndex >= 0) {
    return -1;
  }
  if (bIndex >= 0) {
    return 1;
  }
  return a.localeCompare(b);
}

function resolveOptionKind(key: string, value: EditableOptionValue): "boolean" | "number" | "select" | "text" {
  if (typeof value === "boolean") {
    return "boolean";
  }
  const selectValues = OPTION_SELECT_VALUES[key] ?? [];
  if (typeof value === "string" && selectValues.includes(value.trim())) {
    return "select";
  }
  if (typeof value === "number" || (value === null && NUMBER_OPTION_KEYS.has(key))) {
    return "number";
  }
  return "text";
}

function resolveSelectValues(key: string, configuredValues?: string[]): string[] {
  const sourceValues = configuredValues === undefined ? OPTION_SELECT_VALUES[key] : configuredValues;
  return Array.from(new Set((sourceValues ?? []).map((item) => item.trim()).filter(Boolean)));
}

function resolveModelOptionFilterStatus(
  policy: ModelOptionPolicy | null,
  protocol: string,
  path: string,
): ModelOptionFilterStatus {
  if (!policy) {
    return "unknown";
  }
  return isModelOptionPathFiltered({ policy, protocol, path }) ? "filtered" : "passed";
}

function ModelOptionFilterBadge({
  status,
  inactiveLabel,
  ignoredLabel,
  passedLabel,
}: {
  status: ModelOptionFilterStatus;
  inactiveLabel: string;
  ignoredLabel: string;
  passedLabel: string;
}) {
  if (status === "unknown") {
    return null;
  }
  return (
    <span
      data-filtered={status === "filtered"}
      data-inactive={status === "inactive"}
      className="shrink-0 rounded-md bg-emerald-500/10 px-1.5 py-0.5 text-[10px] leading-none text-emerald-700 data-[filtered=true]:bg-muted data-[filtered=true]:text-muted-foreground data-[inactive=true]:bg-muted data-[inactive=true]:text-muted-foreground"
    >
      {status === "inactive" ? inactiveLabel : status === "filtered" ? ignoredLabel : passedLabel}
    </span>
  );
}

function parseVisualNumberInput(value: string): number | string | null {
  const normalized = value.trim();
  if (!normalized) {
    return null;
  }
  if (/^[+-]?(?:\d+|\d*\.\d+)(?:e[+-]?\d+)?$/i.test(normalized)) {
    return Number(normalized);
  }
  return value;
}

function resolveProtocolLabel(protocol: string): string {
  return PROTOCOL_LABELS[protocol] ?? protocol;
}

function resolveNativeToolGroupTitle(provider: string, fallback: string, tComposer: (key: string) => string): string {
  switch (provider.trim().toLowerCase()) {
    case "anthropic":
      return tComposer("nativeTools.claude");
    case "google":
      return tComposer("nativeTools.google");
    case "openai":
      return tComposer("nativeTools.openai");
    case "xai":
      return tComposer("nativeTools.grok");
    default:
      return fallback;
  }
}

function resolveNativeToolLabel(tool: NativeToolDefinition, messages: unknown): string {
  return localizedNativeToolText(messages, "nativeToolLabels", tool.toolKey) || tool.label || tool.type || tool.toolKey;
}

function resolveNativeToolDescription(tool: NativeToolDefinition, messages: unknown): string {
  return localizedNativeToolText(messages, "nativeToolDescriptions", tool.toolKey) || tool.description || tool.type || tool.toolKey;
}

export function ChatModelConfig({
  disabled,
  options,
  defaultOptions,
  modelOptionPolicy,
  selectedProtocol,
  selectedModelName,
  onOptionsChange,
  onOptionsReset,
}: ChatModelConfigProps) {
  const tComposer = useTranslations("chat.composer");
  const tOptionLabels = useTranslations("chat.optionLabels");
  const [hovered, setHovered] = React.useState(false);
  const [customInputs, setCustomInputs] = React.useState<Record<string, string>>({});
  const settings = React.useMemo(
    () =>
      resolveAdvancedSettings({
        protocol: selectedProtocol,
        modelName: selectedModelName,
        options,
        defaultOptions,
        policy: modelOptionPolicy,
      }),
    [defaultOptions, modelOptionPolicy, options, selectedModelName, selectedProtocol],
  );
  const visibleSettings = React.useMemo(
    () => settings.filter((setting) => setting.kind !== "temperature"),
    [settings],
  );

  React.useEffect(() => {
    setCustomInputs({});
  }, [selectedModelName, selectedProtocol]);

  const updateSetting = React.useCallback(
    (setting: AdvancedSettingItem, value: number | string) => {
      onOptionsChange((current) => setAdvancedSettingValue(current, setting, value));
    },
    [onOptionsChange],
  );

  const resetSettings = React.useCallback(() => {
    const nextOptions = resetAdvancedSettings(options, defaultOptions, selectedProtocol, modelOptionPolicy, selectedModelName);
    setCustomInputs({});
    if (JSON.stringify(nextOptions) === JSON.stringify(defaultOptions)) {
      onOptionsReset(defaultOptions);
      return;
    }
    onOptionsChange(nextOptions);
  }, [defaultOptions, modelOptionPolicy, onOptionsChange, onOptionsReset, options, selectedModelName, selectedProtocol]);

  if (visibleSettings.length === 0) {
    return null;
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <InputGroupButton
          type="button"
          variant="ghost"
          size="icon-sm"
          className="size-7 rounded-md text-muted-foreground hover:text-foreground sm:size-8"
          disabled={disabled}
          aria-label={tComposer("advancedSettings")}
          title={tComposer("advancedSettings")}
          onMouseEnter={() => setHovered(true)}
          onMouseLeave={() => setHovered(false)}
        >
          <Cog
            size={20}
            strokeWidth={1.4}
            animate={hovered ? "default" : undefined}
          />
        </InputGroupButton>
      </PopoverTrigger>
      <PopoverContent
        side="bottom"
        align="start"
        sideOffset={8}
        className="w-[min(calc(100vw-2rem),13rem)] overflow-hidden rounded-xl border-[0.5px] border-border p-1.5 shadow-xs"
      >
        <div className="space-y-3">
          <div className="px-1 text-xs font-medium text-muted-foreground">
            {tComposer("advancedSettings")}
          </div>
          {visibleSettings.map((setting) => {
            const label = tOptionLabels(labelKeyForSetting(setting));
            if (setting.valueType === "number") {
              const numericValue = typeof setting.value === "number" ? setting.value : Number(setting.value);
              const value = Number.isFinite(numericValue) ? numericValue : Number(setting.fallbackValue);
              return (
                <div key={setting.key} className="space-y-2 rounded-md px-1 py-1">
                  <div className="flex items-center justify-between gap-3">
                    <span className="truncate text-xs text-foreground/85">{label}</span>
                    <Input
                      className="h-7 w-16 px-2 text-right text-xs"
                      inputMode="decimal"
                      max={setting.max}
                      min={setting.min}
                      step={setting.step}
                      type="number"
                      value={formatTemperature(value)}
                      onChange={(event) => updateSetting(setting, event.target.value)}
                    />
                  </div>
                  <Slider
                    min={setting.min}
                    max={setting.max}
                    step={setting.step}
                    value={[value]}
                    onValueChange={(nextValue) => {
                      const next = nextValue[0];
                      if (typeof next === "number") {
                        updateSetting(setting, next);
                      }
                    }}
                  />
                </div>
              );
            }
            const currentValue = String(setting.value);
            const presetValues = setting.values ?? [];
            const isPresetValue = presetValues.includes(currentValue);
            const customMode = Boolean(setting.customValueKind) && (customInputs[setting.key] !== undefined || !isPresetValue);
            const selectValue = customMode ? CUSTOM_SELECT_VALUE : currentValue;
            const customInputValue = customInputs[setting.key] ?? (isPresetValue ? "" : currentValue);
            const customValueInvalid =
              customMode &&
              customInputValue.trim() !== "" &&
              !isAdvancedSettingCustomValueValid(setting, customInputValue);
            return (
              <div key={setting.key} className="space-y-2 rounded-md px-1 py-1">
                <div className="grid grid-cols-[minmax(0,1fr)_6.5rem] items-center gap-3">
                  <span className="truncate text-xs text-foreground/85">{label}</span>
                  <Select
                    value={selectValue}
                    onValueChange={(nextValue) => {
                      if (nextValue === CUSTOM_SELECT_VALUE) {
                        setCustomInputs((current) => ({
                          ...current,
                          [setting.key]: isPresetValue ? "" : currentValue,
                        }));
                        return;
                      }
                      setCustomInputs((current) => {
                        const { [setting.key]: _removed, ...remaining } = current;
                        return remaining;
                      });
                      updateSetting(setting, nextValue);
                    }}
                  >
                    <SelectTrigger size="sm" className="h-8 text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {presetValues.map((value) => (
                        <SelectItem key={value} value={value}>
                          {value}
                        </SelectItem>
                      ))}
                      {setting.customValueKind ? (
                        <SelectItem value={CUSTOM_SELECT_VALUE}>
                          {tComposer("customResolution")}
                        </SelectItem>
                      ) : null}
                    </SelectContent>
                  </Select>
                </div>
                {customMode ? (
                  <div className="space-y-1">
                    <Input
                      className="h-7 px-2 text-xs"
                      placeholder={tComposer("customResolutionPlaceholder")}
                      value={customInputValue}
                      onChange={(event) => {
                        const nextValue = event.target.value;
                        setCustomInputs((current) => ({ ...current, [setting.key]: nextValue }));
                        if (isAdvancedSettingCustomValueValid(setting, nextValue)) {
                          updateSetting(setting, nextValue);
                        }
                      }}
                    />
                    {customValueInvalid ? (
                      <p className="px-1 text-[11px] leading-4 text-destructive">
                        {tComposer("invalidResolution")}
                      </p>
                    ) : null}
                  </div>
                ) : null}
              </div>
            );
          })}
          <button
            type="button"
            className="flex h-8 w-full items-center justify-center rounded-md px-2 text-xs text-muted-foreground outline-none transition-colors hover:bg-accent/40 hover:text-foreground focus-visible:bg-accent/40 focus-visible:text-foreground"
            onClick={resetSettings}
          >
            {tComposer("resetAdvancedDefaults")}
          </button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
