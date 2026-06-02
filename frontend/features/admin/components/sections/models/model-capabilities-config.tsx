"use client";

import { useState } from "react";
import { CircleHelp, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

export const MODEL_CAPABILITIES_PLACEHOLDER = `{
  "defaultOptions": {},
  "optionControls": [
    {
      "path": "size",
      "label": "Size",
      "type": "select",
      "options": ["1024x1024", "1024x1536", "1536x1024"]
    },
    {
      "path": "quality",
      "label": "Quality",
      "type": "select",
      "options": ["standard", "hd"]
    }
  ]
}`;

type CapabilityControlType = "text" | "select" | "number" | "boolean";

type DefaultOptionRow = {
  id: string;
  path: string;
  value: string;
};

type OptionControlRow = {
  id: string;
  path: string;
  label: string;
  type: CapabilityControlType;
  options: string;
  placeholder: string;
};

const CAPABILITY_CONTROL_TYPES: CapabilityControlType[] = ["text", "select", "number", "boolean"];

function createCapabilityRowID(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function isPlainJSONObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function parseCapabilitiesObject(raw: string): Record<string, unknown> | null {
  const normalized = raw.trim();
  if (!normalized) {
    return {};
  }
  try {
    const parsed = JSON.parse(normalized) as unknown;
    return isPlainJSONObject(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function optionPathSegments(path: string): string[] {
  return path
    .split(".")
    .map((segment) => segment.trim())
    .filter(Boolean);
}

function formatDefaultOptionValue(value: unknown): string {
  if (value === undefined) {
    return "";
  }
  return JSON.stringify(value);
}

function flattenDefaultOptions(value: unknown, prefix: string[] = []): DefaultOptionRow[] {
  if (isPlainJSONObject(value)) {
    return Object.entries(value).flatMap(([key, child]) => flattenDefaultOptions(child, [...prefix, key]));
  }
  if (prefix.length === 0) {
    return [];
  }
  return [{
    id: createCapabilityRowID(),
    path: prefix.join("."),
    value: formatDefaultOptionValue(value),
  }];
}

function parseDefaultOptionValue(value: string): unknown {
  const normalized = value.trim();
  if (!normalized) {
    return null;
  }
  try {
    return JSON.parse(normalized) as unknown;
  } catch {
    return normalized;
  }
}

function setNestedOptionValue(target: Record<string, unknown>, path: string[], value: unknown) {
  if (path.length === 0) {
    return;
  }
  let current = target;
  path.slice(0, -1).forEach((segment) => {
    const nextValue = current[segment];
    if (!isPlainJSONObject(nextValue)) {
      current[segment] = {};
    }
    current = current[segment] as Record<string, unknown>;
  });
  current[path[path.length - 1]] = value;
}

function buildDefaultOptions(rows: DefaultOptionRow[]): Record<string, unknown> {
  const options: Record<string, unknown> = {};
  rows.forEach((row) => {
    const path = optionPathSegments(row.path);
    if (path.length === 0) {
      return;
    }
    setNestedOptionValue(options, path, parseDefaultOptionValue(row.value));
  });
  return options;
}

function normalizeControlType(value: unknown): CapabilityControlType {
  return CAPABILITY_CONTROL_TYPES.includes(value as CapabilityControlType)
    ? (value as CapabilityControlType)
    : "text";
}

function parseOptionControls(value: unknown): OptionControlRow[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.flatMap((item): OptionControlRow[] => {
    if (!isPlainJSONObject(item) || typeof item.path !== "string") {
      return [];
    }
    const path = optionPathSegments(item.path);
    if (path.length === 0) {
      return [];
    }
    const options = Array.isArray(item.options)
      ? item.options.map((option) => (typeof option === "string" ? option.trim() : "")).filter(Boolean).join("\n")
      : "";
    return [{
      id: createCapabilityRowID(),
      path: path.join("."),
      label: typeof item.label === "string" ? item.label : "",
      type: normalizeControlType(item.type),
      options,
      placeholder: typeof item.placeholder === "string" ? item.placeholder : "",
    }];
  });
}

function parseControlOptions(value: string): string[] {
  return Array.from(
    new Set(
      value
        .split(/\n|,/)
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  );
}

function buildOptionControls(rows: OptionControlRow[]): Record<string, unknown>[] {
  return rows.flatMap((row): Record<string, unknown>[] => {
    const path = optionPathSegments(row.path);
    if (path.length === 0) {
      return [];
    }
    const control: Record<string, unknown> = {
      path: path.join("."),
      type: row.type,
    };
    const label = row.label.trim();
    const placeholder = row.placeholder.trim();
    const options = parseControlOptions(row.options);
    if (label) {
      control.label = label;
    }
    if (placeholder) {
      control.placeholder = placeholder;
    }
    if (options.length > 0) {
      control.options = options;
    }
    return [control];
  });
}

function buildCapabilitiesJSON(
  currentJSON: string,
  defaultRows: DefaultOptionRow[],
  controlRows: OptionControlRow[],
): string | null {
  const payload = parseCapabilitiesObject(currentJSON);
  if (!payload) {
    return null;
  }
  const defaultOptions = buildDefaultOptions(defaultRows);
  const optionControls = buildOptionControls(controlRows);
  if (Object.keys(defaultOptions).length > 0) {
    payload.defaultOptions = defaultOptions;
  } else {
    delete payload.defaultOptions;
  }
  if (optionControls.length > 0) {
    payload.optionControls = optionControls;
  } else {
    delete payload.optionControls;
  }
  return Object.keys(payload).length > 0 ? JSON.stringify(payload, null, 2) : "";
}

export function ModelCapabilitiesGuideButton({ t }: { t: (key: string) => string }) {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button type="button" variant="ghost" size="sm" className="h-6 px-2 text-xs font-normal text-muted-foreground hover:text-foreground">
          <CircleHelp className="size-3.5" />
          {t("sheet.capabilitiesGuide.button")}
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-[760px]">
        <DialogHeader>
          <DialogTitle>{t("sheet.capabilitiesGuide.title")}</DialogTitle>
          <DialogDescription>{t("sheet.capabilitiesGuide.description")}</DialogDescription>
        </DialogHeader>

        <Tabs defaultValue="defaults" className="gap-3">
          <TabsList>
            <TabsTrigger value="defaults">{t("sheet.capabilitiesGuide.defaultsTab")}</TabsTrigger>
            <TabsTrigger value="controls">{t("sheet.capabilitiesGuide.controlsTab")}</TabsTrigger>
            <TabsTrigger value="policy">{t("sheet.capabilitiesGuide.policyTab")}</TabsTrigger>
          </TabsList>

          <TabsContent value="defaults" className="space-y-3 text-sm text-muted-foreground">
            <p className="text-xs">{t("sheet.capabilitiesGuide.defaultsDescription")}</p>
            <pre className="max-h-72 overflow-auto rounded-md bg-muted/50 p-3 text-xs text-foreground">
{`{
  "defaultOptions": {
    "reasoning": {
      "effort": "high"
    }
  }
}`}
            </pre>
          </TabsContent>

          <TabsContent value="controls" className="space-y-3 text-sm text-muted-foreground">
            <p className="text-xs">{t("sheet.capabilitiesGuide.controlsDescription")}</p>
            <pre className="max-h-72 overflow-auto rounded-md bg-muted/50 p-3 text-xs text-foreground">
{`{
  "defaultOptions": {},
  "optionControls": [
    {
      "path": "size",
      "label": "Size",
      "type": "select",
      "options": ["1024x1024", "1024x1536", "1536x1024"]
    },
    {
      "path": "quality",
      "label": "Quality",
      "type": "select",
      "options": ["standard", "hd"]
    },
    {
      "path": "n",
      "label": "Count",
      "type": "number",
      "placeholder": "1"
    }
  ]
}`}
            </pre>
            <p className="text-xs">{t("sheet.capabilitiesGuide.controlTypes")}</p>
          </TabsContent>

          <TabsContent value="policy" className="space-y-3 text-sm text-muted-foreground">
            <p className="text-xs">{t("sheet.capabilitiesGuide.policyDescription")}</p>
            <pre className="max-h-72 overflow-auto rounded-md bg-muted/50 p-3 text-xs text-foreground">
{`{
  "openai_image_generations": [
    "size",
    "quality",
    "n"
  ],
  "openai_image_edits": [
    "size",
    "quality",
    "n"
  ]
}`}
            </pre>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}

export function ModelCapabilitiesQuickConfig({
  value,
  disabled,
  t,
  commonT,
  onApply,
}: {
  value: string;
  disabled: boolean;
  t: (key: string) => string;
  commonT: (key: string) => string;
  onApply: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<"defaults" | "controls">("defaults");
  const [defaultRows, setDefaultRows] = useState<DefaultOptionRow[]>([]);
  const [controlRows, setControlRows] = useState<OptionControlRow[]>([]);

  function loadDraft() {
    const payload = parseCapabilitiesObject(value);
    if (!payload) {
      toast.error(t("sheet.capabilitiesQuick.invalidJSON"));
      return false;
    }
    setDefaultRows(flattenDefaultOptions(payload.defaultOptions));
    setControlRows(parseOptionControls(payload.optionControls));
    setActiveTab("defaults");
    return true;
  }

  function openDialog() {
    if (!loadDraft()) {
      return;
    }
    setOpen(true);
  }

  function updateDefaultRow(id: string, patch: Partial<DefaultOptionRow>) {
    setDefaultRows((prev) => prev.map((row) => (row.id === id ? { ...row, ...patch } : row)));
  }

  function updateControlRow(id: string, patch: Partial<OptionControlRow>) {
    setControlRows((prev) => prev.map((row) => (row.id === id ? { ...row, ...patch } : row)));
  }

  function updateControlType(id: string, type: CapabilityControlType) {
    setControlRows((prev) => prev.map((row) => (
      row.id === id ? { ...row, type, options: type === "select" ? row.options : "" } : row
    )));
  }

  function applyDraft() {
    const nextValue = buildCapabilitiesJSON(value, defaultRows, controlRows);
    if (nextValue === null) {
      toast.error(t("sheet.capabilitiesQuick.invalidJSON"));
      return;
    }
    onApply(nextValue);
    setOpen(false);
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="h-6 px-2 text-[11px]"
        disabled={disabled}
        onClick={openDialog}
      >
        {t("sheet.capabilitiesQuick.button")}
      </Button>
      <DialogContent className="flex max-h-[min(90dvh,680px)] min-w-0 flex-col overflow-x-hidden sm:max-w-[760px]">
        <DialogHeader>
          <DialogTitle>{t("sheet.capabilitiesQuick.title")}</DialogTitle>
          <DialogDescription>{t("sheet.capabilitiesQuick.description")}</DialogDescription>
        </DialogHeader>

        <div className="min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto pr-1">
          <Tabs
            value={activeTab}
            onValueChange={(value) => setActiveTab(value as "defaults" | "controls")}
            className="min-w-0 gap-3"
          >
            <div className="flex min-w-0 items-center justify-between gap-2">
              <TabsList className="h-8 min-w-0 shrink">
                <TabsTrigger value="defaults">{t("sheet.capabilitiesGuide.defaultsTab")}</TabsTrigger>
                <TabsTrigger value="controls">{t("sheet.capabilitiesGuide.controlsTab")}</TabsTrigger>
              </TabsList>
              {activeTab === "defaults" ? (
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  className="h-7 shrink-0 whitespace-nowrap px-2 text-xs"
                  onClick={() => setDefaultRows((prev) => [...prev, { id: createCapabilityRowID(), path: "", value: "" }])}
                >
                  <Plus className="size-3.5" />
                  {t("sheet.capabilitiesQuick.addDefault")}
                </Button>
              ) : (
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  className="h-7 shrink-0 whitespace-nowrap px-2 text-xs"
                  onClick={() => setControlRows((prev) => [...prev, {
                    id: createCapabilityRowID(),
                    path: "",
                    label: "",
                    type: "text",
                    options: "",
                    placeholder: "",
                  }])}
                >
                  <Plus className="size-3.5" />
                  {t("sheet.capabilitiesQuick.addControl")}
                </Button>
              )}
            </div>
            <p className="text-xs leading-5 text-muted-foreground">
              {activeTab === "defaults" ? t("sheet.capabilitiesQuick.defaultsHelp") : t("sheet.capabilitiesQuick.controlsHelp")}
            </p>

            <TabsContent value="defaults" className="space-y-3">
              {defaultRows.length === 0 ? (
                <div className="rounded-md border border-dashed px-3 py-8 text-center text-xs text-muted-foreground">
                  {t("sheet.capabilitiesQuick.emptyDefaults")}
                </div>
              ) : (
                <div className="min-w-0 space-y-1.5">
                    <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_32px] gap-2 px-1 text-[11px] text-muted-foreground">
                      <span>{t("sheet.capabilitiesQuick.pathColumn")}</span>
                      <span>{t("sheet.capabilitiesQuick.valueColumn")}</span>
                      <span />
                    </div>
                    {defaultRows.map((row) => (
                      <div key={row.id} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_32px] items-center gap-2">
                        <Input
                          className="h-8"
                          value={row.path}
                          placeholder="reasoning.effort"
                          onChange={(event) => updateDefaultRow(row.id, { path: event.target.value })}
                        />
                        <Input
                          className="h-8"
                          value={row.value}
                          placeholder='"high"'
                          onChange={(event) => updateDefaultRow(row.id, { value: event.target.value })}
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon-sm"
                          className="justify-self-end text-muted-foreground hover:text-destructive"
                          onClick={() => setDefaultRows((prev) => prev.filter((item) => item.id !== row.id))}
                          aria-label={commonT("actions.delete")}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </div>
                    ))}
                </div>
              )}
            </TabsContent>

            <TabsContent value="controls" className="space-y-3">
              {controlRows.length === 0 ? (
                <div className="rounded-md border border-dashed px-3 py-8 text-center text-xs text-muted-foreground">
                  {t("sheet.capabilitiesQuick.emptyControls")}
                </div>
              ) : (
                <div className="min-w-0 space-y-1.5">
                    <div className="hidden grid-cols-[repeat(5,minmax(0,1fr))_32px] gap-2 px-1 text-[11px] text-muted-foreground sm:grid">
                      <span>{t("sheet.capabilitiesQuick.pathColumn")}</span>
                      <span>{t("sheet.capabilitiesQuick.labelColumn")}</span>
                      <span>{t("sheet.capabilitiesQuick.typeColumn")}</span>
                      <span>{t("sheet.capabilitiesQuick.optionsColumn")}</span>
                      <span>{t("sheet.capabilitiesQuick.placeholderColumn")}</span>
                      <span />
                    </div>
                    {controlRows.map((row) => (
                      <div key={row.id} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_32px] items-center gap-2 sm:grid-cols-[repeat(5,minmax(0,1fr))_32px]">
                        <Input
                          className="h-8"
                          value={row.path}
                          placeholder="size"
                          onChange={(event) => updateControlRow(row.id, { path: event.target.value })}
                        />
                        <Input
                          className="h-8"
                          value={row.label}
                          placeholder={t("sheet.capabilitiesQuick.labelPlaceholder")}
                          onChange={(event) => updateControlRow(row.id, { label: event.target.value })}
                        />
                        <Select
                          value={row.type}
                          onValueChange={(type) => updateControlType(row.id, type as CapabilityControlType)}
                        >
                          <SelectTrigger className="h-8">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {CAPABILITY_CONTROL_TYPES.map((type) => (
                              <SelectItem key={type} value={type}>
                                {type}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <Input
                          className="h-8"
                          value={row.options}
                          disabled={row.type !== "select"}
                          placeholder={t("sheet.capabilitiesQuick.optionsPlaceholder")}
                          onChange={(event) => updateControlRow(row.id, { options: event.target.value })}
                        />
                        <Input
                          className="h-8"
                          value={row.placeholder}
                          placeholder={t("sheet.capabilitiesQuick.placeholderPlaceholder")}
                          onChange={(event) => updateControlRow(row.id, { placeholder: event.target.value })}
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon-sm"
                          className="justify-self-end text-muted-foreground hover:text-destructive"
                          onClick={() => setControlRows((prev) => prev.filter((item) => item.id !== row.id))}
                          aria-label={commonT("actions.delete")}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </div>
                    ))}
                </div>
              )}
            </TabsContent>
          </Tabs>
        </div>

        <DialogFooter>
          <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
            {commonT("actions.cancel")}
          </Button>
          <Button type="button" onClick={applyDraft}>
            {commonT("actions.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
