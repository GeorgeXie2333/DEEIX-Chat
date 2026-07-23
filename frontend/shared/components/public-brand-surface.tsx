"use client";

import * as React from "react";

let publicSurfaceMounts = 0;

export function PublicBrandSurface({ children }: { children: React.ReactNode }) {
  React.useEffect(() => {
    const root = document.documentElement;
    publicSurfaceMounts += 1;
    root.dataset.publicSurface = "true";

    return () => {
      publicSurfaceMounts = Math.max(0, publicSurfaceMounts - 1);
      if (publicSurfaceMounts === 0) {
        delete root.dataset.publicSurface;
      }
    };
  }, []);

  return children;
}
