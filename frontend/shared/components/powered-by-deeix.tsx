"use client";

import { cn } from "@/lib/utils";
import { AppLogo } from "@/shared/components/app-logo";
import { useBranding } from "@/shared/config/branding-provider";

export function PoweredByDeeix({ className }: { className?: string }) {
  const branding = useBranding();
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-1.5 text-[11px] font-medium leading-none text-muted-foreground/70",
        className,
      )}
    >
      <span>Powered by</span>
      <a
        href="/"
        aria-label={branding.title}
        className="inline-flex shrink-0 rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:ring-offset-2"
      >
        <AppLogo
          alt={branding.title}
          width={58}
          height={18}
          className="h-3.5 w-auto opacity-65"
        />
      </a>
    </span>
  );
}

export function CustomBrandAttribution({ className }: { className?: string }) {
  const branding = useBranding();
  if (!branding.logoURL) {
    return null;
  }
  return (
    <div className={className}>
      <PoweredByDeeix />
    </div>
  );
}
