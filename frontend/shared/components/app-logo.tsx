"use client";

import type { CSSProperties } from "react";

import { cn } from "@/lib/utils";
import { brandAssets, brandText } from "@/shared/lib/branding";

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

function logoStyle(height: number, width: number | undefined): CSSProperties {
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
    backgroundImage: brandAssets.logo ? `url("${brandAssets.logo}")` : undefined,
    backgroundPosition: "center",
    backgroundRepeat: "no-repeat",
    backgroundSize: "contain",
  };
}

export function AppLogo({
  alt = brandText.title,
  width,
  height,
  priority: _priority,
  className,
}: AppLogoProps) {
  return (
    <span
      role="img"
      aria-label={alt}
      title={alt}
      className={cn(
        "inline-block shrink-0 bg-current text-foreground align-middle leading-none",
        className,
      )}
      style={logoStyle(height, width)}
    />
  );
}

// Kept as a compatibility alias for older callers; it uses the Comi-configured asset.
export function DeeixLogo({ alt = brandText.title, ...props }: AppLogoProps) {
  return <AppLogo {...props} alt={alt} />;
}
