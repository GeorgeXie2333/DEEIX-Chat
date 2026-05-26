export const APP_LOCALES = ["zh-CN", "en-US", "ja-JP"] as const;

export type AppLocale = (typeof APP_LOCALES)[number];

export const DEFAULT_LOCALE: AppLocale = "zh-CN";
export const LOCALE_COOKIE_NAME = "deeix_chat_locale";

export const APP_LOCALE_LABELS: Record<AppLocale, string> = {
  "zh-CN": "简体中文",
  "en-US": "English",
  "ja-JP": "日本語",
};

export function normalizeAppLocale(value: string | null | undefined): AppLocale {
  const normalized = String(value ?? "").trim();
  const canonical = normalized.replace("_", "-");
  const lower = canonical.toLowerCase();
  if (lower === "zh" || lower.startsWith("zh-")) {
    return "zh-CN";
  }
  if (lower === "ja" || lower.startsWith("ja-")) {
    return "ja-JP";
  }
  if (lower === "en" || lower.startsWith("en-")) {
    return "en-US";
  }
  return APP_LOCALES.includes(canonical as AppLocale) ? (canonical as AppLocale) : DEFAULT_LOCALE;
}

export function resolveBrowserLocale(languages: readonly string[] | undefined): AppLocale {
  for (const language of languages ?? []) {
    const normalized = String(language ?? "").trim().toLowerCase().replace("_", "-");
    if (normalized === "zh" || normalized.startsWith("zh-")) {
      return "zh-CN";
    }
    if (normalized === "ja" || normalized.startsWith("ja-")) {
      return "ja-JP";
    }
    if (normalized === "en" || normalized.startsWith("en-")) {
      return "en-US";
    }
  }
  return DEFAULT_LOCALE;
}
