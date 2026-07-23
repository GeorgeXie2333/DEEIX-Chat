import { Suspense } from "react";

import { LoginRoute } from "@/app/(auth)/login/login-route";
import { AppLogo } from "@/shared/components/app-logo";

function LoginRouteFallback() {
  return (
    <main
      className="h-svh overflow-hidden bg-background text-foreground"
      aria-label="Comi AI"
      aria-busy="true"
    >
      <header className="h-16 border-b border-border/60 bg-background/90">
        <div className="mx-auto flex h-full w-full max-w-[1180px] items-center px-4 sm:px-6 lg:px-8 xl:px-0">
          <AppLogo height={48} priority className="text-foreground" />
        </div>
      </header>
      <section className="mx-auto flex min-h-[calc(100svh-4rem)] w-full max-w-[1180px] items-start justify-center px-4 py-8 sm:items-center sm:px-6 sm:py-12 lg:px-8 xl:px-0">
        <div className="w-full max-w-[420px] rounded-[1.5rem] border border-border/70 bg-card/88 p-5 shadow-[0_28px_90px_-58px_color-mix(in_oklch,var(--foreground)_45%,transparent)] sm:p-8">
          <div
            className="min-h-[29rem] animate-pulse space-y-6 motion-reduce:animate-none"
            aria-hidden
          >
            <div className="grid grid-cols-2 gap-2 rounded-xl bg-muted/65 p-1">
              <span className="h-11 rounded-lg bg-background/75" />
              <span className="h-11 rounded-lg bg-muted" />
            </div>
            <div className="space-y-3">
              <span className="block h-7 w-2/5 rounded-md bg-muted" />
              <span className="block h-4 w-4/5 rounded-md bg-muted" />
            </div>
            <div className="space-y-5">
              <span className="block h-11 rounded-lg bg-muted" />
              <span className="block h-11 rounded-lg bg-muted" />
              <span className="block h-11 rounded-lg bg-muted" />
            </div>
          </div>
        </div>
      </section>
    </main>
  );
}

export default function Page() {
  return (
    <div className="contents" data-public-branding-ready>
      <Suspense fallback={<LoginRouteFallback />}>
        <LoginRoute />
      </Suspense>
    </div>
  );
}
