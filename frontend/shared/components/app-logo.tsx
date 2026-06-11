"use client";

import type { CSSProperties } from "react";

import { cn } from "@/lib/utils";

import {
  appLogoMaskImage,
  appLogoVisualHeight,
  appLogoVisualWidth,
} from "./app-logo-assets";

type AppLogoProps = {
  alt?: string;
  height: number;
  priority?: boolean;
  className?: string;
};

export function AppLogo({
  alt = "Comi AI",
  height,
  className,
}: AppLogoProps) {
  const visualHeight = appLogoVisualHeight(height);
  const visualWidth = appLogoVisualWidth(height);
  const style = {
    width: visualWidth,
    height: visualHeight,
    maskImage: appLogoMaskImage(),
    maskPosition: "center",
    maskRepeat: "no-repeat",
    maskSize: "contain",
    WebkitMaskImage: appLogoMaskImage(),
    WebkitMaskPosition: "center",
    WebkitMaskRepeat: "no-repeat",
    WebkitMaskSize: "contain",
  } as CSSProperties;

  return (
    <span
      role="img"
      aria-label={alt}
      title={alt}
      className={cn(
        "inline-block shrink-0 bg-current text-foreground align-middle leading-none",
        className,
      )}
      style={style}
    />
  );
}
