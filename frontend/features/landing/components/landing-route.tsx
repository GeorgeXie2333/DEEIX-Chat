"use client";

import * as React from "react";
import { useRouter } from "next/navigation";

import { LandingPage } from "@/features/landing/components/landing-page";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { AppLogo } from "@/shared/components/app-logo";
import { PublicBrandSurface } from "@/shared/components/public-brand-surface";

const LANDING_GATE_TIMEOUT_MS = 800;

function LandingGateSkeleton() {
  return (
    <main
      className="flex h-svh items-center justify-center overflow-hidden bg-background text-foreground"
      aria-label="Comi AI"
      aria-busy="true"
    >
      <div className="flex flex-col items-center gap-5">
        <AppLogo height={72} priority className="animate-pulse text-foreground motion-reduce:animate-none" />
        <span className="h-1 w-20 overflow-hidden rounded-full bg-muted" aria-hidden>
          <span className="block h-full w-1/2 animate-[landing-gate_1.2s_ease-in-out_infinite] rounded-full bg-primary motion-reduce:animate-none" />
        </span>
      </div>
    </main>
  );
}

export function LandingRoute() {
  const router = useRouter();
  const [showLanding, setShowLanding] = React.useState(false);

  React.useEffect(() => {
    let mounted = true;
    const timeout = window.setTimeout(() => {
      if (mounted) {
        setShowLanding(true);
      }
    }, LANDING_GATE_TIMEOUT_MS);

    void resolveAccessToken()
      .then((token) => {
        if (!mounted) {
          return;
        }
        if (token) {
          router.replace("/chat");
          return;
        }
        window.clearTimeout(timeout);
        setShowLanding(true);
      })
      .catch(() => {
        if (!mounted) {
          return;
        }
        window.clearTimeout(timeout);
        setShowLanding(true);
      });

    return () => {
      mounted = false;
      window.clearTimeout(timeout);
    };
  }, [router]);

  return (
    <PublicBrandSurface>
      {showLanding ? <LandingPage /> : <LandingGateSkeleton />}
    </PublicBrandSurface>
  );
}
