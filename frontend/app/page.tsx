import type { Metadata } from "next";

import { LandingRoute } from "@/features/landing/components/landing-route";

const title = "Comi AI｜一个入口，用遍主流 AI";
const description =
  "汇集 11+ 家主流模型厂商，在同一个入口按任务选择模型。闭源模型均为官方 API 直连，所有模型统一稳定接入，价格与官方同步。";

export const metadata: Metadata = {
  metadataBase: new URL("https://comiai.cc"),
  title,
  description,
  openGraph: {
    type: "website",
    title,
    description,
    images: [
      {
        url: "/og/comi-ai-landing.png",
        width: 1200,
        height: 630,
        alt: "Comi AI — 一个入口，用遍主流 AI",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title,
    description,
    images: ["/og/comi-ai-landing.png"],
  },
};

export default function Page() {
  return (
    <div className="contents" data-public-branding-ready>
      <LandingRoute />
    </div>
  );
}
