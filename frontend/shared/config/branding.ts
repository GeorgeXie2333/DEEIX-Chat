import type { BrandingDTO } from "@/shared/api/branding";
import { pwaAsset } from "@/shared/pwa/assets";

export const DEFAULT_BRANDING: BrandingDTO = {
  title: "Comi AI",
  shortName: "Comi",
  description: "Comi AI is a multi-model AI conversation system.",
  logoURL: "/logo.svg",
  faviconURL: "/favicon.ico",
  pwaIcon192URL: pwaAsset("/pwa/icon-192.png"),
  pwaIcon512URL: pwaAsset("/pwa/icon-512.png"),
  pwaMaskableIcon512URL: pwaAsset("/pwa/icon-maskable-512.png"),
  appleTouchIcon180URL: pwaAsset("/pwa/apple-touch-icon.png"),
};

let brandingSnapshot = DEFAULT_BRANDING;

export function getBrandingSnapshot(): BrandingDTO {
  return brandingSnapshot;
}

export function setBrandingSnapshot(branding: BrandingDTO): void {
  brandingSnapshot = branding;
}

export function replaceDefaultBrandTitle(value: string, brandTitle: string): string {
  let result = value;
  for (const legacyTitle of ["DEEIX Chat", "Comi AI"]) {
    if (legacyTitle !== brandTitle) {
      result = result.replaceAll(legacyTitle, () => brandTitle);
    }
  }
  return result;
}
