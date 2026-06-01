"use client";

import * as React from "react";
import { Copy, Download, KeyRound, RefreshCw, ShieldCheck, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { SpinnerLabel } from "@/components/ui/spinner";
import {
  SettingsFieldList,
  SettingsFieldRow,
  SettingsPage,
  SettingsSection,
  SettingsSectionSeparator,
} from "@/shared/components/settings-layout";
import {
  createOpenAPIKey,
  deleteOpenAPIKey,
  getOpenAPIKey,
  regenerateOpenAPIKey,
  type OpenAPIKeyDTO,
} from "@/shared/api/openapi-key";
import { getCurrentTwoFactorStatus } from "@/shared/api/auth";
import { resolveApiBaseURL } from "@/shared/api/http-client";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { cn } from "@/lib/utils";
import { buildOpenAPIKeyExportText } from "@/features/settings/model/openapi-key-export";

function ValueBox({
  value,
  action,
  muted,
}: {
  value: string;
  action?: React.ReactNode;
  muted?: boolean;
}) {
  return (
    <div
      className={cn(
        "flex min-h-9 w-full min-w-0 items-center justify-between gap-2 rounded-md border bg-background px-2.5 py-1.5 text-xs",
        muted ? "text-muted-foreground" : "font-mono",
      )}
    >
      <span className="min-w-0 truncate">{value}</span>
      {action ? <span className="shrink-0">{action}</span> : null}
    </div>
  );
}

function statusLabel(keyView: OpenAPIKeyDTO | null, t: ReturnType<typeof useTranslations>): string {
  if (!keyView?.exists) return t("status.none");
  return keyView.status === "active" ? t("status.active") : t("status.revoked");
}

function formatTime(value: string | undefined, fallback: string): string {
  if (!value) return fallback;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return fallback;
  return new Intl.DateTimeFormat(undefined, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function SettingsOpenAPI() {
  const t = useTranslations("settings.openAPIPage");
  const translateError = useLocalizedErrorMessage();
  const { accessToken } = useAuthSession();
  const [keyView, setKeyView] = React.useState<OpenAPIKeyDTO | null>(null);
  const [twoFactorEnabled, setTwoFactorEnabled] = React.useState(false);
  const [loading, setLoading] = React.useState(true);
  const [mutating, setMutating] = React.useState(false);
  const [regenerateOpen, setRegenerateOpen] = React.useState(false);
  const [deleteOpen, setDeleteOpen] = React.useState(false);
  const baseURL = React.useMemo(() => `${resolveApiBaseURL()}/v1`, []);

  React.useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void Promise.all([getOpenAPIKey(accessToken), getCurrentTwoFactorStatus(accessToken)])
      .then(([view, twoFactorStatus]) => {
        if (!cancelled) {
          setKeyView(view);
          setTwoFactorEnabled(Boolean(twoFactorStatus.totpEnabled));
        }
      })
      .catch((error) => {
        if (!cancelled) {
          toast.error(t("toast.loadFailed"), { description: translateError(error) });
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [accessToken, t, translateError]);

  const copyText = React.useCallback(
    async (value: string, label: string) => {
      try {
        await navigator.clipboard.writeText(value);
        toast.success(t("toast.copied", { label }));
      } catch {
        toast.error(t("toast.copyFailed"));
      }
    },
    [t],
  );

  const exportKey = React.useCallback(() => {
    const apiKey = keyView?.apiKey?.trim();
    if (!apiKey) {
      toast.error(t("toast.exportUnavailable"));
      return;
    }
    const blob = new Blob([buildOpenAPIKeyExportText(apiKey, baseURL)], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "deeix-openapi-key.txt";
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
    toast.success(t("toast.exported"));
  }, [baseURL, keyView?.apiKey, t]);

  const createKey = React.useCallback(async () => {
    if (!twoFactorEnabled) {
      toast.error(t("toast.twoFactorRequired"));
      return;
    }
    setMutating(true);
    try {
      const view = await createOpenAPIKey(accessToken);
      setKeyView(view);
      toast.success(t("toast.created"));
    } catch (error) {
      toast.error(t("toast.createFailed"), { description: translateError(error) });
    } finally {
      setMutating(false);
    }
  }, [accessToken, t, translateError, twoFactorEnabled]);

  const regenerateKey = React.useCallback(async () => {
    if (!twoFactorEnabled) {
      toast.error(t("toast.twoFactorRequired"));
      return;
    }
    setMutating(true);
    try {
      const view = await regenerateOpenAPIKey(accessToken);
      setKeyView(view);
      setRegenerateOpen(false);
      toast.success(t("toast.regenerated"));
    } catch (error) {
      toast.error(t("toast.regenerateFailed"), { description: translateError(error) });
    } finally {
      setMutating(false);
    }
  }, [accessToken, t, translateError, twoFactorEnabled]);

  const deleteKey = React.useCallback(async () => {
    setMutating(true);
    try {
      const view = await deleteOpenAPIKey(accessToken);
      setKeyView(view);
      setDeleteOpen(false);
      toast.success(t("toast.deleted"));
    } catch (error) {
      toast.error(t("toast.deleteFailed"), { description: translateError(error) });
    } finally {
      setMutating(false);
    }
  }, [accessToken, t, translateError]);

  const active = keyView?.exists && keyView.status === "active";
  const apiKey = active ? (keyView?.apiKey ?? "") : "";
  const canExport = Boolean(active && keyView?.exportable && apiKey);
  const createDisabled = loading || mutating || !twoFactorEnabled;
  const regenerateDisabled = loading || mutating || !twoFactorEnabled;

  return (
    <SettingsPage>
      <SettingsSection
        title={t("title")}
        actions={
          active ? (
            <>
              <Button type="button" variant="outline" size="sm" disabled={regenerateDisabled} onClick={() => setRegenerateOpen(true)}>
                <RefreshCw className="size-4" />
                {t("actions.regenerate")}
              </Button>
              <Button type="button" variant="outline" size="sm" disabled={loading || mutating} onClick={() => setDeleteOpen(true)}>
                <Trash2 className="size-4" />
                {t("actions.disable")}
              </Button>
            </>
          ) : (
            <Button type="button" size="sm" disabled={createDisabled} onClick={createKey}>
              {mutating ? <SpinnerLabel>{t("actions.creating")}</SpinnerLabel> : <><KeyRound className="size-4" />{t("actions.create")}</>}
            </Button>
          )
        }
      >
        <SettingsFieldList>
          <SettingsFieldRow title={t("fields.status")} controlClassName="md:w-56">
            <div className="flex h-9 w-full items-center justify-end">
              <Badge variant={active ? "default" : "outline"}>{loading ? t("status.loading") : statusLabel(keyView, t)}</Badge>
            </div>
          </SettingsFieldRow>
          <SettingsFieldRow title={t("fields.keyPrefix")} controlClassName="md:w-72">
            <ValueBox value={keyView?.keyPrefix || t("empty")} muted={!keyView?.keyPrefix} />
          </SettingsFieldRow>
          <SettingsFieldRow title={t("fields.lastUsedAt")} controlClassName="md:w-72">
            <ValueBox value={formatTime(keyView?.lastUsedAt, t("never"))} muted />
          </SettingsFieldRow>
          <SettingsFieldRow title={t("fields.baseURL")} controlClassName="md:w-72">
            <ValueBox
              value={baseURL}
              action={
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-7"
                  aria-label={t("copyBaseURL")}
                  onClick={() => copyText(baseURL, t("fields.baseURL"))}
                >
                  <Copy className="size-3.5" />
                </Button>
              }
            />
          </SettingsFieldRow>
        </SettingsFieldList>
        <p className="mt-3 text-xs text-muted-foreground">{t("formatHint")}</p>
        {!loading && !twoFactorEnabled ? (
          <Alert className="mt-4">
            <ShieldCheck className="size-4" />
            <AlertTitle>{t("twoFactor.title")}</AlertTitle>
            <AlertDescription>{t("twoFactor.description")}</AlertDescription>
          </Alert>
        ) : null}
      </SettingsSection>

      {active ? (
        <>
          <SettingsSectionSeparator />
          <SettingsSection
            title={t("key.title")}
            actions={
              canExport ? (
                <Button type="button" variant="outline" size="sm" disabled={loading || mutating} onClick={exportKey}>
                  <Download className="size-4" />
                  {t("actions.export")}
                </Button>
              ) : null
            }
          >
            <SettingsFieldRow title={t("key.key")} controlClassName="md:w-[34rem]">
              <ValueBox
                value={apiKey || t("key.hidden")}
                muted={!apiKey}
                action={
                  apiKey ? (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-7"
                      aria-label={t("copyAPIKey")}
                      onClick={() => copyText(apiKey, t("key.key"))}
                    >
                      <Copy className="size-3.5" />
                    </Button>
                  ) : null
                }
              />
            </SettingsFieldRow>
            {!canExport ? <p className="mt-3 text-xs text-muted-foreground">{t("key.exportRequiresTwoFactor")}</p> : null}
          </SettingsSection>
        </>
      ) : null}

      <AlertDialog open={regenerateOpen} onOpenChange={setRegenerateOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("regenerateDialog.title")}</AlertDialogTitle>
            <AlertDialogDescription>{t("regenerateDialog.description")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={mutating}>{t("actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction disabled={mutating} onClick={(event) => {
              event.preventDefault();
              void regenerateKey();
            }}>
              {mutating ? t("actions.saving") : t("actions.regenerate")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteDialog.title")}</AlertDialogTitle>
            <AlertDialogDescription>{t("deleteDialog.description")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={mutating}>{t("actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction disabled={mutating} onClick={(event) => {
              event.preventDefault();
              void deleteKey();
            }}>
              {mutating ? t("actions.saving") : t("actions.disable")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsPage>
  );
}
