"use client";

import { LoaderCircle } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

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
import { Switch } from "@/components/ui/switch";
import {
  type AdvancedSettingItem,
  isAdvancedSettingCustomValueValid,
  resetAdvancedSettings,
  resolveAdvancedSettings,
  setAdvancedSettingValue,
} from "@/features/chat/model/advanced-settings";
import {
  type ResolvedServerOptionControl,
  removeOptionAtPath,
  resolveServerOptionControls,
  setOptionAtPath,
} from "@/features/chat/model/chat-model-controls";
import type { ChatSubmitTask } from "@/features/chat/model/chat-task";
import type { ModelOptionControl } from "@/features/chat/types/chat-runtime";
import type { ConversationOptions } from "@/shared/api/conversation.types";
import type { ModelOptionPolicy } from "@/shared/lib/model-option-policy";

type ChatModelConfigProps = {
  disabled: boolean;
  options: ConversationOptions;
  defaultOptions: ConversationOptions;
  optionControls?: ModelOptionControl[];
  lockedOptionPaths?: string[];
  modelOptionPolicy: ModelOptionPolicy | null;
  selectedProtocol: string;
  selectedModelName: string;
  submitTask: ChatSubmitTask;
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
    case "reasoningSummary":
      return "reasoning_summary";
    case "verbosity":
      return "verbosity";
    case "outputFormat":
      return "output_format";
    case "speed":
      return "speed";
    case "imageQuality":
      return "quality";
    case "imageResolution":
      return "resolution";
    case "imageAspectRatio":
      return "aspect_ratio";
    case "imageSize":
      return "image_size";
    case "responseFormat":
      return "response_format";
    case "thinkingLevel":
      return "thinking_level";
    case "videoAspectRatio":
      return "aspect_ratio";
    case "videoDuration":
      return "video_duration";
    case "videoResolution":
      return "videoResolution";
    case "videoSeconds":
      return "videoSeconds";
    case "videoTask":
      return "video_task_type";
  }
}


