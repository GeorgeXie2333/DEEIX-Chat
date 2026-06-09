import type { MetadataRoute } from "next";

import { pwaAsset } from "@/shared/pwa/assets";

export const dynamic = "force-static";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "Comi AI",
    short_name: "Comi AI",
    description: "Comi AI is a multi-model AI conversation workspace.",
    id: "/",
    start_url: "/chat",
    scope: "/",
    display: "standalone",
    background_color: "#ffffff",
    theme_color: "#0f172a",
    orientation: "any",
    categories: ["productivity", "business", "utilities"],
    lang: "en",
    icons: [
      {
        src: pwaAsset("/pwa/icon-192.png"),
        sizes: "192x192",
        type: "image/png",
        purpose: "any",
      },
      {
        src: pwaAsset("/pwa/icon-512.png"),
        sizes: "512x512",
        type: "image/png",
        purpose: "any",
      },
      {
        src: pwaAsset("/pwa/icon-maskable-512.png"),
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ],
  };
}
