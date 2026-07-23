"use client";

import Link from "next/link";
import { Languages, Monitor, Moon, Sun } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { APP_LOCALE_LABELS, APP_LOCALES, type AppLocale } from "@/i18n/config";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import { AppLogo } from "@/shared/components/app-logo";
import { type Theme, useTheme } from "@/shared/components/theme-provider";

function PublicLanguageMenu() {
  const t = useTranslations("landing.nav");
  const { locale, setLocale } = useAppLocale();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          className="h-11 min-w-11 gap-2 rounded-full px-3 text-sm text-muted-foreground hover:text-foreground sm:px-4"
          aria-label={t("language")}
        >
          <Languages className="size-4" aria-hidden />
          <span className="hidden sm:inline">{APP_LOCALE_LABELS[locale]}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-40">
        <DropdownMenuRadioGroup
          value={locale}
          onValueChange={(value) => {
            void setLocale(value as AppLocale);
          }}
        >
          {APP_LOCALES.map((item) => (
            <DropdownMenuRadioItem key={item} value={item} className="min-h-11 text-sm">
              {APP_LOCALE_LABELS[item]}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function PublicThemeMenu() {
  const t = useTranslations("landing.theme");
  const { theme, setTheme } = useTheme();
  const ThemeIcon = theme === "light" ? Sun : theme === "dark" ? Moon : Monitor;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon-lg"
          className="size-11 rounded-full text-muted-foreground hover:text-foreground"
          aria-label={t("label")}
        >
          <ThemeIcon className="size-4" aria-hidden />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-36">
        <DropdownMenuRadioGroup
          value={theme}
          onValueChange={(value) => setTheme(value as Theme)}
        >
          <DropdownMenuRadioItem value="light" className="min-h-11 text-sm">
            <Sun className="size-4" aria-hidden />
            {t("light")}
          </DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="dark" className="min-h-11 text-sm">
            <Moon className="size-4" aria-hidden />
            {t("dark")}
          </DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="system" className="min-h-11 text-sm">
            <Monitor className="size-4" aria-hidden />
            {t("system")}
          </DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function PublicPageHeader({ showLoginAction = true }: { showLoginAction?: boolean }) {
  const t = useTranslations("landing.nav");

  return (
    <header className="sticky top-0 z-40 h-16 border-b border-border/60 bg-background/88 backdrop-blur-xl">
      <nav
        className="mx-auto flex h-full w-full max-w-[1180px] items-center justify-between px-4 sm:px-6 lg:px-8 xl:px-0"
        aria-label={t("ariaLabel")}
      >
        <Link
          href="/"
          aria-label={t("home")}
          className="inline-flex min-h-11 items-center rounded-md text-foreground outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
        >
          <AppLogo height={58} priority className="text-foreground" />
        </Link>
        <div className="flex items-center gap-0.5 sm:gap-1">
          <PublicLanguageMenu />
          <PublicThemeMenu />
          {showLoginAction ? (
            <Button asChild className="ml-1 h-11 rounded-full px-4 text-sm font-semibold sm:px-5">
              <Link href="/login">{t("login")}</Link>
            </Button>
          ) : null}
        </div>
      </nav>
    </header>
  );
}