export function ChatModelConfig({
  disabled,
  options,
  defaultOptions,
  modelOptionPolicy,
  selectedProtocol,
  selectedModelName,
  submitTask,
  optionControls = [],
  lockedOptionPaths = [],
  onOptionsChange,
  onOptionsReset,
  onDefaultOptionsRestore,
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
        submitTask,
        options,
        defaultOptions,
        policy: modelOptionPolicy,
      }),
    [defaultOptions, modelOptionPolicy, options, selectedModelName, selectedProtocol, submitTask],
  );
  const visibleSettings = React.useMemo(
    () => settings.filter((setting) => setting.kind !== "temperature"),
    [settings],
  );
  const lockedAdvancedSettingKeys = React.useMemo(
    () => new Set(lockedOptionPaths.map((path) => path.split(".").map((segment) => segment.trim()).filter(Boolean).join(".")).filter(Boolean)),
    [lockedOptionPaths],
  );
  const customSettings = React.useMemo(
    () => resolveServerOptionControls({
      controls: optionControls,
      options,
      defaultOptions,
      lockedOptionPaths,
      policy: modelOptionPolicy,
      protocol: selectedProtocol,
      excludedPaths: settings.map((setting) => setting.path.join(".")),
    }),
    [defaultOptions, lockedOptionPaths, modelOptionPolicy, optionControls, options, selectedProtocol, settings],
  );
  const [restoringDefaults, setRestoringDefaults] = React.useState(false);

  React.useEffect(() => {
    setCustomInputs({});
  }, [selectedModelName, selectedProtocol]);

  const updateSetting = React.useCallback(
    (setting: AdvancedSettingItem, value: number | string) => {
      onOptionsChange((current) => setAdvancedSettingValue(current, setting, value));
    },
    [onOptionsChange],
  );

  const updateCustomSetting = React.useCallback(
    (setting: ResolvedServerOptionControl, value: number | string | boolean) => {
      if (setting.locked) {
        return;
      }
      onOptionsChange((current) => setOptionAtPath(current, setting.path, value));
    },
    [onOptionsChange],
  );

  const resetSettings = React.useCallback(() => {
    let nextOptions = resetAdvancedSettings(
      options,
      defaultOptions,
      selectedProtocol,
      modelOptionPolicy,
      selectedModelName,
      submitTask,
    );
    for (const setting of customSettings) {
      nextOptions = removeOptionAtPath(nextOptions, setting.path);
      if (setting.defaultValue !== undefined) {
        nextOptions = setOptionAtPath(nextOptions, setting.path, setting.defaultValue);
      }
    }
    setCustomInputs({});
    if (JSON.stringify(nextOptions) === JSON.stringify(defaultOptions)) {
      onOptionsReset(defaultOptions);
      return;
    }
    onOptionsChange(nextOptions);
  }, [customSettings, defaultOptions, modelOptionPolicy, onOptionsChange, onOptionsReset, options, selectedModelName, selectedProtocol, submitTask]);

  const restoreDefaults = React.useCallback(async () => {
    if (!onDefaultOptionsRestore || restoringDefaults) {
      return;
    }
    setRestoringDefaults(true);
    try {
      const latestDefaults = await onDefaultOptionsRestore();
      if (!latestDefaults) {
        toast.error(tComposer("defaultModelUnavailable"));
        return;
      }
      setCustomInputs({});
      onOptionsReset(latestDefaults);
    } catch {
      toast.error(tComposer("defaultLoadFailed"));
    } finally {
      setRestoringDefaults(false);
    }
  }, [onDefaultOptionsRestore, onOptionsReset, restoringDefaults, tComposer]);

  if (visibleSettings.length === 0 && customSettings.length === 0) {
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
            const settingLocked = lockedAdvancedSettingKeys.has(setting.key);
            if (setting.valueType === "number") {
              const numericValue = typeof setting.value === "number" ? setting.value : Number(setting.value);
              const value = Number.isFinite(numericValue) ? numericValue : Number(setting.fallbackValue);
              return (
                <div key={setting.key} className="space-y-2 rounded-md px-1 py-1">
                  <div className="flex items-center justify-between gap-3">
                    <span className="truncate text-xs text-foreground/85">
                      {label}
                      {settingLocked ? <span className="ml-1 text-[10px] text-muted-foreground">{tComposer("locked")}</span> : null}
                    </span>
                    <Input
                      className="h-7 w-16 px-2 text-right text-xs"
                      inputMode="decimal"
                      max={setting.max}
                      min={setting.min}
                      step={setting.step}
                      type="number"
                      value={formatTemperature(value)}
                      disabled={disabled || settingLocked}
                      onChange={(event) => updateSetting(setting, event.target.value)}
                    />
                  </div>
                  <Slider
                    min={setting.min}
                    max={setting.max}
                    step={setting.step}
                    value={[value]}
                    disabled={disabled || settingLocked}
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
                  <span className="truncate text-xs text-foreground/85">
                    {label}
                    {settingLocked ? <span className="ml-1 text-[10px] text-muted-foreground">{tComposer("locked")}</span> : null}
                  </span>
                  <Select
                    value={selectValue}
                    disabled={disabled || settingLocked}
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
                      disabled={disabled || settingLocked}
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
          {customSettings.length > 0 ? (
            <>
              {visibleSettings.length > 0 ? <div className="h-px bg-border/70" /> : null}
              {customSettings.map((setting) => {
                const label = setting.label;
                const description = setting.description || undefined;
                const commonLabel = (
                  <span className="min-w-0 truncate text-xs text-foreground/85" title={description}>
                    {label}
                  </span>
                );
                if (setting.valueType === "boolean") {
                  return (
                    <div key={setting.key} className="flex items-center justify-between gap-3 rounded-md px-1 py-1">
                      {commonLabel}
                      <div className="flex shrink-0 items-center gap-2">
                        {setting.locked ? <span className="text-[10px] text-muted-foreground">{tComposer("locked")}</span> : null}
                        <Switch
                          size="sm"
                          checked={setting.value === true}
                          disabled={disabled || setting.locked}
                          aria-label={label}
                          onCheckedChange={(checked) => updateCustomSetting(setting, checked)}
                        />
                      </div>
                    </div>
                  );
                }
                if (setting.valueType === "select") {
                  return (
                    <div key={setting.key} className="space-y-1 rounded-md px-1 py-1">
                      <div className="grid grid-cols-[minmax(0,1fr)_6.5rem] items-center gap-3">
                        {commonLabel}
                        <Select
                          value={String(setting.value)}
                          onValueChange={(value) => updateCustomSetting(setting, value)}
                          disabled={disabled || setting.locked}
                        >
                          <SelectTrigger size="sm" className="h-8 text-xs" aria-label={label}>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {setting.options.map((value) => (
                              <SelectItem key={value} value={value}>
                                {value}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                      {setting.locked ? <p className="px-1 text-[10px] text-muted-foreground">{tComposer("lockedHelp")}</p> : null}
                    </div>
                  );
                }
                return (
                  <div key={setting.key} className="space-y-1 rounded-md px-1 py-1">
                    <div className="flex items-center justify-between gap-3">
                      {commonLabel}
                      {setting.locked ? <span className="shrink-0 text-[10px] text-muted-foreground">{tComposer("locked")}</span> : null}
                    </div>
                    <Input
                      className="h-7 px-2 text-xs"
                      type={setting.valueType === "number" ? "number" : "text"}
                      inputMode={setting.valueType === "number" ? "decimal" : undefined}
                      value={String(setting.value)}
                      placeholder={setting.placeholder}
                      disabled={disabled || setting.locked}
                      aria-label={label}
                      onChange={(event) => {
                        const raw = event.target.value;
                        if (setting.valueType !== "number") {
                          updateCustomSetting(setting, raw);
                          return;
                        }
                        const numeric = raw.trim() === "" ? "" : Number(raw);
                        updateCustomSetting(
                          setting,
                          typeof numeric === "number" && Number.isFinite(numeric) ? numeric : raw,
                        );
                      }}
                    />
                    {setting.locked ? <p className="px-1 text-[10px] text-muted-foreground">{tComposer("lockedHelp")}</p> : null}
                  </div>
                );
              })}
            </>
          ) : null}
          <div className="flex items-center gap-1">
            <button
              type="button"
              className="flex h-8 min-w-0 flex-1 items-center justify-center rounded-md px-2 text-xs text-muted-foreground outline-none transition-colors hover:bg-accent/40 hover:text-foreground focus-visible:bg-accent/40 focus-visible:text-foreground"
              onClick={resetSettings}
            >
              {tComposer("resetAdvancedDefaults")}
            </button>
            {onDefaultOptionsRestore ? (
              <button
                type="button"
                className="flex h-8 shrink-0 items-center justify-center gap-1 rounded-md px-2 text-xs text-muted-foreground outline-none transition-colors hover:bg-accent/40 hover:text-foreground focus-visible:bg-accent/40 focus-visible:text-foreground disabled:pointer-events-none disabled:opacity-50"
                disabled={disabled || restoringDefaults}
                onClick={() => void restoreDefaults()}
              >
                {restoringDefaults ? <LoaderCircle className="size-3 animate-spin" /> : null}
                {tComposer("default")}
              </button>
            ) : null}
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
