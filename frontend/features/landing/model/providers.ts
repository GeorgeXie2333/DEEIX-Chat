export const LOBEHUB_ICON_VERSION = "1.90.0";
export const LOBEHUB_ICON_CDN_BASE =
  `https://cdn.jsdelivr.net/npm/@lobehub/icons-static-svg@${LOBEHUB_ICON_VERSION}/icons`;
export const LOBEHUB_ICON_LOCAL_BASE = "/vendor/lobehub-icons";

export type LandingProvider = {
  slug: string;
  label: string;
  featured: boolean;
  demoModel?: string;
};

export const LANDING_PROVIDERS = [
  { slug: "openai", label: "OpenAI", featured: true, demoModel: "GPT" },
  { slug: "anthropic", label: "Anthropic", featured: true, demoModel: "Claude" },
  { slug: "google", label: "Google", featured: true, demoModel: "Gemini" },
  { slug: "xai", label: "xAI", featured: true, demoModel: "Grok" },
  { slug: "deepseek", label: "DeepSeek", featured: true },
  { slug: "zhipu", label: "ZhiPu", featured: true },
  { slug: "alibaba", label: "Alibaba", featured: false },
  { slug: "moonshot", label: "MoonShot", featured: false },
  { slug: "minimax", label: "MiniMax", featured: false },
  { slug: "stepfun", label: "StepFun", featured: false },
  { slug: "meta", label: "Meta", featured: false },
] as const satisfies readonly LandingProvider[];

export const FEATURED_LANDING_PROVIDERS = LANDING_PROVIDERS.filter((provider) => provider.featured);

export function lobeHubIconCDNURL(slug: string): string {
  return `${LOBEHUB_ICON_CDN_BASE}/${slug}.svg`;
}

export function lobeHubIconLocalURL(slug: string): string {
  return `${LOBEHUB_ICON_LOCAL_BASE}/${slug}.svg`;
}

export function findLandingProvider(slug: string): LandingProvider {
  return LANDING_PROVIDERS.find((provider) => provider.slug === slug) ?? LANDING_PROVIDERS[0];
}
