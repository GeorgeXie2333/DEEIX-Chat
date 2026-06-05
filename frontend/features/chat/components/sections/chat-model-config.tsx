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
    case "reasoningEffort":
      return "reasoning_effort";
    case "verbosity":
      return "verbosity";
    case "imageQuality":
      return "quality";
    case "imageResolution":
      return "resolution";
    case "videoResolution":
      return "videoResolution";
    case "videoSeconds":
      return "videoSeconds";
  }
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

  if (settings.length === 0) {
    return null;
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <InputGroupButton
          type="button"
          variant="ghost"
          size="icon-sm"
          className="rounded-md text-muted-foreground hover:text-foreground"
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
        className="w-[min(calc(100vw-2rem),20rem)] p-2"
      >
        <div className="space-y-3">
          <div className="px-1 text-xs font-medium text-muted-foreground">
            {tComposer("advancedSettings")}
          </div>
          {settings.map((setting) => {
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
                <div className="grid grid-cols-[minmax(0,1fr)_9rem] items-center gap-3">
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
