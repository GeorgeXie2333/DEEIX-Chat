import { pwaAsset } from "@/shared/pwa/assets";

const DEFAULT_BRAND_TITLE = "Comi AI";
const LEGACY_BRAND_TITLES = ["DEEIX Chat", DEFAULT_BRAND_TITLE] as const;

function assetURL(configured: string | undefined, fallback: string): string {
  return configured?.trim() || fallback;
}

export const brandText = {
  title: process.env.NEXT_PUBLIC_BRAND_TITLE?.trim() || DEFAULT_BRAND_TITLE,
  shortName: process.env.NEXT_PUBLIC_BRAND_SHORT_NAME?.trim() || "Comi",
  description:
    process.env.NEXT_PUBLIC_BRAND_DESCRIPTION?.trim() ||
    "Comi AI is a multi-model AI conversation system.",
} as const;

export function replaceDefaultBrandTitle(value: string): string {
  return LEGACY_BRAND_TITLES.reduce(
    (result, legacyTitle) => result.replaceAll(legacyTitle, () => brandText.title),
    value,
  );
}

export const brandAssets = {
  logo: process.env.NEXT_PUBLIC_LOGO_URL?.trim() || undefined,
  favicon: assetURL(process.env.NEXT_PUBLIC_FAVICON_URL, "/favicon.ico"),
  pwaIcon192: assetURL(
    process.env.NEXT_PUBLIC_PWA_ICON_192_URL,
    pwaAsset("/pwa/icon-192.png"),
  ),
  pwaIcon512: assetURL(
    process.env.NEXT_PUBLIC_PWA_ICON_512_URL,
    pwaAsset("/pwa/icon-512.png"),
  ),
  pwaMaskableIcon512: assetURL(
    process.env.NEXT_PUBLIC_PWA_MASKABLE_ICON_512_URL,
    pwaAsset("/pwa/icon-maskable-512.png"),
  ),
  appleTouchIcon180: assetURL(
    process.env.NEXT_PUBLIC_APPLE_TOUCH_ICON_180_URL,
    pwaAsset("/pwa/apple-touch-icon.png"),
  ),
} as const;
