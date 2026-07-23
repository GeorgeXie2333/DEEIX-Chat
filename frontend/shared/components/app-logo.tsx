"use client";

import type { CSSProperties } from "react";

import { cn } from "@/lib/utils";
import { useBranding } from "@/shared/config/branding-provider";

import {
  appLogoMaskImage,
  appLogoVisualHeight,
  appLogoVisualWidth,
  customAppLogoSource,
} from "./app-logo-assets";

type AppLogoProps = {
  alt?: string;
  width?: number;
  height: number;
  priority?: boolean;
  className?: string;
};

function logoStyle(height: number, width: number | undefined, logoURL: string): CSSProperties {
  const customLogoSource = customAppLogoSource(logoURL);

  return {
    width: width ?? appLogoVisualWidth(height),
    height: width ? height : appLogoVisualHeight(height),
    maskImage: customLogoSource ? undefined : appLogoMaskImage(),
    maskPosition: "center",
    maskRepeat: "no-repeat",
    maskSize: "contain",
    WebkitMaskImage: customLogoSource ? undefined : appLogoMaskImage(),
    WebkitMaskPosition: "center",
    WebkitMaskRepeat: "no-repeat",
    WebkitMaskSize: "contain",
    backgroundImage: customLogoSource
      ? `url(${JSON.stringify(customLogoSource)})`
      : undefined,
    backgroundPosition: "center",
    backgroundRepeat: "no-repeat",
    backgroundSize: "contain",
  };
}

export function AppLogo({
  alt,
  width,
  height,
  priority: _priority,
  className,
}: AppLogoProps) {
  const branding = useBranding();
  const label = alt ?? branding.title;
  const hasCustomLogo = Boolean(customAppLogoSource(branding.logoURL));

  return (
    <span
      role="img"
      aria-label={label}
      title={label}
      className={cn(
        "inline-block shrink-0 align-middle leading-none",
        hasCustomLogo ? undefined : "bg-current",
        className,
      )}
      style={logoStyle(height, width, branding.logoURL)}
    />
  );
}

// Kept as a compatibility alias for older callers; it uses the Comi-configured asset.
export function DeeixLogo({ alt, ...props }: AppLogoProps) {
  return <AppLogo {...props} alt={alt} />;
}
