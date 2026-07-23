"use client";

import * as React from "react";
import Link from "next/link";
import { ArrowRight, Plus } from "lucide-react";
import { useReducedMotion } from "motion/react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ProviderLogo } from "@/features/landing/components/provider-logo";
import { LANDING_PROVIDERS } from "@/features/landing/model/providers";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import { cn } from "@/lib/utils";
import { AppLogo } from "@/shared/components/app-logo";
import { PublicPageHeader } from "@/shared/components/public-page-header";

const DEMO_STEPS = [
  { key: "gpt", model: { slug: "openai", label: "GPT" } },
  { key: "claude", model: { slug: "claude", label: "Claude" } },
  { key: "gemini", model: { slug: "gemini", label: "Gemini" } },
  { key: "grok", model: { slug: "grok", label: "Grok" } },
] as const;

const CAPABILITY_KEYS = ["switching", "connection", "pricing"] as const;

function ProviderCarouselGroup({ duplicate = false }: { duplicate?: boolean }) {
  return (
    <div
      className={cn(
        "flex shrink-0 items-center gap-1 pr-1",
        duplicate && "landing-provider-carousel-duplicate",
      )}
      role={duplicate ? undefined : "list"}
      aria-hidden={duplicate || undefined}
    >
      {LANDING_PROVIDERS.map((provider) => (
        <span key={provider.slug} role={duplicate ? undefined : "listitem"}>
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="inline-flex size-10 items-center justify-center rounded-full transition-colors hover:bg-muted/70 motion-reduce:transition-none">
                <ProviderLogo provider={provider} size={21} eager={!duplicate} iconOnly />
              </span>
            </TooltipTrigger>
            <TooltipContent sideOffset={6}>{provider.label}</TooltipContent>
          </Tooltip>
        </span>
      ))}
    </div>
  );
}

function ProviderCarousel() {
  const t = useTranslations("landing.hero");

  return (
    <div className="mt-7 flex min-w-0 items-center gap-3">
      <span className="shrink-0 text-xs font-medium tracking-wide text-muted-foreground">
        {t("providerCount")}
      </span>
      <div
        className="landing-provider-carousel min-w-0 max-w-[21rem] flex-1"
        aria-label={t("providerCount")}
      >
        <div className="landing-provider-carousel-track">
          <ProviderCarouselGroup />
          <ProviderCarouselGroup duplicate />
        </div>
      </div>
    </div>
  );
}

