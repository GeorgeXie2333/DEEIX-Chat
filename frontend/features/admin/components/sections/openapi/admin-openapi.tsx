"use client";

import * as React from "react";
import { RefreshCw, Save } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { listAdminLLMModels, listAdminSettingsByNamespace, patchAdminSettings } from "@/features/admin/api";
import type { AdminLLMModelDTO } from "@/features/admin/api/llm.types";
import { isOpenAPITextModel } from "@/features/admin/model/openapi-model-filter";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { cn } from "@/lib/utils";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  SettingsFieldList,
  SettingsFieldRow,
  SettingsPage,
  SettingsSection,
  SettingsSectionSeparator,
} from "@/shared/components/settings-layout";
import type { PatchSettingItem, SettingItem } from "@/shared/api/settings.types";

function settingValue(items: SettingItem[], key: string, fallback = ""): string {
  return items.find((item) => item.key === key)?.value ?? fallback;
}

function parseModelNames(value: string): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const item of value.split(/[,\n\r\t]+/)) {
    const name = item.trim();
    if (!name || seen.has(name)) continue;
    seen.add(name);
    result.push(name);
  }
  return result;
}

function selectedModelNames(models: AdminLLMModelDTO[], selected: Set<string>, manualText: string): string[] {
  const names = models
    .map((model) => model.platformModelName)
    .filter((name) => selected.has(name));
  return [...names, ...parseModelNames(manualText)].filter((name, index, array) => array.indexOf(name) === index);
}

export function AdminOpenAPISettingsPage() {
  const t = useTranslations("adminOpenAPI");
  const translateError = useLocalizedErrorMessage();
  const [models, setModels] = React.useState<AdminLLMModelDTO[]>([]);
  const [selected, setSelected] = React.useState<Set<string>>(new Set());
  const [manualText, setManualText] = React.useState("");
  const [rpm, setRPM] = React.useState("60");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"));
        return;
      }
      const [settings, modelPage] = await Promise.all([
        listAdminSettingsByNamespace(token, "openapi"),
        listAdminLLMModels(token, {
          page: 1,
          pageSize: 500,
          onlyActive: true,
          sort: "sortOrder_asc",
        }),
      ]);
      const availableModels = modelPage.results.filter(isOpenAPITextModel);
      const allowlist = parseModelNames(settingValue(settings, "model_allowlist"));
      const availableNames = new Set(availableModels.map((model) => model.platformModelName));
      setModels(availableModels);
      setSelected(new Set(allowlist.filter((name) => availableNames.has(name))));
      setManualText(allowlist.filter((name) => !availableNames.has(name)).join("\n"));
      setRPM(settingValue(settings, "rate_limit_rpm", "60"));
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: translateError(error) });
    } finally {
      setLoading(false);
    }
  }, [t, translateError]);

  React.useEffect(() => {
    void load();
  }, [load]);

  const toggleModel = React.useCallback((name: string, checked: boolean) => {
    setSelected((current) => {
      const next = new Set(current);
      if (checked) next.add(name);
      else next.delete(name);
      return next;
    });
  }, []);

  const save = React.useCallback(async () => {
    const normalizedRPM = rpm.trim();
    if (!/^\d+$/.test(normalizedRPM)) {
      toast.error(t("toast.rpmInvalid"));
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"));
        return;
      }
      const items: PatchSettingItem[] = [
        {
          namespace: "openapi",
          key: "model_allowlist",
          value: selectedModelNames(models, selected, manualText).join("\n"),
        },
        {
          namespace: "openapi",
          key: "rate_limit_rpm",
          value: normalizedRPM,
        },
      ];
      await patchAdminSettings(token, { items });
      toast.success(t("toast.saved"));
    } catch (error) {
      toast.error(t("toast.saveFailed"), { description: translateError(error) });
    } finally {
      setSaving(false);
    }
  }, [manualText, models, rpm, selected, t, translateError]);

  return (
    <SettingsPage>
      <SettingsSection
        title={t("title")}
        actions={
          <>
            <Button type="button" variant="outline" size="sm" disabled={loading || saving} onClick={() => void load()}>
              <RefreshCw className="size-4" />
              {t("actions.reload")}
            </Button>
            <Button type="button" size="sm" disabled={loading || saving} onClick={() => void save()}>
              <Save className="size-4" />
              {saving ? t("actions.saving") : t("actions.save")}
            </Button>
          </>
        }
      >
        <SettingsFieldList>
          <SettingsFieldRow title={t("fields.rpm")} controlClassName="md:w-40">
            <Input
              inputMode="numeric"
              pattern="[0-9]*"
              value={rpm}
              disabled={loading || saving}
              onChange={(event) => setRPM(event.target.value)}
              className="h-9 text-right"
            />
          </SettingsFieldRow>
        </SettingsFieldList>
      </SettingsSection>

      <SettingsSectionSeparator />

      <SettingsSection title={t("fields.models")}>
        <div className="overflow-hidden rounded-md border">
          <div className="max-h-[26rem] overflow-y-auto">
            {loading ? (
              <div className="px-3 py-8 text-center text-sm text-muted-foreground">{t("state.loading")}</div>
            ) : models.length === 0 ? (
              <div className="px-3 py-8 text-center text-sm text-muted-foreground">{t("state.empty")}</div>
            ) : (
              models.map((model, index) => {
                const checked = selected.has(model.platformModelName);
                return (
                  <label
                    key={model.id}
                    className={cn(
                      "flex min-h-11 cursor-pointer items-center gap-3 px-3 py-2 text-sm",
                      index > 0 && "border-t",
                    )}
                  >
                    <Checkbox
                      checked={checked}
                      onCheckedChange={(value) => toggleModel(model.platformModelName, value === true)}
                    />
                    <span className="min-w-0 flex-1 truncate font-medium">{model.platformModelName}</span>
                    <span className="shrink-0 text-xs text-muted-foreground">
                      {t("state.activeSources", { count: model.activeSourceCount })}
                    </span>
                  </label>
                );
              })
            )}
          </div>
        </div>
      </SettingsSection>

      <SettingsSection title={t("fields.manual")}>
        <Textarea
          value={manualText}
          disabled={loading || saving}
          onChange={(event) => setManualText(event.target.value)}
          placeholder={t("placeholders.manual")}
          className="min-h-24 font-mono text-xs"
        />
      </SettingsSection>
    </SettingsPage>
  );
}
