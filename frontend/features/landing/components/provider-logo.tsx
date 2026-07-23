"use client";

import * as React from "react";
import { Bot } from "lucide-react";

import {
  lobeHubIconCDNURL,
  lobeHubIconLocalURL,
  type LandingProvider,
} from "@/features/landing/model/providers";
import { cn } from "@/lib/utils";

type ProviderLogoProps = {
  provider: Pick<LandingProvider, "slug" | "label">;
  size?: number;
  eager?: boolean;
  iconOnly?: boolean;
  className?: string;
};

type IconSource = "cdn" | "local" | "generic";

export function ProviderLogo({
  provider,
  size = 24,
  eager = false,
  iconOnly = false,
  className,
}: ProviderLogoProps) {
  const [source, setSource] = React.useState<IconSource>("cdn");

  React.useEffect(() => {
    setSource("cdn");
  }, [provider.slug]);

  const sharedClassName = cn(
    "shrink-0 opacity-70 transition-opacity duration-200 dark:invert motion-reduce:transition-none",
    className,
  );

  return (
    <span
      className="inline-flex shrink-0 items-center justify-center"
      style={{ width: size, height: size }}
      title={provider.label}
      data-provider-logo={provider.slug}
      data-icon-source={source}
    >
      {source === "generic" ? (
        <Bot
          aria-hidden
          className={sharedClassName}
          style={{ width: size, height: size }}
          strokeWidth={1.7}
        />
      ) : (
        <img
          src={source === "cdn" ? lobeHubIconCDNURL(provider.slug) : lobeHubIconLocalURL(provider.slug)}
          alt=""
          width={size}
          height={size}
          loading={eager ? "eager" : "lazy"}
          fetchPriority={eager ? "high" : "auto"}
          decoding="async"
          className={sharedClassName}
          onError={() => {
            setSource((current) => current === "cdn" ? "local" : "generic");
          }}
        />
      )}
      {iconOnly ? <span className="sr-only">{provider.label}</span> : null}
    </span>
  );
}
