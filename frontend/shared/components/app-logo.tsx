"use client";

import type { CSSProperties } from "react";

import { cn } from "@/lib/utils";
import { useBranding } from "@/shared/config/branding-provider";

import {
  appLogoMaskImage,
  appLogoVisualHeight,
  appLogoVisualWidth,
} from "./app-logo-assets";

type AppLogoProps = {
  alt?: string;
  width?: number;
  height: number;
  priority?: boolean;
  className?: string;
};

function logoStyle(height: number, width: number | undefined, logoURL: string): CSSProperties {
  return {
    width: width ?? appLogoVisualWidth(height),
    height: width ? height : appLogoVisualHeight(height),
    maskImage: appLogoMaskImage(),
    maskPosition: "center",
    maskRepeat: "no-repeat",
    maskSize: "contain",
    WebkitMaskImage: appLogoMaskImage(),
    WebkitMaskPosition: "center",
    WebkitMaskRepeat: "no-repeat",
    WebkitMaskSize: "contain",
    backgroundImage: logoURL ? `url("${logoURL}")` : undefined,
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
  return (
    <span
      role="img"
      aria-label={label}
      title={label}
      className={cn(
        "inline-block shrink-0 bg-current text-foreground align-middle leading-none",
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