function InteractiveDemo() {
  const t = useTranslations("landing.demo");
  const reduceMotion = useReducedMotion() ?? false;
  const [activeIndex, setActiveIndex] = React.useState(0);
  const [autoPlay, setAutoPlay] = React.useState(true);
  const [isInView, setIsInView] = React.useState(true);
  const demoRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    const target = demoRef.current;
    if (!target || !("IntersectionObserver" in window)) {
      return undefined;
    }

    const observer = new IntersectionObserver(
      ([entry]) => setIsInView(entry?.isIntersecting ?? false),
      { threshold: 0.3 },
    );
    observer.observe(target);
    return () => observer.disconnect();
  }, []);

  React.useEffect(() => {
    if (reduceMotion || !autoPlay || !isInView) {
      return undefined;
    }

    const timer = window.setInterval(() => {
      setActiveIndex((current) => (current + 1) % DEMO_STEPS.length);
    }, 4_500);
    return () => window.clearInterval(timer);
  }, [autoPlay, isInView, reduceMotion]);

  const activeStep = DEMO_STEPS[activeIndex];

  return (
    <div
      ref={demoRef}
      className="relative overflow-hidden rounded-[1.5rem] border border-border/70 bg-card/86 p-4 shadow-[0_24px_80px_-48px_color-mix(in_oklch,var(--foreground)_40%,transparent)] sm:p-5"
      data-interactive-demo
      data-demo-autoplay={!reduceMotion && autoPlay && isInView}
    >
      <div className="flex items-center justify-between gap-4 border-b border-border/60 pb-4">
        <div className="flex items-center gap-2">
          <span className="relative flex size-2" aria-hidden>
            <span className="absolute inline-flex size-full animate-ping rounded-full bg-primary/50 motion-reduce:animate-none" />
            <span className="relative inline-flex size-2 rounded-full bg-primary" />
          </span>
          <span className="text-xs font-semibold tracking-[0.12em] text-foreground uppercase">
            {t("label")}
          </span>
        </div>
        <span className="rounded-full border border-border/70 px-2.5 py-1 text-[0.6875rem] font-medium text-muted-foreground">
          {t("round", { current: activeIndex + 1, total: DEMO_STEPS.length })}
        </span>
      </div>

      <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-4" aria-label={t("controlsLabel")}>
        {DEMO_STEPS.map((step, index) => {
          const selected = index === activeIndex;
          return (
            <button
              key={step.key}
              type="button"
              aria-pressed={selected}
              className={cn(
                "flex min-h-11 items-center justify-center gap-2 rounded-xl border px-2.5 text-xs font-semibold outline-none transition-colors focus-visible:ring-[3px] focus-visible:ring-ring/50 motion-reduce:transition-none",
                selected
                  ? "border-primary/45 bg-primary/10 text-primary"
                  : "border-border/60 bg-background/45 text-muted-foreground hover:border-border hover:text-foreground",
              )}
              onClick={() => {
                setActiveIndex(index);
                setAutoPlay(false);
              }}
            >
              <ProviderLogo provider={step.model} size={17} eager />
              <span>{step.model.label}</span>
            </button>
          );
        })}
      </div>

      <div className="mt-4 min-h-[19rem] rounded-2xl bg-muted/42 p-4 sm:min-h-[17.5rem] sm:p-5">
        <div className="flex gap-3">
          <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-foreground text-[0.65rem] font-bold text-background">
            {t("userInitial")}
          </span>
          <div className="min-w-0">
            <div className="text-xs font-semibold text-muted-foreground">{t("userLabel")}</div>
            <p className="mt-1 text-pretty text-sm leading-6 text-foreground">{t("userPrompt")}</p>
          </div>
        </div>

        <div
          key={activeStep.key}
          className="mt-5 flex gap-3 border-t border-border/55 pt-5"
          aria-live="polite"
        >
          <span className="flex size-8 shrink-0 items-center justify-center rounded-full border border-border/70 bg-background">
            <ProviderLogo provider={activeStep.model} size={17} eager />
          </span>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
              <span className="text-xs font-semibold text-foreground">{activeStep.model.label}</span>
              <span className="text-[0.6875rem] text-muted-foreground">
                {t(`steps.${activeStep.key}.role`)}
              </span>
            </div>
            <h3 className="mt-2 text-balance text-base font-semibold leading-6 text-foreground">
              {t(`steps.${activeStep.key}.title`)}
            </h3>
            <p className="mt-2 text-pretty text-sm leading-6 text-muted-foreground">
              {t(`steps.${activeStep.key}.body`)}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

function HeroSection() {
  const t = useTranslations("landing.hero");
  const { locale } = useAppLocale();

  return (
    <section className="relative mx-auto grid min-h-[calc(100svh-4rem)] w-full max-w-[1180px] grid-cols-1 items-center gap-12 px-4 py-16 sm:px-6 sm:py-20 lg:grid-cols-2 lg:gap-16 lg:px-8 lg:py-24 xl:px-0">
      <div className="relative z-10 min-w-0">
        <p className="mb-5 text-xs font-semibold tracking-[0.18em] text-primary uppercase">
          {t("eyebrow")}
        </p>
        <h1
          className={cn(
            "font-semibold tracking-[-0.045em] text-foreground",
            locale === "zh-CN"
              ? "text-[clamp(3rem,7.5vw,5.5rem)] leading-[1.05]"
              : locale === "ja-JP"
                ? "max-w-full break-keep text-[clamp(2.75rem,5vw,4.25rem)] leading-[1.08]"
                : "max-w-full text-[clamp(3rem,5.5vw,4.75rem)] leading-[1.02]",
          )}
        >
          {locale === "zh-CN" ? (
            <>
              <span className="block whitespace-nowrap">{t("headlineLine1")}</span>
              <span className="block whitespace-nowrap">{t("headlineLine2")}</span>
            </>
          ) : t("headline")}
        </h1>
        <p className="mt-7 max-w-[38rem] text-balance text-base leading-7 text-muted-foreground sm:text-[1.0625rem] sm:leading-8">
          {t("description")}
        </p>
        <Button asChild className="mt-8 h-12 rounded-full px-6 text-sm font-semibold">
          <Link href="/login">
            {t("cta")}
            <ArrowRight className="size-4" aria-hidden />
          </Link>
        </Button>
        <ProviderCarousel />
      </div>

      <div className="relative z-10 min-w-0">
        <div className="absolute inset-0 -z-10 rounded-full bg-primary/[0.055] blur-3xl" aria-hidden />
        <InteractiveDemo />
      </div>
    </section>
  );
}

function ProviderSection() {
  const t = useTranslations("landing.providers");

  return (
    <section className="border-y border-border/60 bg-card/26">
      <div className="mx-auto w-full max-w-[1180px] px-4 py-20 sm:px-6 sm:py-24 lg:px-8 xl:px-0">
        <div className="max-w-2xl">
          <p className="text-xs font-semibold tracking-[0.18em] text-primary uppercase">{t("eyebrow")}</p>
          <h2 className="mt-4 text-3xl font-semibold tracking-[-0.035em] text-foreground sm:text-4xl">
            {t("title")}
          </h2>
          <p className="mt-4 text-pretty text-base leading-7 text-muted-foreground">{t("description")}</p>
        </div>

        <div className="mt-10 grid grid-cols-3 border-l border-t border-border/65 md:grid-cols-4 lg:grid-cols-6">
          {LANDING_PROVIDERS.map((provider) => (
            <div
              key={provider.slug}
              className="flex h-[4.5rem] min-w-0 items-center gap-2.5 border-b border-r border-border/65 px-3 sm:px-4"
            >
              <ProviderLogo provider={provider} size={23} />
              <span className="min-w-0 truncate text-xs font-medium text-foreground/80 sm:text-sm">
                {provider.label}
              </span>
            </div>
          ))}
          <div className="flex h-[4.5rem] min-w-0 items-center gap-2.5 border-b border-r border-border/65 px-3 text-muted-foreground sm:px-4">
            <Plus className="size-[23px] shrink-0" strokeWidth={1.5} aria-hidden />
            <span className="min-w-0 truncate text-xs font-medium sm:text-sm">{t("ongoing")}</span>
          </div>
        </div>
      </div>
    </section>
  );
}

function CapabilitySection() {
  const t = useTranslations("landing.capabilities");

  return (
    <section className="mx-auto w-full max-w-[1180px] px-4 py-20 sm:px-6 sm:py-24 lg:px-8 xl:px-0">
      <div className="max-w-xl">
        <p className="text-xs font-semibold tracking-[0.18em] text-primary uppercase">{t("eyebrow")}</p>
        <h2 className="mt-4 text-3xl font-semibold tracking-[-0.035em] text-foreground sm:text-4xl">
          {t("title")}
        </h2>
      </div>
      <ol className="mt-12 grid grid-cols-1 border-t border-border/70 md:grid-cols-3">
        {CAPABILITY_KEYS.map((key, index) => (
          <li
            key={key}
            className="border-b border-border/70 py-8 md:border-b-0 md:border-r md:px-8 md:first:pl-0 md:last:border-r-0 md:last:pr-0"
          >
            <span className="font-mono text-xs text-primary">0{index + 1}</span>
            <h3 className="mt-5 text-xl font-semibold tracking-[-0.02em] text-foreground">
              {t(`${key}.title`)}
            </h3>
            <p className="mt-3 text-pretty text-sm leading-7 text-muted-foreground">
              {t(`${key}.description`)}
            </p>
          </li>
        ))}
      </ol>
    </section>
  );
}

function FinalCTASection() {
  const t = useTranslations("landing.finalCta");

  return (
    <section className="mx-auto w-full max-w-[1180px] px-4 pb-20 sm:px-6 sm:pb-24 lg:px-8 xl:px-0">
      <div className="overflow-hidden rounded-[1.75rem] bg-foreground px-6 py-14 text-background sm:px-12 sm:py-16 lg:flex lg:items-center lg:justify-between lg:gap-12">
        <div>
          <h2 className="text-3xl font-semibold tracking-[-0.04em] sm:text-4xl">{t("title")}</h2>
          <p className="mt-4 max-w-xl text-pretty text-sm leading-7 opacity-70 sm:text-base">{t("description")}</p>
        </div>
        <Button
          asChild
          className="mt-8 h-12 rounded-full bg-background px-6 text-sm font-semibold text-foreground hover:bg-background/90 lg:mt-0"
        >
          <Link href="/login">
            {t("button")}
            <ArrowRight className="size-4" aria-hidden />
          </Link>
        </Button>
      </div>
    </section>
  );
}

function LandingFooter() {
  const t = useTranslations("landing.footer");

  return (
    <footer className="border-t border-border/60">
      <div className="mx-auto flex min-h-24 w-full max-w-[1180px] flex-col items-start justify-center gap-4 px-4 py-6 sm:flex-row sm:items-center sm:justify-between sm:px-6 lg:px-8 xl:px-0">
        <AppLogo height={48} className="text-foreground" />
        <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-xs text-muted-foreground">
          <span>{t("copyright", { year: new Date().getFullYear() })}</span>
          <a
            href="https://docs.comiai.cc"
            target="_blank"
            rel="noreferrer"
            className="rounded-sm font-medium underline-offset-4 hover:text-foreground hover:underline focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            {t("docs")}
          </a>
        </div>
      </div>
    </footer>
  );
}

export function LandingPage() {
  return (
    <main className="h-svh overflow-x-hidden overflow-y-auto bg-background text-foreground">
      <div className="relative isolate">
        <div
          className="pointer-events-none absolute inset-x-0 top-0 -z-10 h-[52rem] opacity-70"
          style={{
            background:
              "radial-gradient(48rem 34rem at 82% 10%, color-mix(in oklch, var(--primary) 10%, transparent), transparent 68%), radial-gradient(34rem 28rem at 8% 28%, color-mix(in oklch, var(--foreground) 4%, transparent), transparent 72%)",
          }}
          aria-hidden
        />
        <PublicPageHeader />
        <HeroSection />
        <ProviderSection />
        <CapabilitySection />
        <FinalCTASection />
        <LandingFooter />
      </div>
    </main>
  );
}
