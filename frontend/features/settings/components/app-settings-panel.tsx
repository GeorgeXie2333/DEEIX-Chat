import type { ReactNode } from "react";

import { SettingsSidebar } from "@/features/settings/components/settings-sidebar";

export function AppSettingsPanel({
  children,
  activeSection,
  basePath = "/setting",
}: {
  children: ReactNode;
  activeSection?: string;
  basePath?: string;
}) {
  const wideContent = activeSection === "account" || activeSection === "subscription" || activeSection === "openapi";

  return (
    <div className="flex h-full min-h-0 w-full flex-1 overflow-hidden bg-background">
      <div className="mx-auto flex h-full min-h-0 w-full max-w-[1230px] flex-col gap-4 overflow-hidden px-3 py-4 md:px-6 xl:flex-row xl:gap-8 xl:px-0 xl:py-6">
        <SettingsSidebar basePath={basePath} />
        <main className="min-h-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto overscroll-x-none [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden">
          <div
            className={
              wideContent
                ? "mx-auto w-full max-w-[1080px] min-w-0 xl:pt-20"
                : "mx-auto w-full max-w-[960px] min-w-0 xl:pt-20"
            }
          >
            {children}
          </div>
        </main>
      </div>
    </div>
  );
}
