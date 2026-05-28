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
  modelOptionPolicy: ModelOptionPolicy | null;
  selectedProtocol: string;
  onOptionsChange: React.Dispatch<React.SetStateAction<ConversationOptions>>;
  onOptionsReset: () => void;
};

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
  }
}

export function ChatModelConfig({
  disabled,
  options,
  defaultOptions,
  modelOptionPolicy,
  selectedProtocol,
  onOptionsChange,
  onOptionsReset,
}: ChatModelConfigProps) {
  const tComposer = useTranslations("chat.composer");
  const tOptionLabels = useTranslations("chat.optionLabels");
  const [hovered, setHovered] = React.useState(false);
  const settings = React.useMemo(
    () =>
      resolveAdvancedSettings({
        protocol: selectedProtocol,
        options,
        defaultOptions,
        policy: modelOptionPolicy,
      }),
    [defaultOptions, modelOptionPolicy, options, selectedProtocol],
  );

  const updateSetting = React.useCallback(
    (setting: AdvancedSettingItem, value: number | string) => {
      onOptionsChange((current) => setAdvancedSettingValue(current, setting, value));
    },
    [onOptionsChange],
  );

  const resetSettings = React.useCallback(() => {
    const nextOptions = resetAdvancedSettings(options, defaultOptions, selectedProtocol, modelOptionPolicy);
    if (JSON.stringify(nextOptions) === JSON.stringify(defaultOptions)) {
      onOptionsReset();
      return;
    }
    onOptionsChange(nextOptions);
  }, [defaultOptions, modelOptionPolicy, onOptionsChange, onOptionsReset, options, selectedProtocol]);

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
            return (
              <div key={setting.key} className="grid grid-cols-[minmax(0,1fr)_9rem] items-center gap-3 rounded-md px-1 py-1">
                <span className="truncate text-xs text-foreground/85">{label}</span>
                <Select value={String(setting.value)} onValueChange={(nextValue) => updateSetting(setting, nextValue)}>
                  <SelectTrigger size="sm" className="h-8 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {(setting.values ?? []).map((value) => (
                      <SelectItem key={value} value={value}>
                        {value}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
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
