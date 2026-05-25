"use client";

import { cn } from "@/lib/utils";

type AppLogoProps = {
  alt?: string;
  width: number;
  height: number;
  priority?: boolean;
  className?: string;
};

export function AppLogo({
  alt = "Comi AI",
  height,
  className,
}: AppLogoProps) {
  const fontSize = Math.max(13, Math.min(20, Math.round(height * 0.42)));

  return (
    <span
      aria-label={alt}
      title={alt}
      className={cn(
        "flex min-w-fit items-center justify-center whitespace-nowrap font-semibold leading-none tracking-normal text-foreground",
        className,
      )}
      style={{ fontSize }}
    >
      Comi AI
    </span>
  );
}
