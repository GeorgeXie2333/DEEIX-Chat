"use client";

import * as React from "react";
import { Copy, KeyRound, RefreshCw, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

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
import { resolveApiBaseURL } from "@/shared/api/http-client";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { cn } from "@/lib/utils";

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
  const [freshKey, setFreshKey] = React.useState("");
  const [loading, setLoading] = React.useState(true);
  const [mutating, setMutating] = React.useState(false);
  const [regenerateOpen, setRegenerateOpen] = React.useState(false);
  const [deleteOpen, setDeleteOpen] = React.useState(false);
  const baseURL = React.useMemo(() => `${resolveApiBaseURL()}/v1`, []);

  React.useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void getOpenAPIKey(accessToken)
      .then((view) => {
        if (!cancelled) {
          setKeyView(view);
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

  const createKey = React.useCallback(async () => {
    setMutating(true);
    try {
      const view = await createOpenAPIKey(accessToken);
      setKeyView(view);
      setFreshKey(view.apiKey ?? "");
      toast.success(t("toast.created"));
    } catch (error) {
      toast.error(t("toast.createFailed"), { description: translateError(error) });
    } finally {
      setMutating(false);
    }
  }, [accessToken, t, translateError]);

  const regenerateKey = React.useCallback(async () => {
    setMutating(true);
    try {
      const view = await regenerateOpenAPIKey(accessToken);
      setKeyView(view);
      setFreshKey(view.apiKey ?? "");
      setRegenerateOpen(false);
      toast.success(t("toast.regenerated"));
    } catch (error) {
      toast.error(t("toast.regenerateFailed"), { description: translateError(error) });
    } finally {
      setMutating(false);
    }
  }, [accessToken, t, translateError]);

  const deleteKey = React.useCallback(async () => {
    setMutating(true);
    try {
      const view = await deleteOpenAPIKey(accessToken);
      setKeyView(view);
      setFreshKey("");
      setDeleteOpen(false);
      toast.success(t("toast.deleted"));
    } catch (error) {
      toast.error(t("toast.deleteFailed"), { description: translateError(error) });
    } finally {
      setMutating(false);
    }
  }, [accessToken, t, translateError]);

  const active = keyView?.exists && keyView.status === "active";

  return (
    <SettingsPage>
      <SettingsSection
        title={t("title")}
        actions={
          active ? (
            <>
              <Button type="button" variant="outline" size="sm" disabled={loading || mutating} onClick={() => setRegenerateOpen(true)}>
                <RefreshCw className="size-4" />
                {t("actions.regenerate")}
              </Button>
              <Button type="button" variant="outline" size="sm" disabled={loading || mutating} onClick={() => setDeleteOpen(true)}>
                <Trash2 className="size-4" />
                {t("actions.disable")}
              </Button>
            </>
          ) : (
            <Button type="button" size="sm" disabled={loading || mutating} onClick={createKey}>
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
      </SettingsSection>

      {freshKey ? (
        <>
          <SettingsSectionSeparator />
          <SettingsSection title={t("fresh.title")}>
            <SettingsFieldRow title={t("fresh.key")} controlClassName="md:w-[34rem]">
              <ValueBox
                value={freshKey}
                action={
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    aria-label={t("copyAPIKey")}
                    onClick={() => copyText(freshKey, t("fresh.key"))}
                  >
                    <Copy className="size-3.5" />
                  </Button>
                }
              />
            </SettingsFieldRow>
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
